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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// orphanZoneDetector finds operator-marked PDNS zones with no matching CR.
type orphanZoneDetector struct {
	client   client.Client
	pdns     PdnsClienter
	interval time.Duration
	cleanup  bool
	grace    time.Duration
	log      logr.Logger
}

// clearOrphanZoneMetadata deletes stale orphan-since metadata; errors are returned for retry.
func clearOrphanZoneMetadata(ctx context.Context, zoneName string, PDNSClient PdnsClienter, log logr.Logger) error {
	if PDNSClient.Metadata == nil {
		return nil
	}
	md, err := PDNSClient.Metadata.Get(ctx, zoneName, OrphanSinceMetadataKind)
	if err != nil {
		if isPDNSNotFound(err) {
			return nil
		}
		return fmt.Errorf("orphan metadata lookup failed for %s: %w", zoneName, err)
	}
	if md == nil || len(md.Metadata) == 0 {
		return nil
	}
	if err := PDNSClient.Metadata.Delete(ctx, zoneName, OrphanSinceMetadataKind); err != nil && !isPDNSNotFound(err) {
		return fmt.Errorf("orphan metadata cleanup failed for %s: %w", zoneName, err)
	}
	log.V(1).Info("cleared stale orphan-since metadata", "zone", zoneName)
	return nil
}

// findOrphanZones returns operator-marked PDNS zones missing from expected.
func findOrphanZones(pdnsZones []powerdns.Zone, expected map[string]struct{}) []string {
	orphans := make([]string, 0)
	for _, z := range pdnsZones {
		if ptr.Deref(z.Account, "") != OperatorAccount {
			continue
		}
		name := makeCanonical(ptr.Deref(z.Name, ""))
		if name == "" {
			continue
		}
		if _, ok := expected[name]; ok {
			continue
		}
		orphans = append(orphans, name)
	}
	return orphans
}

func (d orphanZoneDetector) Start(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	d.scan(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.scan(ctx)
		}
	}
}

func (d orphanZoneDetector) expectedZoneNames(ctx context.Context) (map[string]struct{}, error) {
	var zones dnsv1alpha2.ZoneList
	if err := d.client.List(ctx, &zones); err != nil {
		return nil, err
	}
	var clusterZones dnsv1alpha2.ClusterZoneList
	if err := d.client.List(ctx, &clusterZones); err != nil {
		return nil, err
	}

	expected := make(map[string]struct{}, len(zones.Items)+len(clusterZones.Items))
	for i := range zones.Items {
		expected[makeCanonical(zones.Items[i].Name)] = struct{}{}
	}
	for i := range clusterZones.Items {
		expected[makeCanonical(clusterZones.Items[i].Name)] = struct{}{}
	}
	return expected, nil
}

func (d orphanZoneDetector) scan(ctx context.Context) {
	pdnsZones, err := d.pdns.Zones.List(ctx)
	if err != nil {
		d.log.Error(err, "orphan zone detect: failed to list PDNS zones")
		return
	}

	expected, err := d.expectedZoneNames(ctx)
	if err != nil {
		d.log.Error(err, "orphan zone detect: failed to list Zone/ClusterZone CRs")
		return
	}

	orphans := findOrphanZones(pdnsZones, expected)
	for _, name := range orphans {
		d.log.Info("detected orphan PDNS zone", "zone", name)
	}
	setOrphanZonesMetric(float64(len(orphans)))

	if !d.cleanup {
		return
	}
	if d.pdns.Metadata == nil {
		d.log.Error(nil, "orphan zone cleanup enabled but Metadata client is nil")
		return
	}

	now := time.Now().UTC()
	deleted := 0
	for _, name := range orphans {
		since, hasMarker, err := d.readOrphanSince(ctx, name)
		if err != nil {
			d.log.Error(err, "orphan zone cleanup: failed to read metadata", "zone", name)
			continue
		}
		if !hasMarker {
			value := formatOrphanSinceEpoch(now)
			if _, err := d.pdns.Metadata.Set(ctx, name, OrphanSinceMetadataKind, []string{value}); err != nil {
				d.log.Error(err, "failed to flag orphan PDNS zone", "zone", name)
				continue
			}
			d.log.Info("flagged orphan PDNS zone", "zone", name, "since", now.Unix())
			continue
		}

		if !orphanSinceAged(since, d.grace, now) {
			d.log.Info("orphan PDNS zone pending grace", "zone", name, "since", since.Unix())
			continue
		}

		freshExpected, err := d.expectedZoneNames(ctx)
		if err != nil {
			d.log.Error(err, "orphan zone delete: failed to re-list Zone/ClusterZone CRs")
			setOrphanZonesMetric(float64(len(orphans) - deleted))
			return
		}
		if _, stillExpected := freshExpected[name]; stillExpected {
			continue
		}
		// Skip if re-adopted (marker cleared) or no longer aged.
		since2, hasMarker2, err := d.readOrphanSince(ctx, name)
		if err != nil {
			d.log.Error(err, "orphan zone delete: failed to re-read metadata", "zone", name)
			continue
		}
		if !hasMarker2 || !orphanSinceAged(since2, d.grace, now) {
			continue
		}
		if err := d.pdns.Zones.Delete(ctx, name); err != nil {
			d.log.Error(err, "failed to delete orphan PDNS zone", "zone", name)
			continue
		}
		incOrphanDeletion("zone")
		deleted++
		d.log.Info("deleted orphan PDNS zone", "zone", name)
	}
	if deleted > 0 {
		setOrphanZonesMetric(float64(len(orphans) - deleted))
	}
}

func (d orphanZoneDetector) readOrphanSince(ctx context.Context, zoneName string) (time.Time, bool, error) {
	md, err := d.pdns.Metadata.Get(ctx, zoneName, OrphanSinceMetadataKind)
	if err != nil {
		if isPDNSNotFound(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if md == nil || len(md.Metadata) == 0 {
		return time.Time{}, false, nil
	}
	since, ok := parseOrphanSinceEpoch(md.Metadata[0])
	if !ok {
		// Malformed → re-stamp, never delete.
		return time.Time{}, false, nil
	}
	return since, true, nil
}
