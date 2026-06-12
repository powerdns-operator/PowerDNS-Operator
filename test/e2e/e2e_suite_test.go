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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joeig/go-powerdns/v3"

	"github.com/powerdns-operator/powerdns-operator/test/utils"
)

// pdnsClient talks to the in-cluster PowerDNS API through a port-forward and is
// used by the specs to assert what the operator actually wrote to PowerDNS.
var pdnsClient *powerdns.Client

// portForward is the `kubectl port-forward` tunnel to the PowerDNS API.
var portForward *utils.PortForward

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting powerdns-operator suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("deploying the PowerDNS test backend")
	Expect(utils.DeployPowerDNS()).To(Succeed())

	By("creating the operator namespace")
	Expect(utils.CreateNamespace(utils.OperatorNamespace)).To(Succeed())

	By("creating the PowerDNS credentials secret")
	Expect(utils.CreateOperatorSecret()).To(Succeed())

	By("building and loading the operator image into Kind")
	Expect(utils.BuildAndLoadOperatorImage(utils.OperatorImage)).To(Succeed())

	By("installing the CRDs")
	Expect(utils.InstallCRDs()).To(Succeed())

	By("deploying the operator")
	Expect(utils.DeployOperator(utils.OperatorImage)).To(Succeed())

	By("waiting for the controller-manager to be available")
	Expect(utils.WaitForControllerManager()).To(Succeed())

	By("starting a port-forward to the PowerDNS API")
	pf, err := utils.StartPowerDNSPortForward()
	Expect(err).NotTo(HaveOccurred())
	portForward = pf
	pdnsClient = utils.NewPDNSClient()
})

var _ = AfterSuite(func() {
	By("stopping the PowerDNS API port-forward")
	portForward.Stop()

	By("undeploying the operator")
	utils.UndeployOperator()

	By("uninstalling the CRDs")
	utils.UninstallCRDs()

	By("deleting the operator namespace")
	utils.DeleteNamespace(utils.OperatorNamespace)

	By("undeploying the PowerDNS test backend")
	utils.UndeployPowerDNS()
})
