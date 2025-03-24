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

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
)

var _ = Describe("RRset Controller", func() {

	const (
		// Zone
		zoneName      = "example2.org"
		zoneNamespace = "example2"
		zoneKind      = NATIVE_KIND_ZONE
		zoneNS1       = "ns1.example2.org"
		zoneNS2       = "ns2.example2.org"

		// RRset
		resourceName      = "test.example2.org"
		resourceNamespace = zoneNamespace
		resourceZoneKind  = "Zone"
		resourceDNSName   = "test"
		resourceTTL       = uint32(300)
		resourceType      = "A"
		resourceComment   = "Just a comment"
		zoneRef           = zoneName

		testRecord1 = "127.0.0.1"
		testRecord2 = "127.0.0.2"

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)

	// Global
	resourceRecords := []string{testRecord1, testRecord2}

	// Zone
	zoneLookupKey := types.NamespacedName{
		Name:      zoneName,
		Namespace: zoneNamespace,
	}

	// RRset
	rssetLookupKey := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}

	BeforeEach(func() {
		ctx := context.Background()
		By("Creating the Zone resource")
		zone := &dnsv1alpha2.Zone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      zoneName,
				Namespace: zoneNamespace,
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
			err := k8sClient.Get(ctx, zoneLookupKey, zone)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		// Confirm that resource is created in the backend
		Eventually(func() bool {
			_, found := readFromZonesMap(makeCanonical(zone.Name))
			return found
		}, timeout, interval).Should(BeTrue())

		By("Ensuring the resource does not already exists")
		emptyResource := &dnsv1alpha2.RRset{}
		err = k8sClient.Get(ctx, rssetLookupKey, emptyResource)
		Expect(err).To(HaveOccurred())

		By("Creating the RRset resource")
		resource := &dnsv1alpha2.RRset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
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
		resource := &dnsv1alpha2.RRset{}
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
		zone := &dnsv1alpha2.Zone{}
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
			ic := countRrsetsMetrics()
			ctx := context.Background()
			By("Getting the existing resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When updating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-modification", "records"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			updatedRecords := []string{"127.0.0.3"}

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Updating RRset records")
			resource := &dnsv1alpha2.RRset{
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
			updatedRRset := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil && updatedRRset.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(updatedRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha2.Zone{}
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
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			modifiedResourceTTL := uint32(150)

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				_, found := readFromZonesMap(makeCanonical(zone.Name))
				return err == nil && found
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Updating RRset TTL")
			resource := &dnsv1alpha2.RRset{
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
			updatedRRset := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil && updatedRRset.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(modifiedResourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(resourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha2.Zone{}
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
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			modifiedResourceComment := "Just another comment"

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha2.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				return err == nil && zone.Status.Serial != nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := *zone.Status.Serial

			By("Updating RRset Comment")
			resource := &dnsv1alpha2.RRset{
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
			updatedRRset := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil && updatedRRset.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())
			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(resourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(modifiedResourceComment))

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha2.Zone{}
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
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			recreationResourceName := "test2.example2.org"
			recreationResourceNamespace := zoneNamespace
			recreationResourceDNSName := "test2"
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
					TTL:     recreationResourceTTL,
					Name:    recreationResourceDNSName,
					Records: []string{recreationRecord},
					Comment: &recreationResourceComment,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: recreationZoneRef,
						Kind: resourceZoneKind,
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedRRset := &dnsv1alpha2.RRset{}
			typeNamespacedName := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: recreationResourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedRRset)
				return err == nil && updatedRRset.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
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

	Context("When existing resource", func() {
		It("should successfully modify a deleted rrset", Label("rrset-modification-after-deletion"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()

			// Specific test variables
			modifiedResourceTTL := uint32(161)
			modifiedResourceComment := "it is an useless comment"
			modifiedResourceRecords := []string{"127.0.0.4"}

			By("Deleting a RRset directly in the mock")
			// Wait all the reconciliation loop to be done before deleting the mock (backend) Zone && RRSet resources
			// Otherwise, the resource will be recreated in the mock backend by a 2nd reconciliation loop
			// Ending up with a Conflict error returned from the PowerDNS client Add() func
			time.Sleep(2 * time.Second)
			deleteFromRecordsMap(makeCanonical(resourceName))

			By("Verifying the Records has been deleted in the mock")
			Eventually(func() bool {
				_, rrsetFound := readFromRecordsMap(makeCanonical(resourceName))
				return !rrsetFound
			}, timeout, interval).Should(BeTrue())

			By("Modifying the deleted RRset")
			resource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec.TTL = modifiedResourceTTL
				resource.Spec.Name = resourceDNSName
				resource.Spec.Comment = &modifiedResourceComment
				resource.Spec.Records = modifiedResourceRecords
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			updatedRRset := &dnsv1alpha2.RRset{}
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: resourceNamespace,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedRRset)
				return err == nil && updatedRRset.IsInExpectedStatus(MODIFIED_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
			Expect(getRrsetMetricWithLabels(resourceDNSName+"."+zoneRef+".", resourceType, SUCCEEDED_STATUS, resourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(resourceName, resourceType)).To(Equal(modifiedResourceRecords))
			Expect(getMockedTTL(resourceName, resourceType)).To(Equal(modifiedResourceTTL))
			Expect(getMockedComment(resourceName, resourceType)).To(Equal(modifiedResourceComment))
		})
	})

	Context("When existing resource", func() {
		It("should successfully delete a deleted rrset", Label("rrset-deletion-after-deletion"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()

			By("Creating a RRset")
			fakeResourceName := "fake.example2.org"
			fakeResourceNamespace := "default"
			fakeResourceDNSName := "fake"
			fakeResourceTTL := uint32(123)
			fakeResourceType := "A"
			fakeResourceComment := "it is a fake comment"
			fakeZoneRef := zoneName
			fakeRecords := []string{"127.0.0.11", "127.0.0.12"}

			fakeResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fakeResourceName,
					Namespace: fakeResourceNamespace,
				},
			}
			fakeResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, fakeResource, func() error {
				fakeResource.Spec = dnsv1alpha2.RRsetSpec{
					Type:    fakeResourceType,
					Name:    fakeResourceDNSName,
					TTL:     fakeResourceTTL,
					Records: fakeRecords,
					Comment: &fakeResourceComment,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: fakeZoneRef,
						Kind: resourceZoneKind,
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting a RRset directly in the mock")
			// Wait all the reconciliation loop to be done before deleting the mock (backend) Zone && RRSet resources
			// Otherwise, the resource will be recreated in the mock backend by a 2nd reconciliation loop
			time.Sleep(2 * time.Second)
			deleteFromRecordsMap(makeCanonical(fakeResourceName))

			By("Deleting the Zone")
			Eventually(func() bool {
				err := k8sClient.Delete(ctx, fakeResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Getting the RRset")
			// Waiting for the resource to be fully deleted
			fakeTypeNamespacedName := types.NamespacedName{
				Name:      fakeResourceName,
				Namespace: fakeResourceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, fakeTypeNamespacedName, fakeResource)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(0), "No more metric should have been created")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "AAAA-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "aaaa"
			additionalResourceType := "AAAA"
			additionalResourceRecords := []string{"2001:0dc8:86a4:0000:0000:7a2f:2360:2341"}
			additionalResourceComment := "This is a AAAA Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")

		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "CNAME-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "cname"
			additionalResourceType := "CNAME"
			additionalResourceRecords := []string{resourceName}
			additionalResourceComment := "This is a CNAME Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "Wildcard-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "wildcard"
			additionalResourceType := "*.a"
			additionalResourceRecords := []string{"192.168.1.123"}
			additionalResourceComment := "This is a A Wildcard Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})
	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "MX-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "mx"
			additionalResourceType := makeCanonical(zoneRef)
			additionalResourceRecords := []string{"10 mail1.example2.org.", "20 mail2.example2.org."}
			additionalResourceComment := "This is a MX Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "NS-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "ns"
			additionalResourceType := makeCanonical(zoneRef)
			additionalResourceRecords := []string{"ns1.example2.org", "ns2.example2.org"}
			additionalResourceComment := "This is a NS Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "TXT-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "txt"
			additionalResourceType := makeCanonical(zoneRef)
			additionalResourceRecords := []string{"This a TXT Record"}
			additionalResourceComment := "This is a TXT Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "SRV-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			additionalResourceName := "srv"
			additionalResourceType := "_srv._protcol.myapp"
			additionalResourceRecords := []string{"1 50 25565 front.example2.org."}
			additionalResourceComment := "This is a SRV Record"

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: resourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneRef,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+zoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-creation", "PTR-Type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			reverseZoneName := "123.168.192.in-addr.arpa"
			reverseZoneNamespace := zoneNamespace
			additionalResourceName := "ptr"
			additionalResourceNamespace := zoneNamespace
			additionalResourceType := "1"
			additionalResourceRecords := []string{"mail1.example2.org"}
			additionalResourceComment := "This is a PTR Record"

			By("Creating the Reverse Zone resource")
			reverseZone := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      reverseZoneName,
					Namespace: reverseZoneNamespace,
				},
			}
			reverseZone.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, reverseZone, func() error {
				reverseZone.Spec = dnsv1alpha2.ZoneSpec{
					Kind:        zoneKind,
					Nameservers: []string{zoneNS1, zoneNS2},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			additionalReverseZoneLookupKey := types.NamespacedName{
				Name:      reverseZoneName,
				Namespace: reverseZoneNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalReverseZoneLookupKey, reverseZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			Eventually(func() bool {
				_, found := readFromZonesMap(makeCanonical(reverseZone.Name))
				return found
			}, timeout, interval).Should(BeTrue())

			By("Creating the RRset resource")
			additionalResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      additionalResourceName,
					Namespace: additionalResourceNamespace,
				},
			}
			additionalResource.SetResourceVersion("")
			_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, additionalResource, func() error {
				additionalResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: reverseZoneName,
						Kind: resourceZoneKind,
					},
					Type:    additionalResourceType,
					Name:    additionalResourceName,
					TTL:     resourceTTL,
					Records: additionalResourceRecords,
					Comment: &additionalResourceComment,
				}
				return nil
			})
			additionalRRsetLookupKey := types.NamespacedName{
				Name:      additionalResourceName,
				Namespace: additionalResourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, additionalResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(additionalResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, additionalRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, SUCCEEDED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(additionalResourceName+"."+reverseZoneName+".", additionalResourceType, SUCCEEDED_STATUS, additionalResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceRecords))
			Expect(getMockedTTL(DnsFqdn, additionalResourceType)).To(Equal(resourceTTL))
			Expect(getMockedComment(DnsFqdn, additionalResourceType)).To(Equal(additionalResourceComment))
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(reverseZoneName), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating a wrong RRset", func() {
		It("should reconcile the resource with Failed status", Label("wrong-rrset", "wrong-type"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			badTypeResourceName := "wrong-type"
			badTypeResourceDNSName := "wrong-type"
			badTypeResourceType := "AA"
			badTypeResourceRecords := []string{"1.1.1.1"}
			badTypeResourceComment := "This is a wrong-type Record"

			By("Creating the RRset resource")
			badTypeResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      badTypeResourceName,
					Namespace: resourceNamespace,
				},
			}
			badTypeResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, badTypeResource, func() error {
				badTypeResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: resourceZoneKind,
					},
					Type:    badTypeResourceType,
					Name:    badTypeResourceDNSName,
					TTL:     resourceTTL,
					Records: badTypeResourceRecords,
					Comment: &badTypeResourceComment,
				}
				return nil
			})
			badTypeRRsetLookupKey := types.NamespacedName{
				Name:      badTypeResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, badTypeRRsetLookupKey, badTypeResource)
				return err == nil && badTypeResource.Status.SyncStatus != nil
			}, timeout, interval).Should(BeTrue())

			DnsFqdn := getRRsetName(badTypeResource)

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, badTypeRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(badTypeResourceDNSName+"."+zoneRef+".", badTypeResourceType, FAILED_STATUS, badTypeResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, badTypeResourceType)).To(Equal([]string{}), "RRset should not have been created in backend")
			Expect(*createdResource.Status.SyncStatus).To(Equal(FAILED_STATUS), "RRset status should be 'Failed'")
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating a wrong RRset", func() {
		It("should reconcile the resource with Failed status", Label("wrong-rrset", "wrong-format"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			badFormatResourceName := "wrong-format"
			badFormatResourceDNSName := "_wrong._record.format"
			badFormatResourceType := "SRV"
			badFormatResourceRecords := []string{"1 50 25565 test2.helloworld.com"}
			badFormatResourceComment := "This is a wrong-format Record"

			By("Creating the RRset resource")
			badFormatResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      badFormatResourceName,
					Namespace: resourceNamespace,
				},
			}
			badFormatResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, badFormatResource, func() error {
				badFormatResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: resourceZoneKind,
					},
					Type:    badFormatResourceType,
					Name:    badFormatResourceDNSName,
					TTL:     resourceTTL,
					Records: badFormatResourceRecords,
					Comment: &badFormatResourceComment,
				}
				return nil
			})
			badFormatRRsetLookupKey := types.NamespacedName{
				Name:      badFormatResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, badFormatRRsetLookupKey, badFormatResource)
				return err == nil && badFormatResource.Status.SyncStatus != nil
			}, timeout, interval).Should(BeTrue())

			DnsFqdn := getRRsetName(badFormatResource)

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, badFormatRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(badFormatResourceDNSName+"."+zoneRef+".", badFormatResourceType, FAILED_STATUS, badFormatResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, badFormatResourceType)).To(Equal([]string{}), "RRset should not have been created in backend")
			Expect(*createdResource.Status.SyncStatus).To(Equal(FAILED_STATUS), "RRset status should be 'Failed'")
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating a wrong RRset", func() {
		It("should reconcile the resource with Failed status", Label("wrong-rrset", "unquoted-txt"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			unquotedResourceName := "unquoted"
			unquotedResourceDNSName := "unquoted-txt"
			unquotedResourceType := "TXT"
			unquotedResourceRecords := []string{"An unquoted record"}
			unquotedResourceComment := "This is an unquoted-TXT Record"

			By("Creating the RRset resource")
			unquotedResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      unquotedResourceName,
					Namespace: resourceNamespace,
				},
			}
			unquotedResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, unquotedResource, func() error {
				unquotedResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: resourceZoneKind,
					},
					Type:    unquotedResourceType,
					Name:    unquotedResourceDNSName,
					TTL:     resourceTTL,
					Records: unquotedResourceRecords,
					Comment: &unquotedResourceComment,
				}
				return nil
			})
			unquotedRRsetLookupKey := types.NamespacedName{
				Name:      unquotedResourceName,
				Namespace: resourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, unquotedRRsetLookupKey, unquotedResource)
				return err == nil && unquotedResource.Status.SyncStatus != nil
			}, timeout, interval).Should(BeTrue())

			DnsFqdn := getRRsetName(unquotedResource)

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, unquotedRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(unquotedResourceDNSName+"."+zoneRef+".", unquotedResourceType, FAILED_STATUS, unquotedResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(getMockedRecordsForType(DnsFqdn, unquotedResourceType)).To(Equal([]string{}), "RRset should not have been created in backend")
			Expect(*createdResource.Status.SyncStatus).To(Equal(FAILED_STATUS), "RRset status should be 'Failed'")
			Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
			Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneRef), "RRset should have setOwnerReference to Zone")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
		})
	})

	Context("When creating a RRset with an existing RRset with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("wrong-rrset", "already-existing"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			existingResourceName := "existing.example2.org"
			existingResourceNamespace := zoneNamespace
			existingResourceDNSName := "test"
			existingResourceType := "A"
			existingResourceRecords := []string{"1.2.3.4", "5.6.7.8"}
			existingResourceComment := "This a duplicate RRset"
			existingResourceTTL := uint32(300)

			By("Creating the RRset resource")
			existingResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      existingResourceName,
					Namespace: existingResourceNamespace,
				},
			}
			existingResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, existingResource, func() error {
				existingResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: zoneName,
						Kind: resourceZoneKind,
					},
					Type:    existingResourceType,
					Name:    existingResourceDNSName,
					TTL:     existingResourceTTL,
					Records: existingResourceRecords,
					Comment: &existingResourceComment,
				}
				return nil
			})
			existingRRsetLookupKey := types.NamespacedName{
				Name:      existingResourceName,
				Namespace: existingResourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, existingRRsetLookupKey, existingResource)
				return err == nil && existingResource.Status.SyncStatus != nil
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, existingRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(existingResourceDNSName+"."+zoneRef+".", existingResourceType, FAILED_STATUS, existingResourceName, resourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(*createdResource.Status.SyncStatus).To(Equal(FAILED_STATUS), "RRset status should be 'Failed'")
			Expect(createdResource.GetFinalizers()).To(ContainElement(RESOURCES_FINALIZER_NAME), "RRset should contain the finalizer")
			Expect(createdResource.GetFinalizers()).To(ContainElement(METRICS_FINALIZER_NAME), "RRset should contain the metrics finalizer")
		})
	})

	Context("When creating a RRset with a non-existing Zone", func() {
		It("should reconcile the resource with Pending status", Label("pending-rrset", "non-existing-zone"), func() {
			ic := countRrsetsMetrics()
			ctx := context.Background()
			// Specific test variables
			pendingZoneName := "example5.org"
			pendingResourceName := "test.example5.org"
			pendingResourceNamespace := "example5"
			pendingResourceDNSName := "test"
			pendingResourceType := "A"
			pendingResourceRecords := []string{"1.2.3.4", "5.6.7.8"}
			pendingResourceComment := "This a pending RRset"
			pendingResourceTTL := uint32(300)

			By("Creating the RRset resource")
			pendingResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pendingResourceName,
					Namespace: pendingResourceNamespace,
				},
			}
			pendingResource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, pendingResource, func() error {
				pendingResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: pendingZoneName,
						Kind: resourceZoneKind,
					},
					Type:    pendingResourceType,
					Name:    pendingResourceDNSName,
					TTL:     pendingResourceTTL,
					Records: pendingResourceRecords,
					Comment: &pendingResourceComment,
				}
				return nil
			})
			pendingRRsetLookupKey := types.NamespacedName{
				Name:      pendingResourceName,
				Namespace: pendingResourceNamespace,
			}

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, pendingRRsetLookupKey, pendingResource)
				return err == nil && pendingResource.Status.SyncStatus != nil
			}, timeout, interval).Should(BeTrue())

			By("Getting the created resource")
			createdResource := &dnsv1alpha2.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, pendingRRsetLookupKey, createdResource)
				return err == nil && createdResource.IsInExpectedStatus(FIRST_GENERATION, PENDING_STATUS)
			}, timeout, interval).Should(BeTrue())

			Expect(countRrsetsMetrics()-ic).To(Equal(1), "One more metric should have been created")
			Expect(getRrsetMetricWithLabels(pendingResourceDNSName+"."+pendingZoneName+".", pendingResourceType, PENDING_STATUS, pendingResourceName, pendingResourceNamespace)).To(Equal(1.0), "metric should be 1.0")
			Expect(*createdResource.Status.SyncStatus).To(Equal(PENDING_STATUS), "RRset status should be 'Pending'")
			Expect(createdResource.GetFinalizers()).To(ContainElement(METRICS_FINALIZER_NAME), "RRset should contain the metrics finalizer")
		})
	})

	Context("When creating a ClusterRRset with an existing RRset with same FQDN", func() {
		It("should reconcile the resource with Failed status", Label("clusterrrset-creation", "existing-rrset"), func() {
			ctx := context.Background()
			// Specific test variables
			recreationZoneName := "example7.org"
			recreationZoneKind := NATIVE_KIND_ZONE
			recreationZoneNameservers := []string{"ns1.example7.org", "ns2.example7.org"}

			recreationResourceName := "test.example7.org"
			recreationResourceNamespace := "example7"
			recreationResourceZoneKind := "ClusterZone"
			recreationResourceDNSName := "test"
			recreationResourceTTL := uint32(300)
			recreationResourceType := "A"
			recreationResourceComment := "Just a comment"
			recreationResourceRecords := []string{"1.2.3.4", "5.6.7.8"}

			By("Creating a ClusterZone")
			recreationZone := &dnsv1alpha2.ClusterZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: recreationZoneName,
				},
			}
			recreationZone.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, recreationZone, func() error {
				recreationZone.Spec = dnsv1alpha2.ZoneSpec{
					Kind:        recreationZoneKind,
					Nameservers: recreationZoneNameservers,
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			recreationZoneLookupKey := types.NamespacedName{
				Name: recreationZoneName,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, recreationZoneLookupKey, recreationZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			Eventually(func() bool {
				_, found := readFromZonesMap(makeCanonical(recreationZone.Name))
				return found
			}, timeout, interval).Should(BeTrue())

			By("Creating a RRset")
			recreationResource := &dnsv1alpha2.RRset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      recreationResourceName,
					Namespace: recreationResourceNamespace,
				},
			}
			recreationResource.SetResourceVersion("")
			_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, recreationResource, func() error {
				recreationResource.Spec = dnsv1alpha2.RRsetSpec{
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: recreationZoneName,
						Kind: recreationResourceZoneKind,
					},
					Type:    recreationResourceType,
					Name:    recreationResourceDNSName,
					TTL:     recreationResourceTTL,
					Records: recreationResourceRecords,
					Comment: &recreationResourceComment,
				}
				return nil
			})
			recreationRRsetLookupKey := types.NamespacedName{
				Name:      recreationResourceName,
				Namespace: recreationResourceNamespace,
			}
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, recreationRRsetLookupKey, recreationResource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// Confirm that resource is created in the backend
			DnsFqdn := getRRsetName(recreationResource)
			Eventually(func() bool {
				_, ok := readFromRecordsMap(makeCanonical(DnsFqdn))
				return ok
			}, timeout, interval).Should(BeTrue())

			By("Creating a ClusterRRset")
			resource := &dnsv1alpha2.ClusterRRset{
				ObjectMeta: metav1.ObjectMeta{
					Name: recreationResourceName,
				},
			}
			resource.SetResourceVersion("")
			_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha2.RRsetSpec{
					Type:    recreationResourceType,
					Name:    recreationResourceDNSName,
					TTL:     recreationResourceTTL,
					Records: recreationResourceRecords,
					Comment: &recreationResourceComment,
					ZoneRef: dnsv1alpha2.ZoneRef{
						Name: recreationZoneName,
						Kind: recreationResourceZoneKind,
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the resource")
			recreatedZone := &dnsv1alpha2.ClusterRRset{}
			typeNamespacedName := types.NamespacedName{
				Name: recreationResourceName,
			}
			// Waiting for the resource to be fully modified
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, recreatedZone)
				return err == nil && recreatedZone.IsInExpectedStatus(FIRST_GENERATION, FAILED_STATUS)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
