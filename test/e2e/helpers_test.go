//go:build e2e

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

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joeig/go-powerdns/v3"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

const (
	// pollTimeout/pollInterval bound the reconcile-latency tolerance of the
	// Eventually-based assertions.
	pollTimeout  = 2 * time.Minute
	pollInterval = 2 * time.Second
)

// canonical appends a trailing dot if missing, matching PowerDNS representation.
func canonical(name string) string {
	return strings.TrimSuffix(name, ".") + "."
}

// findRRset returns the RRset of the given canonical name and type in a zone, or
// an error if it is absent.
func findRRset(ctx context.Context, zone, name string, rrType powerdns.RRType) (*powerdns.RRset, error) {
	want := canonical(name)
	records, err := pdnsClient.Records.Get(ctx, zone, want, &rrType)
	if err != nil {
		return nil, err
	}
	for i := range records {
		r := records[i]
		if r.Name != nil && *r.Name == want && r.Type != nil && *r.Type == rrType {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("rrset %s/%s not found in zone %s", name, rrType, zone)
}

// rrsetContents returns the record contents of an RRset as a string slice.
func rrsetContents(rrset *powerdns.RRset) []string {
	out := make([]string, 0, len(rrset.Records))
	for _, r := range rrset.Records {
		if r.Content != nil {
			out = append(out, *r.Content)
		}
	}
	return out
}

// expectSyncSucceeded waits until the resource reports a "Succeeded" syncStatus.
// namespace may be empty for cluster-scoped resources.
func expectSyncSucceeded(kind, name, namespace string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		status, err := utils.GetResourceField(kind, name, namespace, "{.status.syncStatus}")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(status).To(Equal("Succeeded"))
	}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
}

// applyAndDeferDelete applies a manifest and schedules its deletion at the end
// of the current spec/container.
func applyAndDeferDelete(manifest string) {
	GinkgoHelper()
	Expect(utils.ApplyManifest(manifest)).To(Succeed())
	DeferCleanup(func() {
		Expect(utils.DeleteManifest(manifest)).To(Succeed())
	})
}

func getClusterZoneManifest(zoneName, kind, ns1, ns2 string) string {
	return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterZone
metadata:
  name: %s
spec:
  kind: %s
  nameservers:
    - %s
    - %s
`, zoneName, kind, ns1, ns2)
}

func getZoneManifest(name, namespace, kind, ns1, ns2 string) string {
	return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: Zone
metadata:
  name: %s
  namespace: %s
spec:
  kind: %s
  nameservers:
    - %s
    - %s
`, name, namespace, kind, ns1, ns2)
}

func getClusterRRsetManifest(name, recordType, recordName, recordTTL, recordContent, zoneName, kind string) string {
	return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterRRset
metadata:
  name: %s
spec:
  type: %s
  name: %s
  ttl: %s
  records: %s
  zoneRef:
    name: %s
    kind: %s
`, name, recordType, recordName, recordTTL, recordContent, zoneName, kind)
}

func getRRsetManifest(name, namespace, recordType, recordName, recordTTL, recordContent, zoneName, kind string) string {
	return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: RRset
metadata:
  name: %s
  namespace: %s
spec:
  type: %s
  name: %s
  ttl: %s
  records: %s
  zoneRef:
    name: %s
    kind: %s
`, name, namespace, recordType, recordName, recordTTL, recordContent, zoneName, kind)
}
