/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
 * SPDX-FileCopyrightText: Copyright (c) 2025 Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// expectedRrsetInfo is CR-side data for an RRset owned by this zone.
type expectedRrsetInfo struct {
	comment *string
}

// expectedRrsetsForZone maps name/type → CR info for this zone's RRsets.
func expectedRrsetsForZone(ctx context.Context, zoneName, kind string, cl client.Client) (map[string]expectedRrsetInfo, error) {
	var rrsets dnsv1alpha2.RRsetList
	if err := cl.List(ctx, &rrsets); err != nil {
		return nil, err
	}
	var clusterRrsets dnsv1alpha2.ClusterRRsetList
	if err := cl.List(ctx, &clusterRrsets); err != nil {
		return nil, err
	}

	expected := make(map[string]expectedRrsetInfo)
	for i := range rrsets.Items {
		r := &rrsets.Items[i]
		if r.Spec.ZoneRef.Name == zoneName && r.Spec.ZoneRef.Kind == kind {
			expected[rrsetIdentityKey(getRRsetName(r), r.Spec.Type)] = expectedRrsetInfo{comment: r.Spec.Comment}
		}
	}
	for i := range clusterRrsets.Items {
		r := &clusterRrsets.Items[i]
		if r.Spec.ZoneRef.Name == zoneName && r.Spec.ZoneRef.Kind == kind {
			expected[rrsetIdentityKey(getRRsetName(r), r.Spec.Type)] = expectedRrsetInfo{comment: r.Spec.Comment}
		}
	}
	return expected, nil
}

func expectedRrsetKeySet(expected map[string]expectedRrsetInfo) map[string]struct{} {
	keys := make(map[string]struct{}, len(expected))
	for k := range expected {
		keys[k] = struct{}{}
	}
	return keys
}

func rrsetRecordsAndTTL(rr powerdns.RRset) ([]string, uint32) {
	contents := make([]string, 0, len(rr.Records))
	for _, rec := range rr.Records {
		contents = append(contents, ptr.Deref(rec.Content, ""))
	}
	ttl := uint32(3600)
	if rr.TTL != nil {
		ttl = *rr.TTL
	}
	return contents, ttl
}

// clearStaleOrphanRrsetMarkers drops orphan-since markers on re-owned RRsets.
func clearStaleOrphanRrsetMarkers(ctx context.Context, zoneName string, pdnsRrsets []powerdns.RRset, expected map[string]expectedRrsetInfo, PDNSClient PdnsClienter, log logr.Logger) {
	for _, rr := range pdnsRrsets {
		if rr.Name == nil || rr.Type == nil {
			continue
		}
		key := rrsetIdentityKey(*rr.Name, string(*rr.Type))
		info, owned := expected[key]
		if !owned {
			continue
		}
		content, ok := operatorCommentContent(rr.Comments)
		if !ok {
			continue
		}
		if _, isMarker := parseOrphanSinceComment(content); !isMarker {
			continue
		}
		contents, ttl := rrsetRecordsAndTTL(rr)
		if err := PDNSClient.Records.Change(ctx, zoneName, *rr.Name, *rr.Type, ttl, contents, powerdns.WithComments(operatorComment(info.comment))); err != nil {
			log.Error(err, "failed to clear stale orphan-since on managed RRset", "zone", zoneName, "name", *rr.Name, "type", string(*rr.Type))
			continue
		}
		log.V(1).Info("cleared stale orphan-since marker on managed RRset", "zone", zoneName, "name", *rr.Name, "type", string(*rr.Type))
	}
}

// rrsetStillOrphanAged reports whether the PDNS RRset still has an aged orphan-since marker.
func rrsetStillOrphanAged(ctx context.Context, zoneName, name string, rrType powerdns.RRType, grace time.Duration, now time.Time, PDNSClient PdnsClienter) (bool, error) {
	records, err := PDNSClient.Records.Get(ctx, zoneName, name, &rrType)
	if err != nil {
		return false, err
	}
	var matched powerdns.RRset
	found := false
	for _, fr := range records {
		if fr.Name != nil && fr.Type != nil && makeCanonical(*fr.Name) == makeCanonical(name) && *fr.Type == rrType {
			matched = fr
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	content, ok := operatorCommentContent(matched.Comments)
	if !ok {
		return false, nil
	}
	since, ok := parseOrphanSinceComment(content)
	if !ok {
		return false, nil
	}
	return orphanSinceAged(since, grace, now), nil
}

// findOrphanRrsets returns operator-marked PDNS RRsets missing from expected.
// SOA and apex NS are never orphans.
func findOrphanRrsets(zoneCanonical string, pdnsRrsets []powerdns.RRset, expected map[string]struct{}) []powerdns.RRset {
	zoneCanonical = makeCanonical(zoneCanonical)
	orphans := make([]powerdns.RRset, 0)
	for _, rr := range pdnsRrsets {
		if rr.Name == nil || rr.Type == nil {
			continue
		}
		if *rr.Type == powerdns.RRTypeSOA {
			continue
		}
		if *rr.Type == powerdns.RRTypeNS && makeCanonical(*rr.Name) == zoneCanonical {
			continue
		}
		if !hasOperatorAccount(rr.Comments) {
			continue
		}
		key := rrsetIdentityKey(*rr.Name, string(*rr.Type))
		if _, ok := expected[key]; ok {
			continue
		}
		orphans = append(orphans, rr)
	}
	return orphans
}

// detectOrphanRrsets inventories orphan RRsets; with cleanup, flags then deletes after grace.
func detectOrphanRrsets(ctx context.Context, gz dnsv1alpha2.GenericZone, zoneRes *powerdns.Zone, cl client.Client, PDNSClient PdnsClienter, log logr.Logger, orphanRRsetCleanup bool, orphanRRsetGrace time.Duration) {
	if zoneRes == nil {
		return
	}

	zoneName := gz.GetName()
	kind := zoneRefKind(gz)

	expected, err := expectedRrsetsForZone(ctx, zoneName, kind, cl)
	if err != nil {
		log.Error(err, "orphan detect: failed to list RRsets")
		return
	}

	clearStaleOrphanRrsetMarkers(ctx, zoneName, zoneRes.RRsets, expected, PDNSClient, log)

	orphans := findOrphanRrsets(zoneName, zoneRes.RRsets, expectedRrsetKeySet(expected))
	for _, o := range orphans {
		log.Info("detected orphan PDNS rrset", "zone", zoneName, "name", ptr.Deref(o.Name, ""), "type", string(ptr.Deref(o.Type, "")))
	}
	setOrphanRrsetsMetric(zoneName, float64(len(orphans)))

	if !orphanRRsetCleanup {
		return
	}

	now := time.Now().UTC()
	deleted := 0
	for _, o := range orphans {
		name := ptr.Deref(o.Name, "")
		rrType := ptr.Deref(o.Type, "")
		if name == "" || rrType == "" {
			continue
		}

		content, _ := operatorCommentContent(o.Comments)
		since, ok := parseOrphanSinceComment(content)
		if !ok {
			contents, ttl := rrsetRecordsAndTTL(o)
			if err := PDNSClient.Records.Change(ctx, zoneName, name, rrType, ttl, contents, powerdns.WithComments(orphanSinceComment(now))); err != nil {
				log.Error(err, "failed to flag orphan PDNS rrset", "zone", zoneName, "name", name, "type", string(rrType))
				continue
			}
			log.Info("flagged orphan PDNS rrset", "zone", zoneName, "name", name, "type", string(rrType), "since", now.Unix())
			continue
		}

		if !orphanSinceAged(since, orphanRRsetGrace, now) {
			log.Info("orphan PDNS rrset pending grace", "zone", zoneName, "name", name, "type", string(rrType), "since", since.Unix())
			continue
		}

		freshExpected, err := expectedRrsetsForZone(ctx, zoneName, kind, cl)
		if err != nil {
			log.Error(err, "orphan rrset delete: failed to re-list RRsets", "zone", zoneName)
			setOrphanRrsetsMetric(zoneName, float64(len(orphans)-deleted))
			return
		}
		key := rrsetIdentityKey(name, string(rrType))
		if _, stillExpected := freshExpected[key]; stillExpected {
			continue
		}
		// Skip if re-adopted or no longer aged.
		stillOrphan, err := rrsetStillOrphanAged(ctx, zoneName, name, rrType, orphanRRsetGrace, now, PDNSClient)
		if err != nil {
			log.Error(err, "orphan rrset delete: failed to re-fetch RRset", "zone", zoneName, "name", name, "type", string(rrType))
			continue
		}
		if !stillOrphan {
			continue
		}
		if err := PDNSClient.Records.Delete(ctx, zoneName, name, rrType); err != nil {
			log.Error(err, "failed to delete orphan PDNS rrset", "zone", zoneName, "name", name, "type", string(rrType))
			continue
		}
		incOrphanDeletion("rrset")
		deleted++
		log.Info("deleted orphan PDNS rrset", "zone", zoneName, "name", name, "type", string(rrType))
	}
	if deleted > 0 {
		setOrphanRrsetsMetric(zoneName, float64(len(orphans)-deleted))
	}
}
