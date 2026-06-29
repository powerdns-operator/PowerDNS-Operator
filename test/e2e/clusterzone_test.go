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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joeig/go-powerdns/v3"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

var _ = Describe("ClusterZone", Ordered, func() {
	const zoneName = "clusterzone-e2e.example.com"
	ctx := context.Background()

	manifest := getClusterZoneManifest(
		zoneName,
		"Native",
		"ns1."+zoneName,
		"ns2."+zoneName,
	)

	It("should create the cluster zone in PowerDNS", func() {
		By("applying the ClusterZone resource")
		Expect(utils.ApplyManifest(manifest)).To(Succeed())

		By("checking the ClusterZone reaches Succeeded sync status")
		expectSyncSucceeded("clusterzone", zoneName, "")

		By("checking the zone exists in PowerDNS")
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
				canonical("ns1."+zoneName),
				canonical("ns2."+zoneName),
			))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("should delete the cluster zone from PowerDNS", func() {
		By("deleting the ClusterZone resource")
		Expect(utils.DeleteManifest(manifest)).To(Succeed())

		By("checking the zone no longer exists in PowerDNS")
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})
})
