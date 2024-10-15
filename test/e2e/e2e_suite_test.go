/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var k8sClient client.Client

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting powerdns-operator suite\n")
	RunSpecs(t, "e2e suite")
}

// BeforeSuite run before any specs are run to perform the required actions for all e2e Go tests.
var _ = BeforeSuite(func() {
	var err error
	log := log.FromContext(context.Background())

	//	kbc, err := utils.NewTestContext(util.KubebuilderBinName, "GO111MODULE=on")
	//	Expect(err).NotTo(HaveOccurred())
	//
	//	By("installing the cert-manager bundle")
	//	Expect(kbc.InstallCertManager()).To(Succeed())
	//
	//	By("installing the Prometheus operator")
	//	Expect(kbc.InstallPrometheusOperManager()).To(Succeed())

	path := os.Getenv("TESTCONFIG")
	if path == "" {
		path = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		log.Error(err, "Failed to build config")
		return
	}
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = dnsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	kc, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	k8sClient = kc
	Expect(k8sClient).NotTo(BeNil())

	fmt.Fprintf(GinkgoWriter, time.Now().Format(time.StampMilli)+": Info: Setup Suite Execution\n")
})

// AfterSuite run after all the specs have run, regardless of whether any tests have failed to ensures that
// all be cleaned up
var _ = AfterSuite(func() {
	// kbc, err := utils.NewTestContext(util.KubebuilderBinName, "GO111MODULE=on")
	// Expect(err).NotTo(HaveOccurred())
	// Expect(kbc.Prepare()).To(Succeed())
	//
	// By("uninstalling the Prometheus manager bundle")
	// kbc.UninstallPrometheusOperManager()
	//
	// By("uninstalling the cert-manager bundle")
	// kbc.UninstallCertManager()
})
