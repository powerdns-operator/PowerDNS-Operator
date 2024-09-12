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
	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
	"k8s.io/utils/ptr"
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

// zoneIsIdenticalToExternalZone return True, True if respectively kind and Catalog are identical
// and nameservers are identical between Zone and External Resource
func zoneIsIdenticalToExternalZone(zone *dnsv1alpha1.Zone, externalZone *powerdns.Zone, ns []string) (bool, bool) {
	zoneCatalog := ptr.Deref(zone.Spec.Catalog, "")
	externalZoneCatalog := ptr.Deref(externalZone.Catalog, "")
	return zone.Spec.Kind == string(*externalZone.Kind) && zoneCatalog == externalZoneCatalog, reflect.DeepEqual(zone.Spec.Nameservers, ns)
}

// rrsetIsIdenticalToExternalRRset return True if Comments, Name, Type, TTL and Records are identical between RRSet and External Resource
func rrsetIsIdenticalToExternalRRset(rrset *dnsv1alpha1.RRset, externalRecord powerdns.RRset) bool {
	commentsIdentical := true
	if len(externalRecord.Comments) != 0 {
		if rrset.Spec.Comment != nil {
			commentsIdentical = *rrset.Spec.Comment == *(externalRecord.Comments[0].Content)
		} else {
			commentsIdentical = false
		}
	} else {
		if rrset.Spec.Comment != nil {
			commentsIdentical = false
		}
	}

	externalRecordsSlice := make([]string, 0, len(externalRecord.Records))
	for _, r := range externalRecord.Records {
		externalRecordsSlice = append(externalRecordsSlice, *r.Content)
	}
	return makeCanonical(rrset.ObjectMeta.Name) == *externalRecord.Name && rrset.Spec.Type == string(*externalRecord.Type) && rrset.Spec.TTL == *(externalRecord.TTL) && commentsIdentical && reflect.DeepEqual(rrset.Spec.Records, externalRecordsSlice)
}

func makeCanonical(in string) string {
	return fmt.Sprintf("%s.", strings.TrimSuffix(in, "."))
}
