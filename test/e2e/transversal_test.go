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

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

var _ = Describe("Transversal", Ordered, func() {
	ctx := context.Background()
	namespaces := []string{"example1", "example2", "example3"}
	clusterZoneHelloworldManifest := getManifestFromFile("testdata/ClusterZone-helloworld.com.yaml")
	clusterZoneInAddrArpaManifest := getManifestFromFile("testdata/ClusterZone-1.168.192.in-addr.arpa.yaml")
	zoneExample1Manifest := getManifestFromFile("testdata/Zone-example1-example1.com.yaml")
	zoneDuplicatedHelloworldManifest := getManifestFromFile("testdata/Zone-example3-helloworld.com.yaml")
	zoneExample2Manifest := getManifestFromFile("testdata/Zone-example2-example2.com.yaml")
	zoneDuplicatedExample2Manifest := getManifestFromFile("testdata/Zone-example3-example2.com.yaml")

	clusterRRsetMXHelloworldManifest := getManifestFromFile("testdata/ClusterRRset-mx.helloworld.com.yaml")
	clusterRRsetTestHelloworldManifest := getManifestFromFile("testdata/ClusterRRset-test.helloworld.com.yaml")
	clusterRRsetTestDuplicatedHelloworldManifest := getManifestFromFile("testdata/ClusterRRset-test-duplicated.helloworld.com.yaml")
	RRsetDefaultInAddrArpaHelloworldManifest := getManifestFromFile("testdata/RRset-default-1.1.168.192.in-addr.arpa.helloworld.com.yaml")
	RRsetDefaultDatabaseSrvHelloworldManifest := getManifestFromFile("testdata/RRset-default-database.srv.helloworld.com.yaml")
	RRsetDefaultMXHelloworldManifest := getManifestFromFile("testdata/RRset-default-mx.helloworld.com.yaml")
	RRsetDefaultTestHelloworldManifest := getManifestFromFile("testdata/RRset-default-test.helloworld.com.yaml")
	RRsetDefaultTest1HelloworldManifest := getManifestFromFile("testdata/RRset-default-test1.helloworld.com.yaml")
	RRsetDefaultTest1FailedHelloworldManifest := getManifestFromFile("testdata/RRset-default-test1failed.helloworld.com.yaml")
	RRsetDefaultTest2HelloworldManifest := getManifestFromFile("testdata/RRset-default-test2.helloworld.com.yaml")
	RRsetDefaultTest3IPv4HelloworldManifest := getManifestFromFile("testdata/RRset-default-test3-ipv4.helloworld.com.yaml")
	RRsetDefaultTest3IPv6HelloworldManifest := getManifestFromFile("testdata/RRset-default-test3-ipv6.helloworld.com.yaml")
	RRsetDefaultTest4HelloworldManifest := getManifestFromFile("testdata/RRset-default-test4.helloworld.com.yaml")
	RRsetDefaultTest5HelloworldManifest := getManifestFromFile("testdata/RRset-default-test5.helloworld.com.yaml")
	RRsetDefaultTest6OKHelloworldManifest := getManifestFromFile("testdata/RRset-default-test6-ok.helloworld.com.yaml")
	RRsetDefaultTest6HelloworldManifest := getManifestFromFile("testdata/RRset-default-test6.helloworld.com.yaml")
	RRsetDefaultTXTHelloworldManifest := getManifestFromFile("testdata/RRset-default-txt.helloworld.com.yaml")
	RRsetDefaultTestNozoneHelloworldManifest := getManifestFromFile("testdata/RRset-default-test.nozone.com.yaml")

	BeforeAll(func() {
		for _, namespace := range namespaces {
			Expect(utils.CreateNamespace(namespace)).To(Succeed())
		}
		DeferCleanup(func() {
			for _, namespace := range namespaces {
				utils.DeleteNamespace(namespace)
			}
		})
	})

	AfterAll(func() {
		utils.DeleteManifest(clusterZoneHelloworldManifest)
		utils.DeleteManifest(clusterZoneInAddrArpaManifest)
		utils.DeleteManifest(zoneExample1Manifest)
		utils.DeleteManifest(zoneDuplicatedHelloworldManifest)
		utils.DeleteManifest(zoneExample2Manifest)
		utils.DeleteManifest(zoneDuplicatedExample2Manifest)
		utils.DeleteManifest(clusterRRsetMXHelloworldManifest)
		utils.DeleteManifest(clusterRRsetTestHelloworldManifest)
		utils.DeleteManifest(clusterRRsetTestDuplicatedHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultInAddrArpaHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultDatabaseSrvHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultMXHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTestHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest1HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest1FailedHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest2HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest3IPv4HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest3IPv6HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest4HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest5HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest6OKHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTest6HelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTXTHelloworldManifest)
		utils.DeleteManifest(RRsetDefaultTestNozoneHelloworldManifest)
	})

	Context("When applying a bunch of manifests", func() {
		It("should successfully create a ClusterZone for the helloworld.com", func() {
			By("applying the ClusterZone resource")
			Expect(utils.ApplyManifest(clusterZoneHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("clusterzone", "helloworld.com", "")

			By("checking the Zone exists in PowerDNS")
			Eventually(func(g Gomega) {
				_, err := pdnsClient.Zones.Get(ctx, "helloworld.com")
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a ClusterZone for the in-addr.arpa", func() {
			By("applying the ClusterZone resource")
			Expect(utils.ApplyManifest(clusterZoneInAddrArpaManifest)).To(Succeed())
			expectSyncSucceeded("clusterzone", "1.168.192.in-addr.arpa", "")

			By("checking the Zone exists in PowerDNS")
			Eventually(func(g Gomega) {
				_, err := pdnsClient.Zones.Get(ctx, "1.168.192.in-addr.arpa")
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a Zone for the example1.com", func() {
			By("applying the Zone resource")
			Expect(utils.ApplyManifest(zoneExample1Manifest)).To(Succeed())
			expectSyncSucceeded("zone", "example1.com", "example1")

			By("checking the Zone exists in PowerDNS")
			Eventually(func(g Gomega) {
				_, err := pdnsClient.Zones.Get(ctx, "example1.com")
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a duplicate Zone for the helloworld.com", func() {
			By("applying the Zone resource")
			Expect(utils.ApplyManifest(zoneDuplicatedHelloworldManifest)).To(Succeed())
			expectSyncFailed("zone", "helloworld.com", "example3")
		})

		It("should successfully create a Zone for the example2.com", func() {
			By("applying the Zone resource")
			Expect(utils.ApplyManifest(zoneExample2Manifest)).To(Succeed())
			expectSyncSucceeded("zone", "example2.com", "example2")

			By("checking the Zone exists in PowerDNS")
			Eventually(func(g Gomega) {
				_, err := pdnsClient.Zones.Get(ctx, "example2.com")
				g.Expect(err).NotTo(HaveOccurred())
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a duplicated Zone for the example2.com", func() {
			By("applying the Zone resource")
			Expect(utils.ApplyManifest(zoneDuplicatedExample2Manifest)).To(Succeed())
			expectSyncFailed("zone", "example2.com", "example3")
		})

		It("should successfully create a ClusterRRset for mx.helloworld.com", func() {
			By("applying the ClusterRRset resource")
			Expect(utils.ApplyManifest(clusterRRsetMXHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("clusterrrset", "mx.helloworld.com", "")

			By("checking the MX record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "helloworld.com.", powerdns.RRTypeMX)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("10 mx1.helloworld.com.", "20 mx2.helloworld.com."))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a ClusterRRset for test.helloworld.com", func() {
			By("applying the ClusterRRset resource")
			Expect(utils.ApplyManifest(clusterRRsetTestHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("clusterrrset", "test.helloworld.com", "")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("1.1.1.1", "2.2.2.2"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a duplicated ClusterRRset for test.helloworld.com", func() {
			By("applying the ClusterRRset resource")
			Expect(utils.ApplyManifest(clusterRRsetTestDuplicatedHelloworldManifest)).To(Succeed())
			expectSyncFailed("clusterrrset", "test-duplicated.helloworld.com", "")
		})

		It("should successfully create a RRset for 1.1.168.192.in-addr.arpa.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultInAddrArpaHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "1.1.168.192.in-addr.arpa.helloworld.com", "")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "1.168.192.in-addr.arpa", "1.1.168.192.in-addr.arpa", powerdns.RRTypePTR)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("mailserver.helloworld.com."))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for database.srv.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultDatabaseSrvHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "database.srv.helloworld.com", "")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "_database._tcp.myapp.helloworld.com", powerdns.RRTypeSRV)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("1 50 25565 test2.helloworld.com."))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a duplicated RRset for mx.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultMXHelloworldManifest)).To(Succeed())
			expectSyncFailed("rrset", "mx.helloworld.com", "default")
		})

		It("should fail to create a duplicated RRset for test.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTestHelloworldManifest)).To(Succeed())
			expectSyncFailed("rrset", "test.helloworld.com", "default")
		})

		It("should successfully create a RRset for test1.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest1HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test1.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test1.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("3.3.3.3"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a duplicated RRset for test1failed.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest1FailedHelloworldManifest)).To(Succeed())
			expectSyncFailed("rrset", "test1failed.helloworld.com", "default")
		})

		It("should successfully create a RRset for test2.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest2HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test2.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test2.helloworld.com.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("4.4.4.4"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for test3-ipv4.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest3IPv4HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test3-ipv4.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test3.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("192.168.1.7"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for test3-ipv6.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest3IPv6HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test3-ipv6.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test3.helloworld.com", powerdns.RRTypeAAAA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("2001:dc8:86a4::7a2f:2360:2341"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for test4.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest4HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test4.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test4.helloworld.com", powerdns.RRTypeCNAME)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("test1.helloworld.com."))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for test5.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest5HelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test5.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "*.test5.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("5.5.5.5"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should successfully create a RRset for test6-ok.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest6OKHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "test6-ok.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "test6.helloworld.com", powerdns.RRTypeA)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("192.168.1.7"))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a RRset for test6.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTest6HelloworldManifest)).To(Succeed())
			expectSyncFailed("rrset", "test6.helloworld.com", "default")
		})

		It("should successfully create a RRset for txt.helloworld.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTXTHelloworldManifest)).To(Succeed())
			expectSyncSucceeded("rrset", "txt.helloworld.com", "default")

			By("checking the record exists in PowerDNS")
			Eventually(func(g Gomega) {
				rrset, err := findRRset(ctx, "helloworld.com", "helloworld.com", powerdns.RRTypeTXT)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*rrset.TTL).To(Equal(uint32(300)))
				g.Expect(rrsetContents(rrset)).To(ConsistOf("\"Welcome to the helloworld.com domain\""))
			}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
		})

		It("should fail to create a RRset for test.nozone.com", func() {
			By("applying the RRset resource")
			Expect(utils.ApplyManifest(RRsetDefaultTestNozoneHelloworldManifest)).To(Succeed())
			expectSyncPending("rrset", "test.nozone.com", "default")
		})
	})
})
