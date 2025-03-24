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

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
)

var _ = Describe("Zone Controller", func() {

	const (
		resourceName      = "example1.org"
		resourceNamespace = "example1"
		resourceKind      = NATIVE_KIND_ZONE
		resourceCatalog   = "catalog.example1.org."

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)
	resourceNameservers := []string{"ns1.example1.org", "ns2.example1.org"}

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}

	BeforeEach(func() {
		ctx := context.Background()
		By("creating the Zone resource")
		resource := &dnsv1alpha2.Zone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
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
		resource := &dnsv1alpha2.Zone{}
		err := k8sClient.Get(ctx, typeNamespacedName, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleanup the specific resource instance Zone")
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
		It("should successfully retrieve the resource", Label("zone-initialization"), func() {
			ic := countZonesMetrics()
			ctx := context.Background()
			By("Getting the existing resource")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(countZonesMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getZoneMetricWithLabels(SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(resourceNameservers), "Nameservers should be equal")
			Expect(getMockedCatalog(resourceName)).To(Equal(resourceCatalog), "Catalog should be equal")
			Expect(zone.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "Zone should contain the finalizer")
			Expect(fmt.Sprintf("%d", *(zone.Status.Serial))).To(Equal(fmt.Sprintf("%s01", time.Now().UTC().Format("20060102"))), "Serial should be YYYYMMDD01")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the nameservers of the zone", Label("zone-modification", "nameservers"), func() {
			ctx := context.Background()
			// Specific test variables
			modifiedResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org", "ns3.example1.org"}

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Nameservers = modifiedResourceNameservers
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the modified resource")
			modifiedZone := &dnsv1alpha2.Zone{}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
				return err == nil && modifiedZone.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			expectedSerial := initialSerial + uint32(1)
			Expect(getMockedNameservers(resourceName)).To(Equal(modifiedResourceNameservers), "Nameservers should be equal")
			Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the kind of the zone", Label("zone-modification", "kind"), func() {
			ctx := context.Background()
			// Specific test variables
			var modifiedResourceKind = []string{MASTER_KIND_ZONE, NATIVE_KIND_ZONE, SLAVE_KIND_ZONE, PRODUCER_KIND_ZONE, CONSUMER_KIND_ZONE}

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			// Update the resource for each kind and ensure the serial is incremented
			for i, kind := range modifiedResourceKind {
				_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
					resource.Spec.Kind = kind
					return nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Getting the modified resource")
				modifiedZone := &dnsv1alpha2.Zone{}
				// Waiting for the resource to be fully modified
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
					return err == nil && *modifiedZone.Status.Serial > *initialSerial+uint32(i)
				}, timeout, interval).Should(BeTrue())

				expectedSerial := *initialSerial + uint32(i+1)
				Expect(getMockedKind(resourceName)).To(Equal(kind), "Kind should be equal")
				Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
			}
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the catalog of the zone", Label("zone-modification", "catalog"), func() {
			ctx := context.Background()
			// Specific test variables
			// Sending a 'nil' catalog to PowerDNS is considered as a no modification, so
			// to clear a catalog specification for a zone, you need to specify an empty catalog
			// So we test all use-cases:
			// a. from an empty catalog to a specific catalog
			// b. from a specific catalog to an empty catalog
			var modifiedResourceCatalog = []string{"", "catalog.other-domain.org.", ""}

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			// Update the resource for each catalog and ensure the serial is incremented
			for i, c := range modifiedResourceCatalog {
				catalog := c
				_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
					resource.Spec.Catalog = &catalog
					return nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Getting the modified resource")
				modifiedZone := &dnsv1alpha2.Zone{}
				// Waiting for the resource to be fully modified
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
					return err == nil && *modifiedZone.Status.Serial > *initialSerial+uint32(i)
				}, timeout, interval).Should(BeTrue())

				expectedSerial := *initialSerial + uint32(i+1)
				Expect(getMockedCatalog(resourceName)).To(Equal(catalog), "Catalog should be equal")
				Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
			}
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the catalog of the zone", Label("zone-modification", "soa-edit-api"), func() {
			ctx := context.Background()
			// Specific test variables
			var modifiedResourceSOAEditAPI = "EPOCH"

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())

			By("Modifying the resource")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			epochSerial := uint32(time.Now().UTC().Unix())
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.SOAEditAPI = &modifiedResourceSOAEditAPI
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the modified resource")
			modifiedZone := &dnsv1alpha2.Zone{}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
				return err == nil && modifiedZone.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedSOAEditAPI(resourceName)).To(Equal(modifiedResourceSOAEditAPI), "SOA-Edit-API should have changed")
			Expect(*(modifiedZone.Status.Serial)).To(Equal(epochSerial), "Serial should have changed")
		})
	})

	Context("When existing resource", func() {
		It("should successfully recreate an existing zone", Label("zone-recreation"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "example3.org"
			recreationResourceKind := NATIVE_KIND_ZONE
			recreationResourceNameservers := []string{"ns1.example3.org", "ns2.example3.org"}

			By("Creating a Zone directly in the mock")
			// Serial initialization
			now := time.Now().UTC()
			initialSerial := uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1
			writeToZonesMap(makeCanonical(recreationResourceName), &powerdns.Zone{
				Name:       &recreationResourceName,
				Kind:       powerdns.ZoneKindPtr(powerdns.ZoneKind(recreationResourceKind)),
				Serial:     &initialSerial,
				SOAEditAPI: ptr.To("DEFAULT"),
			})

			By("Recreating a Zone")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recreationResourceName,
					Namespace: resourceNamespace,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha2.ZoneSpec{
					Kind:        recreationResourceKind,
					Nameservers: recreationResourceNameservers,
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedZone := &dnsv1alpha2.Zone{}
			typeNamespacedName := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: resourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil && updatedZone.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				return getMockedKind(recreationResourceName) != ""
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedKind(recreationResourceName)).To(Equal(recreationResourceKind), "Kind should be equal")

			Eventually(func() bool {
				return len(getMockedNameservers(recreationResourceName)) > 0
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedNameservers(recreationResourceName)).To(Equal(recreationResourceNameservers), "Nameservers should be equal")

			expectedSerial := initialSerial + uint32(1)
			Expect(*(updatedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify a deleted zone", Label("zone-modification-after-deletion"), func() {
			ctx := context.Background()

			// Specific test variables
			modifiedResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org", "ns3.example1.org"}

			By("Deleting a Zone & RRset directly in the mock")
			// Wait all the reconciliation loop to be done before deleting the mock (backend) Zone && RRSet resources
			// Otherwise, the resource will be recreated in the mock backend by a 2nd reconciliation loop
			// Ending up with a Conflict error returned from the PowerDNS client Add() func
			time.Sleep(2 * time.Second)
			deleteFromZonesMap(makeCanonical(resourceName))
			deleteFromRecordsMap(makeCanonical(resourceName))

			By("Verifying the Zone & Records has been deleted in the mock")
			Eventually(func() bool {
				_, zoneFound := readFromZonesMap(makeCanonical(resourceName))
				_, rrsetFound := readFromRecordsMap(makeCanonical(resourceName))
				return !zoneFound && !rrsetFound
			}, timeout, interval).Should(BeTrue())

			By("Modifying the deleted Zone")
			resource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Kind = resourceKind
				resource.Spec.Nameservers = modifiedResourceNameservers
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedZone := &dnsv1alpha2.Zone{}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				_, zoneFound := readFromZonesMap(makeCanonical(resourceName))
				_, rrsetFound := readFromRecordsMap(makeCanonical(resourceName))
				return err == nil && zoneFound && rrsetFound
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(modifiedResourceNameservers), "Nameservers should be equal")
		})
	})

	Context("When existing resource", func() {
		It("should successfully delete a deleted zone", Label("zone-deletion-after-deletion"), func() {
			ctx := context.Background()
			By("Creating a Zone")
			fakeResourceName := "fake.org"
			fakeResourceKind := NATIVE_KIND_ZONE
			fakeResourceNameservers := []string{"ns1.fake.org", "ns2.fake.org"}
			fakeResource := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fakeResourceName,
					Namespace: resourceNamespace,
				},
			}
			fakeResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, fakeResource, func() error {
				fakeResource.Spec = dnsv1alpha2.ZoneSpec{
					Kind:        fakeResourceKind,
					Nameservers: fakeResourceNameservers,
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting a Zone directly in the mock")
			// Wait all the reconciliation loop to be done before deleting the mock (backend) Zone && RRSet resources
			// Otherwise, the resource will be recreated in the mock backend by a 2nd reconciliation loop
			time.Sleep(2 * time.Second)
			deleteFromZonesMap(makeCanonical(fakeResourceName))
			deleteFromRecordsMap(makeCanonical(fakeResourceName))

			By("Deleting the Zone")
			Eventually(func() bool {
				err := k8sClient.Delete(ctx, fakeResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Getting the Zone")
			// Waiting for the resource to be fully deleted
			fakeTypeNamespacedName := types.NamespacedName{
				Name:      fakeResourceName,
				Namespace: resourceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, fakeTypeNamespacedName, fakeResource)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("When creating a Zone with an existing Zone with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("zone-creation", "existing-zone"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "example1.org"
			recreationResourceNamespace := "example3"
			recreationResourceKind := NATIVE_KIND_ZONE
			recreationResourceCatalog := "catalog.example1.org."
			recreationResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org"}

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
	Context("When creating a ClusterZone with an existing Zone with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("clusterzone-creation", "existing-zone"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "example1.org"
			recreationResourceKind := NATIVE_KIND_ZONE
			recreationResourceCatalog := "catalog.example1.org."
			recreationResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org"}

			By("Creating a Zone")
			resource := &dnsv1alpha2.ClusterZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: recreationResourceName,
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
			updatedZone := &dnsv1alpha2.ClusterZone{}
			typeNamespacedName := types.NamespacedName{
				Name: recreationResourceName,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil && updatedZone.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
