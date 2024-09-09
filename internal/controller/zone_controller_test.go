/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
)

var _ = Describe("Zone Controller", func() {

	const (
		resourceName = "example1.org"
		resourceKind = "Native"

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)
	resourceNameservers := []string{"ns1.example1.org", "ns2.example1.org"}

	typeNamespacedName := types.NamespacedName{
		Name: resourceName,
	}

	BeforeEach(func() {
		ctx := context.Background()
		By("creating the Zone resource")
		resource := &dnsv1alpha1.Zone{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}
		resource.SetResourceVersion("")
		_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
			resource.Spec = dnsv1alpha1.ZoneSpec{
				Kind:        resourceKind,
				Nameservers: resourceNameservers,
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
		resource := &dnsv1alpha1.Zone{}
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
			ctx := context.Background()
			By("Getting the existing resource")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				_, found := readFromZonesMap(makeCanonical(resourceName))
				return err == nil && found
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(resourceNameservers), "Nameservers should be equal")
			Expect(zone.GetFinalizers()).To(ContainElement(FINALIZER_NAME), "Zone should contain the finalizer")
			Expect(fmt.Sprintf("%d", *(zone.Status.Serial))).To(Equal(fmt.Sprintf("%s01", time.Now().Format("20060102"))), "Serial should be YYYYMMDD01")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the nameservers of the zone", Label("zone-modification", "nameservers"), func() {
			ctx := context.Background()
			// Specific test variables
			modifiedResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org", "ns3.example1.org"}

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Nameservers = modifiedResourceNameservers
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the modified resource")
			modifiedZone := &dnsv1alpha1.Zone{}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
				return err == nil && *modifiedZone.Status.Serial > initialSerial
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
			var modifiedResourceKind = []string{"Master", "Native", "Slave", "Producer", "Consumer"}

			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
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
				modifiedZone := &dnsv1alpha1.Zone{}
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
		It("should successfully recreate an existing zone", Label("zone-recreation"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "example3.org"
			recreationResourceKind := "Native"
			recreationResourceNameservers := []string{"ns1.example3.org", "ns2.example3.org"}

			By("Creating a Zone directly in the mock")
			// Serial initialization
			now := time.Now().UTC()
			initialSerial := uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1
			writeToZonesMap(makeCanonical(recreationResourceName), &powerdns.Zone{
				Name:   &recreationResourceName,
				Kind:   powerdns.ZoneKindPtr(powerdns.ZoneKind(recreationResourceKind)),
				Serial: &initialSerial,
			})

			By("Recreating a Zone")
			resource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: recreationResourceName,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha1.ZoneSpec{
					Kind:        recreationResourceKind,
					Nameservers: recreationResourceNameservers,
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedZone := &dnsv1alpha1.Zone{}
			typeNamespacedName := types.NamespacedName{
				Name: recreationResourceName,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil && updatedZone.Status.Serial != nil
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
			resource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Kind = resourceKind
				resource.Spec.Nameservers = modifiedResourceNameservers
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedZone := &dnsv1alpha1.Zone{}
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
			fakeResourceKind := "Native"
			fakeResourceNameservers := []string{"ns1.fake.org", "ns2.fake.org"}
			fakeResource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeResourceName,
				},
			}
			fakeResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, fakeResource, func() error {
				fakeResource.Spec = dnsv1alpha1.ZoneSpec{
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
				Name: fakeResourceName,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, fakeTypeNamespacedName, fakeResource)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
