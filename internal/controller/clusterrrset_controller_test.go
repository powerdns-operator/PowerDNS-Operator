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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
)

var _ = Describe("ClusterRRset Controller", func() {
	const (
		// Zone
		zoneName = "example6.org"
		zoneKind = NATIVE_KIND_ZONE
		zoneNS1  = "ns1.example6.org"
		zoneNS2  = "ns2.example6.org"

		// RRset
		resourceName     = "test.example6.org"
		resourceZoneKind = "ClusterZone"
		resourceDNSName  = "test"
		resourceTTL      = uint32(300)
		resourceType     = "A"
		resourceComment  = "Just a comment"
		zoneRef          = zoneName

		testRecord1 = "127.0.0.1"
		testRecord2 = "127.0.0.2"

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)

	// Global
	resourceRecords := []string{testRecord1, testRecord2}

	// ClusterZone
	clusterZoneLookupKey := types.NamespacedName{
		Name: zoneName,
	}

	// ClusterRRset
	clusterRrsetLookupKey := types.NamespacedName{
		Name: resourceName,
	}

	BeforeEach(func() {
		ctx := context.Background()
		By("Creating the Zone resource")
		zone := &dnsv1alpha2.ClusterZone{
			ObjectMeta: metav1.ObjectMeta{
				Name: zoneName,
			},
		}
		zone.SetResourceVersion("")
		_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, zone, func() error {
			zone.Spec = dnsv1alpha2.ZoneSpec{
				Kind:        zoneKind,
				Nameservers: []string{zoneNS1, zoneNS2},
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, clusterZoneLookupKey, zone)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is created in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(zone.Name))
			return found
		}, timeout, interval).Should(BeTrue())

		By("Ensuring the resource does not already exists")
		emptyResource := &dnsv1alpha2.ClusterRRset{}
		err = k8sClient.Get(ctx, clusterRrsetLookupKey, emptyResource)
		Expect(err).To(HaveOccurred())

		By("Creating the ClusterRRset resource")
		resource := &dnsv1alpha2.ClusterRRset{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}
		resource.SetResourceVersion("")
		comment := resourceComment
		_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
			resource.Spec = dnsv1alpha2.RRsetSpec{
				ZoneRef: dnsv1alpha2.ZoneRef{
					Name: zoneRef,
					Kind: resourceZoneKind,
				},
				Type:    resourceType,
				Name:    resourceDNSName,
				TTL:     resourceTTL,
				Records: resourceRecords,
				Comment: &comment,
			}
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, clusterRrsetLookupKey, resource)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is created in the backend
		Eventually(func() bool {
			_, ok := readFromRecordsMap(makeCanonical(resource.Name))
			return ok
		}, timeout, interval).Should(BeTrue())
		// Wait for all reconciliations loop to be done
		time.Sleep(1 * time.Second)

	})

	AfterEach(func() {
		ctx := context.Background()
		resource := &dnsv1alpha2.ClusterRRset{}
		err := k8sClient.Get(ctx, clusterRrsetLookupKey, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleaning up the specific resource instance RRset")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, clusterRrsetLookupKey, resource)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())

		By("Cleaning up the specific resource instance Zone")
		zone := &dnsv1alpha2.ClusterZone{}
		err = k8sClient.Get(ctx, clusterZoneLookupKey, zone)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, zone)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, clusterZoneLookupKey, zone)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is deleted in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(zone.Name))
			return found
		}, timeout, interval).Should(BeFalse())
	})

	Context("When existing resource", func() {
		It("should successfully retrieve the resource", Label("clusterrrset-initialization"), func() {
			ic := countClusterRrsetsMetrics()
			ctx := context.Background()
			By("Getting the existing resource")
			createdResource := &dnsv1alpha2.ClusterRRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterRrsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countClusterRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getClusterRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating a RRset with an existing ClusterRRset with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("rrset-creation", "existing-clusterrrset"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "test.example6.org"
			recreationResourceNamespace := "example6"
			recreationResourceZoneKind := "ClusterZone"
			recreationResourceDNSName := "test"
			recreationResourceTTL := uint32(300)
			recreationResourceType := "A"
			recreationResourceComment := "Just a comment"
			recreationResourceRecords := []string{"127.0.0.1", "127.0.0.2"}
			recreationResourceZoneName := zoneName

			By("Creating a RRset")
			resource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recreationResourceName,
					Namespace: recreationResourceNamespace,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha2.RRsetSpec{
					Type:    recreationResourceType,
					Name:    recreationResourceDNSName,
					TTL:     recreationResourceTTL,
					Records: recreationResourceRecords,
					Comment: &recreationResourceComment,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: recreationResourceZoneName,
						Kind: recreationResourceZoneKind,
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			recreatedRrset := &dnsv1alpha2.RRset{}
			typeNamespacedName := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: recreationResourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, recreatedRrset)
				return err == nil && recreatedRrset.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
