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
	"slices"
	"strings"

	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
)

type pdnsRecordsClienter interface {
	Delete(ctx context.Context, domain string, name string, recordType powerdns.RRType) error
	Change(ctx context.Context, domain string, name string, recordType powerdns.RRType, ttl uint32, content []string, options ...func(*powerdns.RRset)) error
	Get(ctx context.Context, domain, name string, recordType *powerdns.RRType) ([]powerdns.RRset, error)
}

type pdnsZonesClienter interface {
	List(ctx context.Context) ([]powerdns.Zone, error)
	Get(ctx context.Context, domain string) (*powerdns.Zone, error)
	Delete(ctx context.Context, domain string) error
	Change(ctx context.Context, domain string, zone *powerdns.Zone) error
	Add(ctx context.Context, zone *powerdns.Zone) (*powerdns.Zone, error)
}

type pdnsMetadataClienter interface {
	Get(ctx context.Context, domain string, kind powerdns.MetadataKind) (*powerdns.Metadata, error)
	Set(ctx context.Context, domain string, kind powerdns.MetadataKind, values []string) (*powerdns.Metadata, error)
	Delete(ctx context.Context, domain string, kind powerdns.MetadataKind) error
}

type PdnsClienter struct {
	Records  pdnsRecordsClienter
	Zones    pdnsZonesClienter
	Metadata pdnsMetadataClienter
}

// OperatorAccount marks Zone and RRset writes in PowerDNS.
const OperatorAccount = "powerdns-operator"

// operatorComment builds a PDNS comment with OperatorAccount; strips orphan-since prefixes.
func operatorComment(content *string) powerdns.Comment {
	c := ptr.Deref(content, "")
	if strings.HasPrefix(strings.TrimSpace(c), orphanSinceCommentPrefix) {
		c = ""
	}
	return powerdns.Comment{
		Content: ptr.To(c),
		Account: ptr.To(OperatorAccount),
	}
}

func hasOperatorAccount(comments []powerdns.Comment) bool {
	_, ok := operatorCommentContent(comments)
	return ok
}

func operatorCommentContent(comments []powerdns.Comment) (string, bool) {
	for _, c := range comments {
		if ptr.Deref(c.Account, "") == OperatorAccount {
			return ptr.Deref(c.Content, ""), true
		}
	}
	return "", false
}

// zoneIsIdenticalToExternalZone return True, True if respectively kind, soa_edit_api, catalog and account are identical
// and nameservers are identical between Zone and External Resource
func zoneIsIdenticalToExternalZone(zone dnsv1alpha2.GenericZone, externalZone *powerdns.Zone, ns []string) (bool, bool) {
	zoneCatalog := makeCanonical(ptr.Deref(zone.GetSpec().Catalog, ""))
	externalZoneCatalog := ptr.Deref(externalZone.Catalog, "")
	zoneSOAEditAPI := ptr.Deref(zone.GetSpec().SOAEditAPI, "")
	externalZoneSOAEditAPI := ptr.Deref(externalZone.SOAEditAPI, "")
	accountIdentical := ptr.Deref(externalZone.Account, "") == OperatorAccount
	return zone.GetSpec().Kind == string(*externalZone.Kind) && zoneCatalog == externalZoneCatalog && zoneSOAEditAPI == externalZoneSOAEditAPI && accountIdentical, stringSlicesEqualUnordered(zone.GetSpec().Nameservers, ns)
}

// rrsetIsIdenticalToExternalRRset return True if Comments (incl. operator account), Name, Type, TTL and Records are identical between RRSet and External Resource
func rrsetIsIdenticalToExternalRRset(rrset dnsv1alpha2.GenericRRset, externalRecord powerdns.RRset) bool {
	// Use sanitized content so stripped orphan-since prefixes do not loop rewrites.
	expectedContent := ptr.Deref(operatorComment(rrset.GetSpec().Comment).Content, "")
	actualContent, hasAccount := operatorCommentContent(externalRecord.Comments)
	commentsIdentical := hasAccount && actualContent == expectedContent

	externalRecordsSlice := make([]string, 0, len(externalRecord.Records))
	for _, r := range externalRecord.Records {
		externalRecordsSlice = append(externalRecordsSlice, *r.Content)
	}
	name := getRRsetName(rrset)
	return name == *externalRecord.Name && rrset.GetSpec().Type == string(*externalRecord.Type) && rrset.GetSpec().TTL == *(externalRecord.TTL) && commentsIdentical && stringSlicesEqualUnordered(rrset.GetSpec().Records, externalRecordsSlice)
}

// stringSlicesEqualUnordered compares string slices as multisets (order-independent).
func stringSlicesEqualUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := slices.Clone(a)
	bb := slices.Clone(b)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

func makeCanonical(in string) string {
	var result string
	if in != "" {
		result = fmt.Sprintf("%s.", strings.TrimSuffix(in, "."))
	}
	return result
}

func getRRsetName(rrset dnsv1alpha2.GenericRRset) string {
	if !strings.HasSuffix(rrset.GetSpec().Name, ".") {
		return makeCanonical(rrset.GetSpec().Name + "." + rrset.GetSpec().ZoneRef.Name)
	}
	return makeCanonical(rrset.GetSpec().Name)
}

func zoneRefKind(gz dnsv1alpha2.GenericZone) string {
	switch gz.(type) {
	case *dnsv1alpha2.ClusterZone:
		return "ClusterZone"
	default:
		return "Zone"
	}
}

func rrsetIdentityKey(name, rrType string) string {
	return makeCanonical(name) + "/" + rrType
}
