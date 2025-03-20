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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestZoneIsIdenticalToExternalZone(t *testing.T) {
	var (
		name         = "example.org"
		namespace    = "example"
		nameservers  = []string{"ns1.example.org", "ns2.example.org"}
		soaEditApi   = "DEFAULT"
		kind         = powerdns.ZoneKind(MASTER_KIND_ZONE)
		nameservers1 = []string{"ns1.example1.org", "ns2.example1.org"}
		soaEditApi1  = "EPOCH"

		catalog  = "catalog.org."
		catalog1 = "catalog1.org."
	)

	var testCases = []struct {
		description          string
		genericZone          dnsv1alpha2.GenericZone
		externalZone         *powerdns.Zone
		nameservers          []string
		zonesIdentical       bool
		nameserversIdentical bool
	}{
		{
			"Identical Zones",
			&dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					Kind:        MASTER_KIND_ZONE,
					Nameservers: nameservers,
					Catalog:     &catalog,
					SOAEditAPI:  &soaEditApi,
				},
			},
			&powerdns.Zone{
				ID:         &name,
				Name:       &name,
				Kind:       &kind,
				Catalog:    &catalog,
				SOAEditAPI: &soaEditApi,
			},
			nameservers,
			true,
			true,
		},
		{
			"Different Zones on NS",
			&dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					Kind:        MASTER_KIND_ZONE,
					Nameservers: nameservers,
					Catalog:     &catalog,
					SOAEditAPI:  &soaEditApi,
				},
			},
			&powerdns.Zone{
				ID:         &name,
				Name:       &name,
				Kind:       &kind,
				Catalog:    &catalog,
				SOAEditAPI: &soaEditApi,
			},
			nameservers1,
			true,
			false,
		},
		{
			"Different Zones on Kind",
			&dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					Kind:        NATIVE_KIND_ZONE,
					Nameservers: nameservers,
					Catalog:     &catalog,
					SOAEditAPI:  &soaEditApi,
				},
			},
			&powerdns.Zone{
				ID:         &name,
				Name:       &name,
				Kind:       &kind,
				Catalog:    &catalog,
				SOAEditAPI: &soaEditApi,
			},
			nameservers,
			false,
			true,
		},
		{
			"Different Zones on Catalog",
			&dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					Kind:        NATIVE_KIND_ZONE,
					Nameservers: nameservers,
					Catalog:     &catalog1,
					SOAEditAPI:  &soaEditApi,
				},
			},
			&powerdns.Zone{
				ID:         &name,
				Name:       &name,
				Kind:       &kind,
				Catalog:    &catalog,
				SOAEditAPI: &soaEditApi,
			},
			nameservers,
			false,
			true,
		},
		{
			"Different Zones on SOAEditAPI",
			&dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					Kind:        NATIVE_KIND_ZONE,
					Nameservers: nameservers,
					Catalog:     &catalog,
					SOAEditAPI:  &soaEditApi1,
				},
			},
			&powerdns.Zone{
				ID:         &name,
				Name:       &name,
				Kind:       &kind,
				Catalog:    &catalog,
				SOAEditAPI: &soaEditApi,
			},
			nameservers,
			false,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			zone, ns := zoneIsIdenticalToExternalZone(tc.genericZone, tc.externalZone, tc.nameservers)
			if !cmp.Equal(zone, tc.zonesIdentical) {
				t.Errorf("ZONE: got %v, want %v", zone, tc.zonesIdentical)
			}
			if !cmp.Equal(ns, tc.nameserversIdentical) {
				t.Errorf("NS: got %v, want %v", ns, tc.nameserversIdentical)
			}
		})
	}
}

func TestRrsetIsIdenticalToExternalRRset(t *testing.T) {
	var (
		name           = "test.example.org"
		recordName     = "test"
		zoneName       = "example.org"
		fqdnName       = recordName + "." + zoneName + "."
		namespace      = "example"
		recordComment1 = "nothing to tell"
		recordComment2 = "really nothing to tell"
		recordType1    = "A"
		recordType2    = "AAAA"
		recordTtl1     = uint32(1500)
		recordTtl2     = uint32(3600)
		recordContent1 = "1.1.1.1"
		recordContent2 = "2.2.2.2"
		recordContent3 = "2001:0dc8:86a4:0000:0000:7a2f:2360:2341"
		records        = []string{recordContent1, recordContent2}
	)

	var testCases = []struct {
		description     string
		rrset           *dnsv1alpha2.RRset
		externalRrset   *powerdns.RRset
		rrsetsIdentical bool
	}{
		{
			"Identical RRsets",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment1,
					Name:    recordName,
					Type:    recordType1,
					TTL:     recordTtl1,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			&powerdns.RRset{
				Name: &fqdnName,
				Type: (*powerdns.RRType)(&recordType1),
				TTL:  &recordTtl1,
				Records: []powerdns.Record{
					{
						Content:  &recordContent1,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
					{
						Content:  &recordContent2,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
				},
				Comments: []powerdns.Comment{
					{
						Content: &recordComment1,
					},
				},
			},
			true,
		},
		{
			"Different RRsets on Comment",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment1,
					Name:    recordName,
					Type:    recordType1,
					TTL:     recordTtl1,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			&powerdns.RRset{
				Name: &fqdnName,
				Type: (*powerdns.RRType)(&recordType1),
				TTL:  &recordTtl1,
				Records: []powerdns.Record{
					{
						Content:  &recordContent1,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
					{
						Content:  &recordContent2,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
				},
				Comments: []powerdns.Comment{
					{
						Content: &recordComment2,
					},
				},
			},
			false,
		},
		{
			"Different RRsets on Records",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment1,
					Name:    recordName,
					Type:    recordType1,
					TTL:     recordTtl1,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			&powerdns.RRset{
				Name: &fqdnName,
				Type: (*powerdns.RRType)(&recordType1),
				TTL:  &recordTtl1,
				Records: []powerdns.Record{
					{
						Content:  &recordContent3,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
				},
				Comments: []powerdns.Comment{
					{
						Content: &recordComment1,
					},
				},
			},
			false,
		},
		{
			"Different RRsets on Type",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment1,
					Name:    recordName,
					Type:    recordType1,
					TTL:     recordTtl1,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			&powerdns.RRset{
				Name: &fqdnName,
				Type: (*powerdns.RRType)(&recordType2),
				TTL:  &recordTtl1,
				Records: []powerdns.Record{
					{
						Content:  &recordContent1,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
					{
						Content:  &recordContent2,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
				},
				Comments: []powerdns.Comment{
					{
						Content: &recordComment1,
					},
				},
			},
			false,
		},
		{
			"Different RRsets on TTL",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment1,
					Name:    recordName,
					Type:    recordType1,
					TTL:     recordTtl1,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			&powerdns.RRset{
				Name: &fqdnName,
				Type: (*powerdns.RRType)(&recordType1),
				TTL:  &recordTtl2,
				Records: []powerdns.Record{
					{
						Content:  &recordContent1,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
					{
						Content:  &recordContent2,
						Disabled: ptr.To(false),
						SetPTR:   ptr.To(false),
					},
				},
				Comments: []powerdns.Comment{
					{
						Content: &recordComment1,
					},
				},
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ns := rrsetIsIdenticalToExternalRRset(tc.rrset, *tc.externalRrset)
			if !cmp.Equal(ns, tc.rrsetsIdentical) {
				t.Errorf("got %v, want %v", ns, tc.rrsetsIdentical)
			}
		})
	}
}

func TestMakeCanonical(t *testing.T) {
	var testCases = []struct {
		description string
		entry       string
		want        string
	}{
		{
			"Non canonical entry",
			"test.example.org",
			"test.example.org.",
		},
		{
			"Canonical entry",
			"test.example.org.",
			"test.example.org.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			canonical := makeCanonical(tc.entry)
			if !cmp.Equal(canonical, tc.want) {
				t.Errorf("got %v, want %v", canonical, tc.want)
			}
		})
	}
}

func TestGetRRsetName(t *testing.T) {
	var (
		name           = "test.example.org"
		recordName     = "test"
		recordFQDName  = "test.example.org."
		zoneName       = "example.org"
		namespace      = "example"
		recordComment  = "nothing to tell"
		recordType     = "A"
		recordTtl      = uint32(1500)
		recordContent  = "1.1.1.1"
		recordContent2 = "2.2.2.2"
		records        = []string{recordContent, recordContent2}
	)
	var testCases = []struct {
		description string
		entry       *dnsv1alpha2.RRset
		want        string
	}{
		{
			"Non FQDN entry",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment,
					Name:    recordName,
					Type:    recordType,
					TTL:     recordTtl,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			"test.example.org.",
		},
		{
			"FQDN entry",
			&dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: dnsv1alpha2.RRsetSpec{
					Comment: &recordComment,
					Name:    recordFQDName,
					Type:    recordType,
					TTL:     recordTtl,
					Records: records,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: "Zone",
					},
				},
			},
			"test.example.org.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			fqdn := getRRsetName(tc.entry)
			if !cmp.Equal(fqdn, tc.want) {
				t.Errorf("got %v, want %v", fqdn, tc.want)
			}
		})
	}
}
