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

var _ = Describe("Zone", Ordered, func() {
	const (
		namespace = "e2e-zone"
		zoneName  = "zone-e2e.example.com"
	)
	ctx := context.Background()

	zoneManifest := func(nameservers string) string {
		return fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: Zone
metadata:
  name: %s
  namespace: %s
spec:
  kind: Native
  nameservers:
%s
`, zoneName, namespace, nameservers)
	}

	BeforeAll(func() {
		Expect(utils.CreateNamespace(namespace)).To(Succeed())
	})

	AfterAll(func() {
		utils.DeleteNamespace(namespace)
	})

	It("should create the zone in PowerDNS", func() {
		By("applying the Zone resource")
		Expect(utils.ApplyManifest(zoneManifest(
			"    - ns1.zone-e2e.example.com\n    - ns2.zone-e2e.example.com",
		))).To(Succeed())

		By("checking the Zone reaches Succeeded sync status")
		expectSyncSucceeded("zone", zoneName, namespace)

		By("checking the zone exists in PowerDNS with the expected kind")
		Eventually(func(g Gomega) {
			zone, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(zone.Kind).NotTo(BeNil())
			g.Expect(string(*zone.Kind)).To(Equal("Native"))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		By("checking the apex NS records were created in PowerDNS")
		Eventually(func(g Gomega) {
			ns, err := findRRset(ctx, zoneName, zoneName, powerdns.RRTypeNS)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rrsetContents(ns)).To(ConsistOf(
				canonical("ns1.zone-e2e.example.com"),
				canonical("ns2.zone-e2e.example.com"),
			))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("should update the nameservers in PowerDNS", func() {
		By("updating the Zone nameservers")
		Expect(utils.ApplyManifest(zoneManifest(
			"    - ns1.zone-e2e.example.com\n    - ns3.zone-e2e.example.com",
		))).To(Succeed())

		By("checking the NS records are updated in PowerDNS")
		Eventually(func(g Gomega) {
			ns, err := findRRset(ctx, zoneName, zoneName, powerdns.RRTypeNS)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rrsetContents(ns)).To(ConsistOf(
				canonical("ns1.zone-e2e.example.com"),
				canonical("ns3.zone-e2e.example.com"),
			))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("should delete the zone from PowerDNS", func() {
		By("deleting the Zone resource")
		Expect(utils.DeleteManifest(zoneManifest(
			"    - ns1.zone-e2e.example.com\n    - ns3.zone-e2e.example.com",
		))).To(Succeed())

		By("checking the zone no longer exists in PowerDNS")
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})
})
