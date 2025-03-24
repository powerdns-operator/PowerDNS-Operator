/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

//nolint:goconst
package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
)

var _ = Describe("ClusterZone Controller", func() {
	const (
		resourceName    = "example4.org"
		resourceKind    = NATIVE_KIND_ZONE
		resourceCatalog = "catalog.example4.org."

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)
	resourceNameservers := []string{"ns1.example4.org", "ns2.example4.org"}

	typeNamespacedName := types.NamespacedName{
		Name: resourceName,
	}

	BeforeEach(func() {
		ctx := context.Background()

		By("creating the ClusterZone resource")
		resource := &dnsv1alpha2.ClusterZone{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}
		resource.SetResourceVersion("")
		_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
			resource.Spec = dnsv1alpha2.ZoneSpec{
				Kind:        resourceKind,
				Nameservers: resourceNameservers,
				Catalog:     ptr.To(resourceCatalog),
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is created in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(resourceName))
			return found
		}, timeout, interval).Should(BeTrue())

		// Wait for all reconciliations loop to be done
		time.Sleep(1 * time.Second)
	})

	AfterEach(func() {
		ctx := context.Background()
		resource := &dnsv1alpha2.ClusterZone{}
		err := k8sClient.Get(ctx, typeNamespacedName, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleanup the specific resource instance ClusterZone")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		By("Verifying the resource has been deleted")
		// Waiting for the resource to be fully deleted
		Eventually(func() bool {
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is deleted in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(resourceName))
			return found
		}, timeout, interval).Should(BeFalse())
	})

	Context("When existing resource", func() {
		It("should successfully retrieve the resource", Label("clusterzone-initialization"), func() {
			ic := countClusterZonesMetrics()
			ctx := context.Background()
			By("Getting the existing resource")
			clusterzone := &dnsv1alpha2.ClusterZone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, clusterzone)
				return err == nil && clusterzone.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(countClusterZonesMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getClusterZoneMetricWithLabels(SUCCEEDED_STATUS, resourceName)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(resourceNameservers), "Nameservers should be equal")
			Expect(getMockedCatalog(resourceName)).To(Equal(resourceCatalog), "Catalog should be equal")
			Expect(clusterzone.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "Zone should contain the finalizer")
			Expect(fmt.Sprintf("%d", *(clusterzone.Status.Serial))).To(Equal(fmt.Sprintf("%s01", time.Now().UTC().Format("20060102"))), "Serial should be YYYYMMDD01")
		})
	})
	Context("When creating a Zone with an existing ClusterZone with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("zone-creation", "existing-clusterzone"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "example4.org"
			recreationResourceNamespace := "example4"
			recreationResourceKind := NATIVE_KIND_ZONE
			recreationResourceCatalog := "catalog.example4.org."
			recreationResourceNameservers := []string{"ns1.example4.org", "ns2.example4.org"}

			By("Creating a Zone")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recreationResourceName,
					Namespace: recreationResourceNamespace,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha2.ZoneSpec{
					Kind:        recreationResourceKind,
					Nameservers: recreationResourceNameservers,
					Catalog:     ptr.To(recreationResourceCatalog),
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedZone := &dnsv1alpha2.Zone{}
			typeNamespacedName := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: recreationResourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil && updatedZone.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
