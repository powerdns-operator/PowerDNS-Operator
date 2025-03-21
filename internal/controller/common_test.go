/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	m          mockClient
	PDNSClient PdnsClienter
)

func init() {
	m = NewMockClient()
	PDNSClient = PdnsClienter{
		Records: m.Records,
		Zones:   m.Zones,
	}
}

func setupTestCase() func() {
	var (
		name        = "example.org"
		nameservers = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi  = "DEFAULT"
		kind        = powerdns.ZoneKind(MASTER_KIND_ZONE)
		catalog     = "catalog.org."

		rrsetName    = "test.example.org"
		rrsetType    = powerdns.RRType("A")
		rrsetTTL     = uint32(1500)
		rrsetRecords = []string{"1.1.1.2", "2.2.2.3"}
	)
	_, _ = PDNSClient.Zones.Add(context.Background(),
		&powerdns.Zone{
			Name:        &name,
			Kind:        &kind,
			Nameservers: nameservers,
			SOAEditAPI:  &soaEditApi,
			Catalog:     &catalog,
		})
	_ = PDNSClient.Records.Change(context.Background(),
		makeCanonical(name),
		makeCanonical(rrsetName),
		rrsetType,
		rrsetTTL,
		rrsetRecords,
	)

	return func() {
		resetZonesMap()
		resetRecordsMap()
	}
}

func TestGetExternalResources(t *testing.T) {
	var (
		name        = "example.org"
		nameservers = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi  = "DEFAULT"
		kind        = powerdns.ZoneKind(MASTER_KIND_ZONE)
		catalog     = "catalog.org."
	)
	ctx := context.Background()
	log := log.FromContext(ctx)
	serial64, _ := strconv.ParseUint(
		fmt.Sprintf("%s02", time.Now().UTC().Format("20060102")), 10, 32)
	serial := uint32(serial64)

	var testCases = []struct {
		description string
		domain      string
		want        *powerdns.Zone
		e           error
	}{
		{"Existing Zone", name, &powerdns.Zone{Name: &name, Kind: &kind, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi, Serial: &serial}, nil},
		{"Missing Zone", "missing.com", &powerdns.Zone{}, nil},
		{"communication error", FAKE_SITE, nil, &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "Internal Server Error"}},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			z, err := getZoneExternalResources(ctx, tc.domain, PDNSClient, log)
			if !reflect.DeepEqual(z, tc.want) {
				t.Errorf("got %v, want %v", *z, tc.want)
			}
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestCreateExternalResources(t *testing.T) {
	var (
		name        = "example.org"
		namespace   = "example"
		nameservers = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi  = "DEFAULT"
		catalog     = "catalog.org."

		name1        = "example1.org"
		namespace1   = "example1"
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
		soaEditApi1  = "DEFAULT"

		name2        = "example2.org"
		nameservers2 = []string{"ns1.example2.org", "ns2.example2.org"}
		soaEditApi2  = "DEFAULT"
	)
	ctx := context.Background()
	log := log.FromContext(ctx)

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		e           error
	}{
		{"Valid Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: namespace1}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi1}}, nil},
		{"Valid ClusterZone", &dnsv1alpha2.ClusterZone{ObjectMeta: metav1.ObjectMeta{Name: name2}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers2, Catalog: &catalog, SOAEditAPI: &soaEditApi2}}, nil},
		{"Already existing Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, powerdns.Error{StatusCode: 409, Status: "409 Conflict", Message: "Conflict"}},
		{"communication error", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: FAKE_SITE, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "Internal Server Error"}},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := createZoneExternalResources(ctx, tc.genericZone, PDNSClient, log)
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestUpdateExternalResources(t *testing.T) {
	var (
		name        = "example.org"
		namespace   = "example"
		nameservers = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi  = "DEFAULT"
		catalog     = "catalog.org."

		name1        = "example1.org"
		namespace1   = "example1"
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
		soaEditApi1  = "DEFAULT"

		name2        = "example2.org"
		nameservers2 = []string{"ns1.example2.org", "ns2.example2.org"}
		soaEditApi2  = "DEFAULT"
	)
	ctx := context.Background()
	log := log.FromContext(ctx)

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		e           error
	}{
		{"Valid Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: SLAVE_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, nil},
		{"Valid ClusterZone", &dnsv1alpha2.ClusterZone{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: dnsv1alpha2.ZoneSpec{Kind: NATIVE_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, nil},
		{"Non-existing Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: namespace1}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi1}}, powerdns.Error{StatusCode: 404, Status: "404 Not Found", Message: "Not Found"}},
		{"Non-existing ClusterZone", &dnsv1alpha2.ClusterZone{ObjectMeta: metav1.ObjectMeta{Name: name2}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers2, Catalog: &catalog, SOAEditAPI: &soaEditApi2}}, powerdns.Error{StatusCode: 404, Status: "404 Not Found", Message: "Not Found"}},
		{"communication error", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: FAKE_SITE, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "Internal Server Error"}},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := updateZoneExternalResources(ctx, tc.genericZone, PDNSClient, log)
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestUpdateNsOnExternalResources(t *testing.T) {
	var (
		name        = "example.org"
		namespace   = "example"
		nameservers = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi  = "DEFAULT"
		catalog     = "catalog.org."

		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
		nameservers2 = []string{"ns1.example2.org", "ns2.example2.org"}
	)
	ctx := context.Background()
	log := log.FromContext(ctx)
	ttl := uint32(1500)

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		e           error
	}{
		{"Valid Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, nil},
		{"Valid ClusterZone", &dnsv1alpha2.ClusterZone{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: dnsv1alpha2.ZoneSpec{Kind: NATIVE_KIND_ZONE, Nameservers: nameservers2, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, nil},
		{"communication error", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: FAKE_SITE, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "Internal Server Error"}},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := updateNsOnZoneExternalResources(ctx, tc.genericZone, ttl, PDNSClient, log)
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestDeleteExternalResources(t *testing.T) {
	var (
		name       = "example.org"
		namespace  = "example"
		soaEditApi = "DEFAULT"
		catalog    = "catalog.org."

		name1        = "example1.org"
		namespace1   = "example1"
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
		soaEditApi1  = "DEFAULT"

		name2        = "example2.org"
		nameservers2 = []string{"ns1.example2.org", "ns2.example2.org"}
		soaEditApi2  = "DEFAULT"
	)
	ctx := context.Background()
	log := log.FromContext(ctx)

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		e           error
	}{
		{"Valid Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, nil},
		{"Non-existing Zone", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: name1, Namespace: namespace1}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi1}}, nil},
		{"Non-existing ClusterZone", &dnsv1alpha2.ClusterZone{ObjectMeta: metav1.ObjectMeta{Name: name2}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers2, Catalog: &catalog, SOAEditAPI: &soaEditApi2}}, nil},
		{"communication error", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: FAKE_SITE, Namespace: namespace1}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi1}}, &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "Internal Server Error"}},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := deleteZoneExternalResources(ctx, tc.genericZone, PDNSClient, log)
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestDeleteRrsetExternalResources(t *testing.T) {
	var (
		zoneName   = "example.org"
		namespace  = "example"
		soaEditApi = "DEFAULT"

		rrsetName1    = "test"
		rrsetFqdn1    = "test.example.org"
		rrsetType1    = "A"
		rrsetTTL1     = uint32(1500)
		rrsetRecords1 = []string{"1.1.1.2", "2.2.2.3"}
		rrsetComment1 = "What you want"

		rrsetName2    = "test2"
		rrsetFqdn2    = "test2.example.org"
		rrsetType2    = "A"
		rrsetTTL2     = uint32(1500)
		rrsetRecords2 = []string{"1.1.1.3", "2.2.2.4"}
		rrsetComment2 = "What you want"

		catalog      = "catalog.org."
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
	)
	ctx := context.Background()
	log := log.FromContext(ctx)

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		rrset       *dnsv1alpha2.RRset
		e           error
	}{
		{"Existing RRset", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: zoneName, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &dnsv1alpha2.RRset{ObjectMeta: metav1.ObjectMeta{Name: rrsetFqdn1, Namespace: namespace}, Spec: dnsv1alpha2.RRsetSpec{ZoneRef: dnsv1alpha2.ZoneRef{Name: zoneName, Kind: "Zone"}, Type: rrsetType1, Name: rrsetName1, TTL: rrsetTTL1, Records: rrsetRecords1, Comment: &rrsetComment1}}, nil},
		{"Inexisting RRset", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: zoneName, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &dnsv1alpha2.RRset{ObjectMeta: metav1.ObjectMeta{Name: rrsetFqdn2, Namespace: namespace}, Spec: dnsv1alpha2.RRsetSpec{ZoneRef: dnsv1alpha2.ZoneRef{Name: zoneName, Kind: "Zone"}, Type: rrsetType2, Name: rrsetName2, TTL: rrsetTTL2, Records: rrsetRecords2, Comment: &rrsetComment2}}, nil},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := deleteRrsetExternalResources(ctx, tc.genericZone, tc.rrset, PDNSClient, log)
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}

func TestCreateOrUpdateRrsetExternalResources(t *testing.T) {
	var (
		zoneName   = "example.org"
		namespace  = "example"
		soaEditApi = "DEFAULT"

		rrsetName1    = "test"
		rrsetFqdn1    = "test.example.org"
		rrsetType1    = "A"
		rrsetTTL1     = uint32(1500)
		rrsetRecords1 = []string{"1.1.1.2", "2.2.2.3"}
		rrsetComment1 = "What you want"

		rrsetName2    = "test2"
		rrsetFqdn2    = "test2.example.org"
		rrsetType2    = "A"
		rrsetTTL2     = uint32(1500)
		rrsetRecords2 = []string{"1.1.1.3", "2.2.2.4"}
		rrsetComment2 = "What you want"

		catalog      = "catalog.org."
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
	)
	ctx := context.Background()

	var testCases = []struct {
		description string
		genericZone dnsv1alpha2.GenericZone
		rrset       *dnsv1alpha2.RRset
		want        bool
		e           error
	}{
		{"RRset creation", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: zoneName, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &dnsv1alpha2.RRset{ObjectMeta: metav1.ObjectMeta{Name: rrsetFqdn2, Namespace: namespace}, Spec: dnsv1alpha2.RRsetSpec{ZoneRef: dnsv1alpha2.ZoneRef{Name: zoneName, Kind: "Zone"}, Type: rrsetType2, Name: rrsetName2, TTL: rrsetTTL2, Records: rrsetRecords2, Comment: &rrsetComment2}}, true, nil},
		{"RRset update", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: zoneName, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &dnsv1alpha2.RRset{ObjectMeta: metav1.ObjectMeta{Name: rrsetFqdn1, Namespace: namespace}, Spec: dnsv1alpha2.RRsetSpec{ZoneRef: dnsv1alpha2.ZoneRef{Name: zoneName, Kind: "Zone"}, Type: rrsetType1, Name: rrsetName1, TTL: rrsetTTL1, Records: rrsetRecords1, Comment: &rrsetComment1}}, true, nil},
		{"RRset identical", &dnsv1alpha2.Zone{ObjectMeta: metav1.ObjectMeta{Name: zoneName, Namespace: namespace}, Spec: dnsv1alpha2.ZoneSpec{Kind: MASTER_KIND_ZONE, Nameservers: nameservers1, Catalog: &catalog, SOAEditAPI: &soaEditApi}}, &dnsv1alpha2.RRset{ObjectMeta: metav1.ObjectMeta{Name: rrsetFqdn1, Namespace: namespace}, Spec: dnsv1alpha2.RRsetSpec{ZoneRef: dnsv1alpha2.ZoneRef{Name: zoneName, Kind: "Zone"}, Type: rrsetType1, Name: rrsetName1, TTL: rrsetTTL1, Records: rrsetRecords1, Comment: &rrsetComment1}}, false, nil},
	}

	// Mock initialization
	teardownTestCase := setupTestCase()
	defer teardownTestCase()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			modified, err := createOrUpdateRrsetExternalResources(ctx, tc.genericZone, tc.rrset, PDNSClient)
			if !cmp.Equal(modified, tc.want) {
				t.Errorf("got %v, want %v", modified, tc.want)
			}
			if !cmp.Equal(err, tc.e) {
				t.Errorf("got %v, want %v", err, tc.e)
			}
		})
	}
}
