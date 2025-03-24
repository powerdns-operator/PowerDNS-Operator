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
	"strings"

	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
)

const (
	FAILED_STATUS    = "Failed"
	PENDING_STATUS   = "Pending"
	SUCCEEDED_STATUS = "Succeeded"
)

type pdnsRecordsClienter interface {
	Delete(ctx context.Context, domain string, name string, recordType powerdns.RRType) error
	Change(ctx context.Context, domain string, name string, recordType powerdns.RRType, ttl uint32, content []string, options ...func(*powerdns.RRset)) error
	Get(ctx context.Context, domain, name string, recordType *powerdns.RRType) ([]powerdns.RRset, error)
}

type pdnsZonesClienter interface {
	Get(ctx context.Context, domain string) (*powerdns.Zone, error)
	Delete(ctx context.Context, domain string) error
	Change(ctx context.Context, domain string, zone *powerdns.Zone) error
	Add(ctx context.Context, zone *powerdns.Zone) (*powerdns.Zone, error)
}

type PdnsClienter struct {
	Records pdnsRecordsClienter
	Zones   pdnsZonesClienter
}

// zoneIsIdenticalToExternalZone return True, True if respectively kind, soa_edit_api and catalog are identical
// and nameservers are identical between Zone and External Resource
func zoneIsIdenticalToExternalZone(zone dnsv1alpha2.GenericZone, externalZone *powerdns.Zone, ns []string) (bool, bool) {
	zoneCatalog := makeCanonical(ptr.Deref(zone.GetSpec().Catalog, ""))
	externalZoneCatalog := ptr.Deref(externalZone.Catalog, "")
	zoneSOAEditAPI := ptr.Deref(zone.GetSpec().SOAEditAPI, "")
	externalZoneSOAEditAPI := ptr.Deref(externalZone.SOAEditAPI, "")
	return zone.GetSpec().Kind == string(*externalZone.Kind) && zoneCatalog == externalZoneCatalog && zoneSOAEditAPI == externalZoneSOAEditAPI, reflect.DeepEqual(zone.GetSpec().Nameservers, ns)
}

// rrsetIsIdenticalToExternalRRset return True if Comments, Name, Type, TTL and Records are identical between RRSet and External Resource
func rrsetIsIdenticalToExternalRRset(rrset dnsv1alpha2.GenericRRset, externalRecord powerdns.RRset) bool {
	commentsIdentical := true
	if len(externalRecord.Comments) != 0 {
		if rrset.GetSpec().Comment != nil {
			commentsIdentical = *rrset.GetSpec().Comment == *(externalRecord.Comments[0].Content)
		} else {
			commentsIdentical = false
		}
	} else {
		if rrset.GetSpec().Comment != nil {
			commentsIdentical = false
		}
	}

	externalRecordsSlice := make([]string, 0, len(externalRecord.Records))
	for _, r := range externalRecord.Records {
		externalRecordsSlice = append(externalRecordsSlice, *r.Content)
	}
	name := getRRsetName(rrset)
	return name == *externalRecord.Name && rrset.GetSpec().Type == string(*externalRecord.Type) && rrset.GetSpec().TTL == *(externalRecord.TTL) && commentsIdentical && reflect.DeepEqual(rrset.GetSpec().Records, externalRecordsSlice)
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
