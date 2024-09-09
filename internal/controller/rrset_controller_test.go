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
	"time"

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dnsv1alpha1 "gitlab.tech.orange/parent-factory/hzf-tools/powerdns-operator/api/v1alpha1"
)

var _ = Describe("RRset Controller", func() {

	const (
		// Zone
		zoneName = "example2.org"
		zoneKind = "Native"
		zoneNS1  = "ns1.example2.org"
		zoneNS2  = "ns2.example2.org"

		// RRset
		resourceName      = "test.example2.org"
		resourceNamespace = "default"
		resourceTTL       = uint32(300)
		resourceType      = "A"
		resourceComment   = "Just a comment"
		zoneIdRef         = zoneName

		testRecord1 = "127.0.0.1"
		testRecord2 = "127.0.0.2"

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)

	// Global
	resourceRecords := []string{testRecord1, testRecord2}

	// Zone
	zoneLookupKey := types.NamespacedName{
		Name: zoneName,
	}

	// RRset
	rssetLookupKey := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}

	BeforeEach(func() {
		ctx := context.Background()
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
		Eventually(func() bool {
			err := k8sClient.Get(ctx, zoneLookupKey, zone)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is created in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(zone.Name))
			return found
		}, timeout, interval).Should(BeTrue())

		By("Ensuring the resource does not already exists")
		emptyResource := &dnsv1alpha1.RRset{}
		err = k8sClient.Get(ctx, rssetLookupKey, emptyResource)
		Expect(err).To(HaveOccurred())

		By("Creating the RRset resource")
		resource := &dnsv1alpha1.RRset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}
		resource.SetResourceVersion("")
		comment := resourceComment
		_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
			resource.Spec = dnsv1alpha1.RRsetSpec{
				ZoneRef: dnsv1alpha1.ZoneRef{
					Name: zoneIdRef,
				},
				Type:    resourceType,
				TTL:     resourceTTL,
				Records: resourceRecords,
				Comment: &comment,
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, rssetLookupKey, resource)
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
		resource := &dnsv1alpha1.RRset{}
		err := k8sClient.Get(ctx, rssetLookupKey, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleaning up the specific resource instance RRset")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, rssetLookupKey, resource)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())

		By("Cleaning up the specific resource instance Zone")
		zone := &dnsv1alpha1.Zone{}
		err = k8sClient.Get(ctx, zoneLookupKey, zone)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, zone)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, zoneLookupKey, zone)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is deleted in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(zone.Name))
			return found
		}, timeout, interval).Should(BeFalse())
	})

	Context("When existing resource", func() {
		It("should successfully retrieve the resource", Label("rrset-initialization"), func() {
			ctx := context.Background()
			By("Getting the existing resource")
			createdResource := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, createdResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneIdRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When updating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-modification", "records"), func() {
			ctx := context.Background()
			// Specific test variables
			updatedRecords := []string{"127.0.0.3"}

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Updating RRset records")
			resource := &dnsv1alpha1.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Records = updatedRecords
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the updated resource")
			updatedRRset := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(updatedRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, modifiedZone)
				return err == nil && *modifiedZone.Status.Serial != *initialSerial
			}, timeout, interval).Should(BeTrue())
			//
			expectedSerial := *initialSerial + uint32(1)
			Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When updating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-modification", "ttl"), func() {
			ctx := context.Background()
			// Specific test variables
			modifiedResourceTTL := uint32(150)

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				_, found := readFromZonesMap(makeCanonical(zone.Name))
				return err == nil && found
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Updating RRset TTL")
			resource := &dnsv1alpha1.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.TTL = modifiedResourceTTL
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the updated resource")
			updatedRRset := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				rrset, found := readFromRecordsMap(makeCanonical(resourceName))
				return err == nil && found && *rrset.TTL != resourceTTL // ensure the RRset has been updated in the backend
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(modifiedResourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, modifiedZone)
				return err == nil && *modifiedZone.Status.Serial > initialSerial
			}, timeout, interval).Should(BeTrue())
			expectedSerial := initialSerial + uint32(1)
			Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When updating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-modification", "comments"), func() {
			ctx := context.Background()
			// Specific test variables
			modifiedResourceComment := "Just another comment"

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Updating RRset TTL")
			resource := &dnsv1alpha1.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.Comment = &modifiedResourceComment
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the updated resource")
			updatedRRset := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil && *updatedRRset.Spec.Comment != resourceComment
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(modifiedResourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, modifiedZone)
				return err == nil && *modifiedZone.Status.Serial > initialSerial
			}, timeout, interval).Should(BeTrue())
			expectedSerial := initialSerial + uint32(1)
			Expect(*modifiedZone.Status.Serial).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})

	Context("When existing resource", func() {
		It("should successfully recreate an existing rrset", Label("rrset-recreation"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "test2.example2.org"
			recreationResourceNamespace := "default"
			recreationResourceTTL := uint32(253)
			recreationResourceType := "A"
			recreationResourceComment := "it is an useless comment"
			recreationZoneRef := zoneName
			recreationRecord := "127.0.0.3"

			By("Creating a RRset directly in the mock")
			writeToRecordsMap(makeCanonical(recreationResourceName), &powerdns.RRset{
				Type: powerdns.RRTypePtr(powerdns.RRType(recreationResourceType)),
				Name: &recreationResourceName,
				TTL:  &recreationResourceTTL,
				Records: []powerdns.Record{
					{Content: &recreationRecord},
				},
				Comments: []powerdns.Comment{
					{Content: &recreationResourceComment},
				},
			})

			By("Recreating a RRset")
			resource := &dnsv1alpha1.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recreationResourceName,
					Namespace: recreationResourceNamespace,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha1.RRsetSpec{
					Type:    recreationResourceType,
					TTL:     recreationResourceTTL,
					Records: []string{recreationRecord},
					Comment: &recreationResourceComment,
					ZoneRef: dnsv1alpha1.ZoneRef{
						Name: recreationZoneRef,
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedRRset := &dnsv1alpha1.RRset{}
			typeNamespacedName := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: recreationResourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedRRset)
				return err == nil && updatedRRset.Status.LastUpdateTime != nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				return getMockedRecordsForType(recreationResourceName, recreationResourceType) != nil
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedRecordsForType(recreationResourceName, recreationResourceType)).To(Equal([]string{recreationRecord}))

			Eventually(func() bool {
				return getMockedTTL(recreationResourceName, recreationResourceType) > uint32(0)
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedTTL(recreationResourceName, recreationResourceType)).To(Equal(recreationResourceTTL))

			Eventually(func() bool {
				return getMockedComment(recreationResourceName, recreationResourceType) != ""
			}, timeout, interval).Should(BeTrue())
			Expect(getMockedComment(recreationResourceName, recreationResourceType)).To(Equal(recreationResourceComment))
		})
	})
})
