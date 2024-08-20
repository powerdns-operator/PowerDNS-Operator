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
	"slices"
	"time"

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dnsv1alpha1 "gitlab.tech.orange/parent-factory/hzf-tools/powerdns-operator/api/v1alpha1"
)

var _ = Describe("Zone Controller", func() {

	const (
		resourceName         = "example1.org"
		resourceKind         = "Native"
		modifiedResourceKind = "Master"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)
	resourceNameservers := []string{"ns1.example1.org", "ns2.example1.org"}
	slices.Sort(resourceNameservers)
	modifiedResourceNameservers := []string{"ns1.example1.org", "ns2.example1.org", "ns3.example1.org"}
	slices.Sort(modifiedResourceNameservers)

	recreationResourceName := "example3.org"
	recreationResourceKind := "Native"
	recreationResourceNameservers := []string{"ns1.example3.org", "ns2.example3.org"}
	slices.Sort(recreationResourceNameservers)

	fakeResourceName := "fake.org"
	fakeResourceKind := "Native"
	fakeResourceNameservers := []string{"ns1.fake.org", "ns2.fake.org"}
	slices.Sort(fakeResourceNameservers)

	ctx := context.Background()
	typeNamespacedName := types.NamespacedName{
		Name: resourceName,
	}

	BeforeEach(func() {
		By("creating the zone resource")
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
	})

	AfterEach(func() {
		resource := &dnsv1alpha1.Zone{}
		err := k8sClient.Get(ctx, typeNamespacedName, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleanup the specific resource instance Zone")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			return errors.IsNotFound(err)
		}).Should(BeTrue())
	})

	Context("When existing resource", func() {
		It("should successfully retrieve the resource", Label("zone-initialization"), func() {
			By("Getting the existing resource")
			zone := &dnsv1alpha1.Zone{}
			// Waiting for the resource to be fully created
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(resourceNameservers), "Nameservers should be equal")
			Expect(zone.GetFinalizers()).To(ContainElement(FINALIZER_NAME), "Zone should contain the finalizer")
			Expect(fmt.Sprintf("%d", *(zone.Status.Serial))).To(Equal(fmt.Sprintf("%s01", time.Now().Format("20060102"))), "Serial should be YYYYMMDD01")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the nameservers of the zone", Label("zone-modification", "nameservers"), func() {
			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha1.Zone{}
			// Waiting for the resource to be fully created
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

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
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			//
			expectedSerial := *initialSerial + uint32(1)
			Expect(getMockedNameservers(resourceName)).To(Equal(modifiedResourceNameservers), "Nameservers should be equal")
			Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify the kind of the zone", Label("zone-modification", "kind"), func() {
			By("Getting the initial Serial of the resource")
			zone := &dnsv1alpha1.Zone{}
			// Waiting for the resource to be fully created
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Modifying the resource")
			resource := &dnsv1alpha1.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}

			// Update the resource for each kind and ensure the serial is incremented
			var modifiedResourceKind = []string{"Master", "Native", "Slave", "Producer", "Consumer"}
			for i, kind := range modifiedResourceKind {
				_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
					resource.Spec.Kind = kind
					return nil
				})
				Expect(err).NotTo(HaveOccurred())

				By("Getting the modified resource")
				modifiedZone := &dnsv1alpha1.Zone{}
				// Waiting for the resource to be fully modified
				time.Sleep(500 * time.Millisecond)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespacedName, modifiedZone)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				expectedSerial := *initialSerial + uint32(i+1)
				Expect(getMockedKind(resourceName)).To(Equal(kind), "Kind should be equal")
				Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
			}
		})

	})

	Context("When existing resource", func() {
		It("should successfully recreate an existing zone", Label("zone-recreation"), func() {
			By("Creating a Zone directly in the mock")
			// Serial initialization
			now := time.Now().UTC()
			initialSerial := uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1
			zones[makeCanonical(recreationResourceName)] = &powerdns.Zone{
				Name:   &recreationResourceName,
				Kind:   powerdns.ZoneKindPtr(powerdns.ZoneKind(recreationResourceKind)),
				Serial: &initialSerial,
			}

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
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			expectedSerial := initialSerial + uint32(1)
			Expect(getMockedKind(recreationResourceName)).To(Equal(recreationResourceKind), "Kind should be equal")
			Expect(getMockedNameservers(recreationResourceName)).To(Equal(recreationResourceNameservers), "Nameservers should be equal")
			Expect(*(updatedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When existing resource", func() {
		It("should successfully modify a deleted zone", Label("zone-modification-after-deletion"), func() {
			// Waiting for the resource to be fully created
			time.Sleep(500 * time.Millisecond)

			By("Deleting a Zone directly in the mock")
			delete(zones, makeCanonical(resourceName))
			delete(records, makeCanonical(resourceName))

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
			time.Sleep(500 * time.Millisecond)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedKind(resourceName)).To(Equal(resourceKind), "Kind should be equal")
			Expect(getMockedNameservers(resourceName)).To(Equal(modifiedResourceNameservers), "Nameservers should be equal")
		})
	})

	Context("When existing resource", func() {
		It("should successfully delete a deleted zone", Label("zone-deletion-after-deletion"), func() {
			By("Creating a Zone")
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
			// Waiting for the resource to be fully created
			time.Sleep(500 * time.Millisecond)
			delete(zones, makeCanonical(fakeResourceName))
			delete(records, makeCanonical(fakeResourceName))

			By("Deleting the Zone")
			Eventually(func() bool {
				err := k8sClient.Delete(ctx, fakeResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Getting the Zone")
			// Waiting for the resource to be fully deleted
			time.Sleep(500 * time.Millisecond)
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
