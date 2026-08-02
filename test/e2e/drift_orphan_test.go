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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joeig/go-powerdns/v3"
	"k8s.io/utils/ptr"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

// Serial so manager arg patches do not race other Ordered Describes.
var _ = Describe("Drift and orphans", Serial, Ordered, func() {
	const (
		zoneName       = "drift-orphan-e2e.example.com"
		commentedRR    = "www.drift-orphan-e2e.example.com"
		bareRR         = "api.drift-orphan-e2e.example.com"
		driftRR        = "drift.drift-orphan-e2e.example.com"
		recreateRR     = "recreate.drift-orphan-e2e.example.com"
		orphanRRname   = "orphan.drift-orphan-e2e.example.com"
		manualRRname   = "manual.drift-orphan-e2e.example.com"
		orphanZoneName = "orphan-zone-e2e.example.com"
		operatorAcct   = operatorAccount
		userComment    = "e2e user comment"
		driftInterval  = "5s"
		rrsetGrace     = "10s"
		zoneGrace      = "12s"
	)
	ctx := context.Background()

	var metricsPF *utils.PortForward

	zoneManifest := getClusterZoneManifest(zoneName, "Native", "ns1."+zoneName, "ns2."+zoneName)

	BeforeAll(func() {
		By("starting metrics port-forward")
		pf, err := utils.StartMetricsPortForward()
		Expect(err).NotTo(HaveOccurred())
		metricsPF = pf

		By("creating managed ClusterZone for the suite")
		Expect(utils.ApplyManifest(zoneManifest)).To(Succeed())
		expectSyncSucceeded("clusterzone", zoneName, "")

		DeferCleanup(func() {
			By("resetting manager args to defaults")
			_ = utils.ResetManagerArgs()
			_ = utils.DeleteManifest(zoneManifest)
			_ = pdnsClient.Zones.Delete(ctx, zoneName)
			_ = pdnsClient.Zones.Delete(ctx, orphanZoneName)
			if metricsPF != nil {
				metricsPF.Stop()
			}
		})
	})

	It("stamps managed markers on zone and rrsets", func() {
		By("applying ClusterRRsets with and without comment")
		comment := userComment
		applyAndDeferDelete(getClusterRRsetManifestWithComment(
			commentedRR, "A", "www", "300", `["1.1.1.1"]`, zoneName, "ClusterZone", &comment,
		))
		applyAndDeferDelete(getClusterRRsetManifestWithComment(
			bareRR, "A", "api", "300", `["2.2.2.2"]`, zoneName, "ClusterZone", nil,
		))

		expectSyncSucceeded("clusterrrset", commentedRR, "")
		expectSyncSucceeded("clusterrrset", bareRR, "")

		By("asserting PowerDNS account markers")
		Eventually(func(g Gomega) {
			zone, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ptr.Deref(zone.Account, "")).To(Equal(operatorAcct))

			ns, err := findRRset(ctx, zoneName, zoneName, powerdns.RRTypeNS)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(operatorCommentAccount(ns)).To(Equal(operatorAcct))

			www, err := findRRset(ctx, zoneName, commentedRR, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(operatorCommentAccount(www)).To(Equal(operatorAcct))
			g.Expect(operatorCommentText(www)).To(Equal(userComment))

			api, err := findRRset(ctx, zoneName, bareRR, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(operatorCommentAccount(api)).To(Equal(operatorAcct))
			g.Expect(operatorCommentText(api)).To(Equal(""))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("with drift-check-interval=0 does not raise orphan gauges on a timer", func() {
		Expect(pdnsClient.Records.Change(ctx, zoneName, orphanRRname, powerdns.RRTypeA, 300, []string{"9.9.9.9"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To(""),
				Account: ptr.To(operatorAcct),
			}),
		)).To(Succeed())
		DeferCleanup(func() {
			_ = pdnsClient.Records.Delete(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
		})

		Consistently(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			v, ok := utils.MetricValue(body, "powerdns_operator_orphan_rrsets", map[string]string{"zone": zoneName})
			if ok {
				g.Expect(v).To(BeZero())
			}
		}).WithTimeout(8 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("enables periodic drift checks", func() {
		By("patching manager with --drift-check-interval=" + driftInterval)
		Expect(utils.SetManagerArgs("--drift-check-interval="+driftInterval)).To(Succeed())
		restartMetricsPortForward(&metricsPF)
	})

	It("corrects managed RRset drift and increments corrections counter", func() {
		applyAndDeferDelete(getClusterRRsetManifestWithComment(
			driftRR, "A", "drift", "300", `["3.3.3.3"]`, zoneName, "ClusterZone", nil,
		))
		expectSyncSucceeded("clusterrrset", driftRR, "")

		body, err := utils.FetchMetrics()
		Expect(err).NotTo(HaveOccurred())
		before, _ := utils.MetricValue(body, "powerdns_operator_managed_corrections_total", map[string]string{"kind": "rrset"})

		By("patching PDNS out-of-band (keep operator account)")
		Expect(pdnsClient.Records.Change(ctx, zoneName, driftRR, powerdns.RRTypeA, 300, []string{"8.8.8.8"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To(""),
				Account: ptr.To(operatorAcct),
			}),
		)).To(Succeed())

		By("waiting for operator to restore desired records")
		Eventually(func(g Gomega) {
			rr, err := findRRset(ctx, zoneName, driftRR, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rrsetContents(rr)).To(ConsistOf("3.3.3.3"))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			after, ok := utils.MetricValue(body, "powerdns_operator_managed_corrections_total", map[string]string{"kind": "rrset"})
			g.Expect(ok).To(BeTrue())
			g.Expect(after).To(BeNumerically(">", before))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("recreates RRset after out-of-band PDNS delete", func() {
		applyAndDeferDelete(getClusterRRsetManifestWithComment(
			recreateRR, "A", "recreate", "300", `["4.4.4.4"]`, zoneName, "ClusterZone", nil,
		))
		expectSyncSucceeded("clusterrrset", recreateRR, "")

		Eventually(func(g Gomega) {
			rr, err := findRRset(ctx, zoneName, recreateRR, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rrsetContents(rr)).To(ConsistOf("4.4.4.4"))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		By("deleting the RRset from PowerDNS only")
		Expect(pdnsClient.Records.Delete(ctx, zoneName, recreateRR, powerdns.RRTypeA)).To(Succeed())
		Eventually(func(g Gomega) {
			_, err := findRRset(ctx, zoneName, recreateRR, powerdns.RRTypeA)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(30 * time.Second).WithPolling(pollInterval).Should(Succeed())

		By("waiting for operator reconcile to recreate the RRset")
		Eventually(func(g Gomega) {
			rr, err := findRRset(ctx, zoneName, recreateRR, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rrsetContents(rr)).To(ConsistOf("4.4.4.4"))
			g.Expect(operatorCommentAccount(rr)).To(Equal(operatorAcct))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("recreates zone after out-of-band PDNS delete", func() {
		By("deleting the managed zone from PowerDNS only")
		Expect(pdnsClient.Zones.Delete(ctx, zoneName)).To(Succeed())
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(30 * time.Second).WithPolling(pollInterval).Should(Succeed())

		By("waiting for operator reconcile to recreate the zone")
		Eventually(func(g Gomega) {
			zone, err := pdnsClient.Zones.Get(ctx, zoneName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(ptr.Deref(zone.Account, "")).To(Equal(operatorAcct))
			ns, err := findRRset(ctx, zoneName, zoneName, powerdns.RRTypeNS)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(operatorCommentAccount(ns)).To(Equal(operatorAcct))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})

	It("detects orphan RRsets without deleting (cleanup off)", func() {
		By("injecting operator-marked orphan and unmarked manual A")
		Expect(pdnsClient.Records.Change(ctx, zoneName, orphanRRname, powerdns.RRTypeA, 300, []string{"9.9.9.9"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To(""),
				Account: ptr.To(operatorAcct),
			}),
		)).To(Succeed())
		Expect(pdnsClient.Records.Change(ctx, zoneName, manualRRname, powerdns.RRTypeA, 300, []string{"7.7.7.7"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To("manual"),
				Account: ptr.To("admin"),
			}),
		)).To(Succeed())
		DeferCleanup(func() {
			_ = pdnsClient.Records.Delete(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
			_ = pdnsClient.Records.Delete(ctx, zoneName, manualRRname, powerdns.RRTypeA)
		})

		Eventually(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			v, ok := utils.MetricValue(body, "powerdns_operator_orphan_rrsets", map[string]string{"zone": zoneName})
			g.Expect(ok).To(BeTrue())
			g.Expect(v).To(BeNumerically(">=", 1))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		Consistently(func(g Gomega) {
			_, err := findRRset(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			_, err = findRRset(ctx, zoneName, manualRRname, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
		}).WithTimeout(8 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("detects orphan zones without deleting (cleanup off)", func() {
		By("creating an operator-marked PDNS zone with no CR")
		_, err := pdnsClient.Zones.Add(ctx, &powerdns.Zone{
			Name:        ptr.To(canonical(orphanZoneName)),
			Kind:        powerdns.ZoneKindPtr(powerdns.NativeZoneKind),
			Nameservers: []string{canonical("ns1." + orphanZoneName), canonical("ns2." + orphanZoneName)},
			Account:     ptr.To(operatorAcct),
		})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = pdnsClient.Zones.Delete(ctx, orphanZoneName)
		})

		By("verifying Zones.List exposes account (FAQ caveat)")
		zones, err := pdnsClient.Zones.List(ctx)
		Expect(err).NotTo(HaveOccurred())
		found := false
		for _, z := range zones {
			if canonical(ptr.Deref(z.Name, "")) == canonical(orphanZoneName) {
				found = true
				Expect(ptr.Deref(z.Account, "")).To(Equal(operatorAcct),
					"Zones.List omitted account; orphan zone detect cannot work on this PDNS")
			}
		}
		Expect(found).To(BeTrue())

		Eventually(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			v, ok := utils.MetricValue(body, "powerdns_operator_orphan_zones", nil)
			g.Expect(ok).To(BeTrue())
			g.Expect(v).To(BeNumerically(">=", 1))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		Consistently(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, orphanZoneName)
			g.Expect(err).NotTo(HaveOccurred())
		}).WithTimeout(8 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("enables orphan RRset and zone cleanup with short grace", func() {
		Expect(utils.SetManagerArgs(
			"--drift-check-interval="+driftInterval,
			"--orphan-rrset-cleanup",
			"--orphan-rrset-grace="+rrsetGrace,
			"--orphan-zone-cleanup",
			"--orphan-zone-grace="+zoneGrace,
		)).To(Succeed())
		restartMetricsPortForward(&metricsPF)
	})

	It("flags then deletes orphan RRsets after grace; leaves unmarked alone", func() {
		Expect(pdnsClient.Records.Change(ctx, zoneName, orphanRRname, powerdns.RRTypeA, 300, []string{"9.9.9.9"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To(""),
				Account: ptr.To(operatorAcct),
			}),
		)).To(Succeed())
		Expect(pdnsClient.Records.Change(ctx, zoneName, manualRRname, powerdns.RRTypeA, 300, []string{"7.7.7.7"},
			powerdns.WithComments(powerdns.Comment{
				Content: ptr.To("manual"),
				Account: ptr.To("admin"),
			}),
		)).To(Succeed())
		DeferCleanup(func() {
			_ = pdnsClient.Records.Delete(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
			_ = pdnsClient.Records.Delete(ctx, zoneName, manualRRname, powerdns.RRTypeA)
		})

		var beforeDel float64
		if body, err := utils.FetchMetrics(); err == nil {
			beforeDel, _ = utils.MetricValue(body, "powerdns_operator_orphan_deletions_total", map[string]string{"kind": "rrset"})
		}

		By("waiting for orphan-since flag")
		Eventually(func(g Gomega) {
			rr, err := findRRset(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
			g.Expect(err).NotTo(HaveOccurred())
			text := operatorCommentText(rr)
			g.Expect(text).To(HavePrefix("powerdns-operator:orphan-since:"))
			g.Expect(text).To(MatchRegexp(`^powerdns-operator:orphan-since:[0-9]+$`))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		By("waiting for orphan RRset deletion after grace")
		Eventually(func(g Gomega) {
			_, err := findRRset(ctx, zoneName, orphanRRname, powerdns.RRTypeA)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			after, ok := utils.MetricValue(body, "powerdns_operator_orphan_deletions_total", map[string]string{"kind": "rrset"})
			g.Expect(ok).To(BeTrue())
			g.Expect(after).To(BeNumerically(">", beforeDel))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		_, err := findRRset(ctx, zoneName, manualRRname, powerdns.RRTypeA)
		Expect(err).NotTo(HaveOccurred())
	})

	It("flags then deletes orphan zones after grace", func() {
		_, err := pdnsClient.Zones.Get(ctx, orphanZoneName)
		if err != nil {
			_, err = pdnsClient.Zones.Add(ctx, &powerdns.Zone{
				Name:        ptr.To(canonical(orphanZoneName)),
				Kind:        powerdns.ZoneKindPtr(powerdns.NativeZoneKind),
				Nameservers: []string{canonical("ns1." + orphanZoneName), canonical("ns2." + orphanZoneName)},
				Account:     ptr.To(operatorAcct),
			})
			Expect(err).NotTo(HaveOccurred())
		}
		DeferCleanup(func() {
			_ = pdnsClient.Zones.Delete(ctx, orphanZoneName)
		})

		var beforeDel float64
		if body, err := utils.FetchMetrics(); err == nil {
			beforeDel, _ = utils.MetricValue(body, "powerdns_operator_orphan_deletions_total", map[string]string{"kind": "zone"})
		}

		By("waiting for orphan-since zone metadata")
		Eventually(func(g Gomega) {
			md, err := pdnsClient.Metadata.Get(ctx, orphanZoneName, powerdns.MetadataKind("X-POWERDNS-OPERATOR-ORPHAN-SINCE"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(md).NotTo(BeNil())
			g.Expect(md.Metadata).NotTo(BeEmpty())
			g.Expect(md.Metadata[0]).To(MatchRegexp(`^[0-9]+$`))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		By("waiting for orphan zone deletion after grace")
		Eventually(func(g Gomega) {
			_, err := pdnsClient.Zones.Get(ctx, orphanZoneName)
			g.Expect(err).To(HaveOccurred())
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			body, err := utils.FetchMetrics()
			g.Expect(err).NotTo(HaveOccurred())
			after, ok := utils.MetricValue(body, "powerdns_operator_orphan_deletions_total", map[string]string{"kind": "zone"})
			g.Expect(ok).To(BeTrue())
			g.Expect(after).To(BeNumerically(">", beforeDel))
		}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(Succeed())
	})
})

const operatorAccount = "powerdns-operator"

func operatorCommentAccount(rr *powerdns.RRset) string {
	for _, c := range rr.Comments {
		if ptr.Deref(c.Account, "") == operatorAccount {
			return ptr.Deref(c.Account, "")
		}
	}
	return ""
}

func operatorCommentText(rr *powerdns.RRset) string {
	for _, c := range rr.Comments {
		if ptr.Deref(c.Account, "") == operatorAccount {
			return ptr.Deref(c.Content, "")
		}
	}
	return ""
}

func restartMetricsPortForward(pf **utils.PortForward) {
	GinkgoHelper()
	By("restarting metrics port-forward after manager rollout")
	if *pf != nil {
		(*pf).Stop()
	}
	newPF, err := utils.StartMetricsPortForward()
	Expect(err).NotTo(HaveOccurred())
	*pf = newPF
}
