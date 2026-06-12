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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joeig/go-powerdns/v3"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

var _ = Describe("RRset", Ordered, func() {
	const (
		namespace = "e2e-rrset"
		zoneName  = "rrset-e2e.example.com"
	)
	ctx := context.Background()

	rrsetManifest := func(metaName, recordName, recordType, ttl, records string) string {
		return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: RRset
metadata:
  name: %s
  namespace: %s
spec:
  type: %s
  name: %q
  ttl: %s
  records:
%s
  zoneRef:
    name: %s
    kind: Zone
`, metaName, namespace, recordType, recordName, ttl, records, zoneName)
	}

	BeforeAll(func() {
		Expect(utils.CreateNamespace(namespace)).To(Succeed())

		By("creating the parent Zone")
		applyAndDeferDelete(fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: Zone
metadata:
  name: %s
  namespace: %s
spec:
  kind: Native
  nameservers:
    - ns1.%s
    - ns2.%s
`, zoneName, namespace, zoneName, zoneName))

		By("waiting for the parent Zone to exist in PowerDNS")
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).NotTo(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		DeferCleanup(func() { utils.DeleteNamespace(namespace) })
	})

	It("should create records of various types in PowerDNS", func() {
		cases := []struct {
			metaName   string
			recordName string
			recordType string
			records    string
			fqdn       string
			rrType     powerdns.RRType
			want       []string
		}{
			{
				metaName:   "a.rrset-e2e.example.com",
				recordName: "a",
				recordType: "A",
				records:    "    - 1.1.1.1\n    - 2.2.2.2",
				fqdn:       "a.rrset-e2e.example.com",
				rrType:     powerdns.RRTypeA,
				want:       []string{"1.1.1.1", "2.2.2.2"},
			},
			{
				metaName:   "aaaa.rrset-e2e.example.com",
				recordName: "aaaa",
				recordType: "AAAA",
				records:    "    - 2001:db8::1",
				fqdn:       "aaaa.rrset-e2e.example.com",
				rrType:     powerdns.RRTypeAAAA,
				want:       []string{"2001:db8::1"},
			},
			{
				metaName:   "cname.rrset-e2e.example.com",
				recordName: "cname",
				recordType: "CNAME",
				records:    "    - a.rrset-e2e.example.com.",
				fqdn:       "cname.rrset-e2e.example.com",
				rrType:     powerdns.RRTypeCNAME,
				want:       []string{"a.rrset-e2e.example.com."},
			},
			{
				metaName:   "txt.rrset-e2e.example.com",
				recordName: "txt",
				recordType: "TXT",
				records:    `    - "\"hello e2e\""`,
				fqdn:       "txt.rrset-e2e.example.com",
				rrType:     powerdns.RRTypeTXT,
				want:       []string{`"hello e2e"`},
			},
			{
				metaName:   "mx.rrset-e2e.example.com",
				recordName: "rrset-e2e.example.com.",
				recordType: "MX",
				records:    "    - \"10 mail.rrset-e2e.example.com.\"",
				fqdn:       "rrset-e2e.example.com",
				rrType:     powerdns.RRTypeMX,
				want:       []string{"10 mail.rrset-e2e.example.com."},
			},
		}

		for _, tc := range cases {
			By(fmt.Sprintf("creating the %s RRset", tc.recordType))
			Expect(utils.ApplyManifest(rrsetManifest(
				tc.metaName, tc.recordName, tc.recordType, "300", tc.records,
			))).To(Succeed())
			expectSyncSucceeded("rrset", tc.metaName, namespace)

			By(fmt.Sprintf("checking the %s record exists in PowerDNS", tc.recordType))
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, zoneName, tc.fqdn, tc.rrType)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf(tc.want))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		}
	})

	It("should update an existing record in PowerDNS", func() {
		By("updating the A record TTL and contents")
		Expect(utils.ApplyManifest(rrsetManifest(
			"a.rrset-e2e.example.com", "a", "A", "600", "    - 9.9.9.9",
		))).To(Succeed())

		By("checking the change is reflected in PowerDNS")
		Eventually(func(g Gomega) {
			rrset, err := findRRset(ctx, zoneName, "a.rrset-e2e.example.com", powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(*rrset.TTL).To(Equal(uint32(600)))
			g.Expect(rrsetContents(rrset)).To(ConsistOf("9.9.9.9"))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("should delete a record from PowerDNS", func() {
		By("deleting the A RRset")
		Expect(utils.DeleteManifest(rrsetManifest(
			"a.rrset-e2e.example.com", "a", "A", "600", "    - 9.9.9.9",
		))).To(Succeed())

		By("checking the record no longer exists in PowerDNS")
		Eventually(func(g Gomega) {
			_, err := findRRset(ctx, zoneName, "a.rrset-e2e.example.com", powerdns.RRTypeA)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})
})
