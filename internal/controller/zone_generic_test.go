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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// failNSChangeRecords fails Records.Change; other calls delegate.
type failNSChangeRecords struct {
	inner pdnsRecordsClienter
}

func (f failNSChangeRecords) Get(ctx context.Context, domain, name string, recordType *powerdns.RRType) ([]powerdns.RRset, error) {
	return f.inner.Get(ctx, domain, name, recordType)
}

func (f failNSChangeRecords) Change(ctx context.Context, domain string, name string, recordType powerdns.RRType, ttl uint32, content []string, options ...func(*powerdns.RRset)) error {
	return &powerdns.Error{StatusCode: 500, Status: "500 Internal Server Error", Message: "stamp failed"}
}

func (f failNSChangeRecords) Delete(ctx context.Context, domain string, name string, recordType powerdns.RRType) error {
	return f.inner.Delete(ctx, domain, name, recordType)
}

func TestZoneExternalResourcesReconcileNSStampAfterCreate(t *testing.T) {
	resetZonesMap()
	resetRecordsMap()
	resetMetadataMap()
	defer func() {
		resetZonesMap()
		resetRecordsMap()
		resetMetadataMap()
	}()

	m := NewMockClient()
	gzr := GenericZoneReconciler{
		PDNSClient: PdnsClienter{
			Zones:    m.Zones,
			Records:  failNSChangeRecords{inner: m.Records},
			Metadata: m.Metadata,
		},
		log: log.FromContext(context.Background()),
	}

	zone := &dnsv1alpha2.Zone{
		ObjectMeta: metav1.ObjectMeta{Name: "stamp-fail.example.org", Namespace: "default"},
		Spec: dnsv1alpha2.ZoneSpec{
			Kind:        NATIVE_KIND_ZONE,
			Nameservers: []string{"ns1.stamp-fail.example.org"},
			SOAEditAPI:  ptr.To("DEFAULT"),
		},
	}

	err := gzr.zoneExternalResourcesReconcile(context.Background(), &powerdns.Zone{}, zone)
	if !errors.Is(err, errNSStampAfterCreate) {
		t.Fatalf("got %v, want errors.Is(..., errNSStampAfterCreate)", err)
	}

	if _, ok := readFromZonesMap(makeCanonical(zone.Name)); !ok {
		t.Fatal("expected zone to exist after Zones.Add succeeded")
	}

	gzr.PDNSClient.Records = m.Records
	existing, ok := readFromZonesMap(makeCanonical(zone.Name))
	if !ok {
		t.Fatal("zone missing before retry")
	}
	if err := gzr.zoneExternalResourcesReconcile(context.Background(), existing, zone); err != nil {
		t.Fatalf("retry stamp: %v", err)
	}
	if got := getMockedCommentAccount(zone.Name, "NS"); got != OperatorAccount {
		t.Fatalf("NS account after retry: got %q, want %q", got, OperatorAccount)
	}
}

func TestNSStampAfterCreateSentinelNotStickyFailed(t *testing.T) {
	stampErr := fmt.Errorf("%w: %w", errNSStampAfterCreate, errors.New("pdns boom"))
	if !errors.Is(stampErr, errNSStampAfterCreate) {
		t.Fatal("expected stamp sentinel")
	}

	other := fmt.Errorf("failed to update NS in external resource: %w", errors.New("boom"))
	if errors.Is(other, errNSStampAfterCreate) {
		t.Fatal("unrelated error must not match stamp sentinel")
	}

	zone := &dnsv1alpha2.Zone{}
	if !errors.Is(stampErr, errNSStampAfterCreate) {
		zone.SetSynchronizationFailed(stampErr)
	}
	if zone.Status.SyncStatus != nil {
		t.Fatalf("SyncStatus should remain unset, got %v", *zone.Status.SyncStatus)
	}

	zone.SetSynchronizationFailed(other)
	if zone.Status.SyncStatus == nil || *zone.Status.SyncStatus != dnsv1alpha2.FAILED_STATUS {
		t.Fatal("unrelated sync error should set Failed")
	}
}

type errMetadataClient struct {
	getErr    error
	deleteErr error
	md        *powerdns.Metadata
}

func (e errMetadataClient) Get(ctx context.Context, domain string, kind powerdns.MetadataKind) (*powerdns.Metadata, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	if e.md != nil {
		return e.md, nil
	}
	return nil, powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Message: NOT_FOUND_ERROR_MSG}
}

func (e errMetadataClient) Set(ctx context.Context, domain string, kind powerdns.MetadataKind, values []string) (*powerdns.Metadata, error) {
	return nil, nil
}

func (e errMetadataClient) Delete(ctx context.Context, domain string, kind powerdns.MetadataKind) error {
	return e.deleteErr
}

func TestClearOrphanZoneMetadata(t *testing.T) {
	resetMetadataMap()
	defer resetMetadataMap()

	m := NewMockClient()
	pdns := PdnsClienter{Metadata: m.Metadata}
	logger := log.FromContext(context.Background())
	zone := "managed.example.org."

	if err := clearOrphanZoneMetadata(context.Background(), zone, pdns, logger); err != nil {
		t.Fatalf("no metadata: %v", err)
	}

	value := formatOrphanSinceEpoch(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	if _, err := m.Metadata.Set(context.Background(), zone, OrphanSinceMetadataKind, []string{value}); err != nil {
		t.Fatal(err)
	}
	if err := clearOrphanZoneMetadata(context.Background(), zone, pdns, logger); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := m.Metadata.Get(context.Background(), zone, OrphanSinceMetadataKind); !isPDNSNotFound(err) {
		t.Fatalf("expected cleared, got %v", err)
	}

	if err := clearOrphanZoneMetadata(context.Background(), zone, PdnsClienter{}, logger); err != nil {
		t.Fatalf("nil metadata: %v", err)
	}
}

func TestClearOrphanZoneMetadataErrors(t *testing.T) {
	logger := log.FromContext(context.Background())
	zone := "managed.example.org."
	apiErr := &powerdns.Error{StatusCode: 500, Message: "Internal Server Error"}

	if err := clearOrphanZoneMetadata(context.Background(), zone, PdnsClienter{Metadata: errMetadataClient{getErr: apiErr}}, logger); err == nil {
		t.Fatal("expected lookup error")
	}

	if err := clearOrphanZoneMetadata(context.Background(), zone, PdnsClienter{Metadata: errMetadataClient{
		md: &powerdns.Metadata{
			Kind:     powerdns.MetadataKindPtr(OrphanSinceMetadataKind),
			Metadata: []string{"1785578400"},
		},
		deleteErr: apiErr,
	}}, logger); err == nil {
		t.Fatal("expected delete error")
	}

	if err := clearOrphanZoneMetadata(context.Background(), zone, PdnsClienter{Metadata: errMetadataClient{
		getErr: powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Message: NOT_FOUND_ERROR_MSG},
	}}, logger); err != nil {
		t.Fatalf("404 get: %v", err)
	}

	if err := clearOrphanZoneMetadata(context.Background(), zone, PdnsClienter{Metadata: errMetadataClient{
		md: &powerdns.Metadata{
			Kind:     powerdns.MetadataKindPtr(OrphanSinceMetadataKind),
			Metadata: []string{"1785578400"},
		},
		deleteErr: powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Message: NOT_FOUND_ERROR_MSG},
	}}, logger); err != nil {
		t.Fatalf("404 delete: %v", err)
	}
}
