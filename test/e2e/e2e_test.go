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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
)

const namespace = "powerdns-operator-system"

var _ = Describe("controller", Ordered, func() {
	/*
		BeforeAll(func() {
			By("installing prometheus operator")
			Expect(utils.InstallPrometheusOperator()).To(Succeed())

			By("installing the cert-manager")
			Expect(utils.InstallCertManager()).To(Succeed())

			By("creating manager namespace")
			cmd := exec.Command("kubectl", "create", "ns", namespace)
			_, _ = utils.Run(cmd)
		})

		AfterAll(func() {
			By("uninstalling the Prometheus manager bundle")
			utils.UninstallPrometheusOperator()

			By("uninstalling the cert-manager bundle")
			utils.UninstallCertManager()

			By("removing manager namespace")
			cmd := exec.Command("kubectl", "delete", "ns", namespace)
			_, _ = utils.Run(cmd)
		})
	*/
	Context("On an existing PowerDNS cluster", func() {
		It("should create the zone and RRsets successfully", func() {
			ctx := context.Background()
			zoneName := "example2.org"
			zoneKind := "Native"
			zoneNS1 := "ns1.example2.org"
			zoneNS2 := "ns2.example2.org"

			By("Creating the Zone resource")
			zone := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: zoneName,
				},
			}
			zone.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, zone, func() error {
				zone.Spec = dnsv1alpha1.ZoneSpec{
					Kind:        zoneKind,
					Nameservers: []string{zoneNS1, zoneNS2},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			/*
				var controllerPodName string

				// projectimage stores the name of the image used in the example
				var projectimage = "example.com/powerdns-operator:v0.0.1"

				By("building the manager(Operator) image")
				cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
				_, err = utils.Run(cmd)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())

				By("loading the the manager(Operator) image on Kind")
				err = utils.LoadImageToKindClusterWithName(projectimage)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())

				By("installing CRDs")
				cmd = exec.Command("make", "install")
				_, err = utils.Run(cmd)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())

				By("deploying the controller-manager")
				cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
				_, err = utils.Run(cmd)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())

				By("validating that the controller-manager pod is running as expected")
				verifyControllerUp := func() error {
					// Get pod name

					cmd = exec.Command("kubectl", "get",
						"pods", "-l", "control-plane=controller-manager",
						"-o", "go-template={{ range .items }}"+
							"{{ if not .metadata.deletionTimestamp }}"+
							"{{ .metadata.name }}"+
							"{{ \"\\n\" }}{{ end }}{{ end }}",
						"-n", namespace,
					)

					podOutput, err := utils.Run(cmd)
					ExpectWithOffset(2, err).NotTo(HaveOccurred())
					podNames := utils.GetNonEmptyLines(string(podOutput))
					if len(podNames) != 1 {
						return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
					}
					controllerPodName = podNames[0]
					ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

					// Validate pod status
					cmd = exec.Command("kubectl", "get",
						"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
						"-n", namespace,
					)
					status, err := utils.Run(cmd)
					ExpectWithOffset(2, err).NotTo(HaveOccurred())
					if string(status) != "Running" {
						return fmt.Errorf("controller pod in %s status", status)
					}
					return nil
				}
				EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
			*/
		})
	})
})
