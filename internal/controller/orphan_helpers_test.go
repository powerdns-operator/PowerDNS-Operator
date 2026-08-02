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
	"testing"
	"time"

	"github.com/joeig/go-powerdns/v3"
	"k8s.io/utils/ptr"
)

func TestFormatParseOrphanSinceComment(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	got := formatOrphanSinceComment(now)
	want := orphanSinceCommentPrefix + "1785672000"
	if got != want {
		t.Fatalf("format: got %q, want %q", got, want)
	}
	parsed, ok := parseOrphanSinceComment(got)
	if !ok {
		t.Fatal("parse comment: ok=false")
	}
	if !parsed.Equal(now) {
		t.Errorf("parse comment: got %v, want %v", parsed, now)
	}
}

func TestParseOrphanSinceCommentRejects(t *testing.T) {
	cases := []string{
		"",
		"user comment",
		"1785672000", // bare epoch is metadata-only
		orphanSinceCommentPrefix + "not-a-time",
		orphanSinceCommentPrefix,
	}
	for _, c := range cases {
		if _, ok := parseOrphanSinceComment(c); ok {
			t.Errorf("parseOrphanSinceComment(%q) ok=true, want false", c)
		}
	}
}

func TestFormatParseOrphanSinceEpoch(t *testing.T) {
	now := time.Date(2026, 8, 2, 15, 30, 0, 0, time.UTC)
	got := formatOrphanSinceEpoch(now)
	if got != "1785684600" {
		t.Fatalf("format epoch: got %q", got)
	}
	parsed, ok := parseOrphanSinceEpoch(got)
	if !ok || !parsed.Equal(now) {
		t.Fatalf("parse epoch: ok=%v got=%v", ok, parsed)
	}
	if _, ok := parseOrphanSinceEpoch(""); ok {
		t.Error("empty epoch should not parse")
	}
	if _, ok := parseOrphanSinceEpoch("bogus"); ok {
		t.Error("bogus epoch should not parse")
	}
}

func TestOrphanSinceAged(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	grace := time.Hour
	since := now.Add(-time.Hour)
	if !orphanSinceAged(since, grace, now) {
		t.Error("exactly grace old should be aged")
	}
	if orphanSinceAged(now.Add(-30*time.Minute), grace, now) {
		t.Error("younger than grace should not be aged")
	}
	if orphanSinceAged(since, 0, now) {
		t.Error("grace<=0 should never age")
	}
	if !orphanSinceAged(now.Add(-2*time.Hour), grace, now) {
		t.Error("older than grace should be aged")
	}
	// Future stamp must not be treated as aged (fail closed).
	if orphanSinceAged(now.Add(time.Hour), grace, now) {
		t.Error("future since should not be aged")
	}
}

func TestRrsetStillOrphanAged(t *testing.T) {
	resetRecordsMap()
	defer resetRecordsMap()

	m := NewMockClient()
	pdns := PdnsClienter{Records: m.Records, Zones: m.Zones, Metadata: m.Metadata}
	zone := "example.org."
	name := "orphan.example.org."
	rrType := powerdns.RRTypeA
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	grace := time.Hour

	// No RRset → not orphan aged
	ok, err := rrsetStillOrphanAged(t.Context(), zone, name, rrType, grace, now, pdns)
	if err != nil || ok {
		t.Fatalf("missing rrset: ok=%v err=%v", ok, err)
	}

	agedMarker := formatOrphanSinceComment(now.Add(-2 * time.Hour))
	_ = pdns.Records.Change(t.Context(), zone, name, rrType, 300, []string{"1.2.3.4"}, powerdns.WithComments(powerdns.Comment{
		Content: ptr.To(agedMarker),
		Account: ptr.To(OperatorAccount),
	}))

	ok, err = rrsetStillOrphanAged(t.Context(), zone, name, rrType, grace, now, pdns)
	if err != nil || !ok {
		t.Fatalf("aged marker: ok=%v err=%v", ok, err)
	}

	// Fresh marker → not aged
	_ = pdns.Records.Change(t.Context(), zone, name, rrType, 300, []string{"1.2.3.4"}, powerdns.WithComments(orphanSinceComment(now)))
	ok, err = rrsetStillOrphanAged(t.Context(), zone, name, rrType, grace, now, pdns)
	if err != nil || ok {
		t.Fatalf("fresh marker: ok=%v err=%v want false", ok, err)
	}

	// Managed comment (no marker) → not orphan aged
	_ = pdns.Records.Change(t.Context(), zone, name, rrType, 300, []string{"1.2.3.4"}, powerdns.WithComments(operatorComment(ptr.To("user"))))
	ok, err = rrsetStillOrphanAged(t.Context(), zone, name, rrType, grace, now, pdns)
	if err != nil || ok {
		t.Fatalf("managed comment: ok=%v err=%v want false", ok, err)
	}
}

func TestOperatorCommentSanitizesOrphanSincePrefix(t *testing.T) {
	marker := formatOrphanSinceComment(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	got := operatorComment(&marker)
	if ptr.Deref(got.Content, "x") != "" {
		t.Errorf("user comment with orphan-since prefix should be stripped, got %q", ptr.Deref(got.Content, ""))
	}
	if ptr.Deref(got.Account, "") != OperatorAccount {
		t.Errorf("account: got %q", ptr.Deref(got.Account, ""))
	}

	normal := "keep me"
	got = operatorComment(&normal)
	if ptr.Deref(got.Content, "") != normal {
		t.Errorf("normal comment: got %q, want %q", ptr.Deref(got.Content, ""), normal)
	}
}

func TestOrphanSinceCommentPreservesMarker(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	got := orphanSinceComment(now)
	want := formatOrphanSinceComment(now)
	if ptr.Deref(got.Content, "") != want {
		t.Errorf("flag comment: got %q, want %q", ptr.Deref(got.Content, ""), want)
	}
	if _, ok := parseOrphanSinceComment(ptr.Deref(got.Content, "")); !ok {
		t.Error("flag comment should parse as orphan-since")
	}
}

func TestIsPDNSNotFound(t *testing.T) {
	if !isPDNSNotFound(powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Message: NOT_FOUND_ERROR_MSG}) {
		t.Error("value Error 404")
	}
	if !isPDNSNotFound(&powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Message: NOT_FOUND_ERROR_MSG}) {
		t.Error("pointer Error 404")
	}
	if isPDNSNotFound(powerdns.Error{StatusCode: 500, Message: " Internal Server Error"}) {
		t.Error("500 should not be not-found")
	}
	if isPDNSNotFound(nil) {
		t.Error("nil should not be not-found")
	}
}

func TestMockMetadataRoundTrip(t *testing.T) {
	resetMetadataMap()
	m := NewMockClient()
	domain := "orphan.test."
	_, err := m.Metadata.Get(t.Context(), domain, OrphanSinceMetadataKind)
	if !isPDNSNotFound(err) {
		t.Fatalf("expected 404, got %v", err)
	}
	value := formatOrphanSinceEpoch(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if _, err := m.Metadata.Set(t.Context(), domain, OrphanSinceMetadataKind, []string{value}); err != nil {
		t.Fatal(err)
	}
	md, err := m.Metadata.Get(t.Context(), domain, OrphanSinceMetadataKind)
	if err != nil {
		t.Fatal(err)
	}
	if len(md.Metadata) != 1 || md.Metadata[0] != value {
		t.Fatalf("got %#v", md)
	}
	if err := m.Metadata.Delete(t.Context(), domain, OrphanSinceMetadataKind); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Metadata.Get(t.Context(), domain, OrphanSinceMetadataKind); !isPDNSNotFound(err) {
		t.Fatalf("expected 404 after delete, got %v", err)
	}
}
