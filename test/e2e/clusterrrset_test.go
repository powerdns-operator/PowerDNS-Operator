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

var _ = Describe("ClusterRRset", Ordered, func() {
	const (
		zoneName   = "clusterrrset-e2e.example.com"
		rrsetName  = "www.clusterrrset-e2e.example.com"
		recordFQDN = "www.clusterrrset-e2e.example.com"
	)
	ctx := context.Background()

	zoneManifest := fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterZone
metadata:
  name: %s
spec:
  kind: Native
  nameservers:
    - ns1.%s
    - ns2.%s
`, zoneName, zoneName, zoneName)

	rrsetManifest := fmt.Sprintf(`
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterRRset
metadata:
  name: %s
spec:
  type: A
  name: "www"
  ttl: 300
  records:
    - 8.8.8.8
  zoneRef:
    name: %s
    kind: ClusterZone
`, rrsetName, zoneName)

	BeforeAll(func() {
		By("creating the parent ClusterZone")
		Expect(utils.ApplyManifest(zoneManifest)).To(Succeed())
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).NotTo(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	AfterAll(func() {
		Expect(utils.DeleteManifest(rrsetManifest)).To(Succeed())
		Expect(utils.DeleteManifest(zoneManifest)).To(Succeed())
	})

	It("should create a cluster rrset referencing a ClusterZone", func() {
		By("applying the ClusterRRset resource")
		Expect(utils.ApplyManifest(rrsetManifest)).To(Succeed())
		expectSyncSucceeded("clusterrrset", rrsetName, "")

		By("checking the record exists in PowerDNS")
		Eventually(func(g Gomega) {
			rrset, err := findRRset(ctx, zoneName, recordFQDN, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(*rrset.TTL).To(Equal(uint32(300)))
			g.Expect(rrsetContents(rrset)).To(ConsistOf("8.8.8.8"))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("should delete the cluster rrset from PowerDNS", func() {
		By("deleting the ClusterRRset resource")
		Expect(utils.DeleteManifest(rrsetManifest)).To(Succeed())

		By("checking the record no longer exists in PowerDNS")
		Eventually(func(g Gomega) {
			_, err := findRRset(ctx, zoneName, recordFQDN, powerdns.RRTypeA)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})
})
