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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		zoneNS1  = "ns1.example.org"
		zoneNS2  = "ns2.example.org"

		// RRset
		resourceName      = "test.example2.org"
		resourceNamespace = "default"
		resourceTTL       = uint32(300)
		resourceType      = "A"
		zoneIdRef         = zoneName

		testRecord1 = "127.0.0.1"
		testRecord2 = "127.0.0.2"
		testRecord3 = "127.0.0.3"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	// Global
	resourceRecords := []string{testRecord1, testRecord2}
	ctx := context.Background()

	// Zone
	zoneLookupKey := types.NamespacedName{
		Name: zoneName,
	}
	zone := &dnsv1alpha1.Zone{
		ObjectMeta: metav1.ObjectMeta{
			Name: zoneName,
		},
		Spec: dnsv1alpha1.ZoneSpec{
			Kind:        zoneKind,
			Nameservers: []string{zoneNS1, zoneNS2},
		},
	}

	// RRset
	rssetLookupKey := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}
	resource := &dnsv1alpha1.RRset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: resourceNamespace,
		},
		Spec: dnsv1alpha1.RRsetSpec{
			ZoneRef: dnsv1alpha1.ZoneRef{
				Name: zoneIdRef,
			},
			Type:    resourceType,
			TTL:     resourceTTL,
			Records: resourceRecords,
		},
	}

	BeforeEach(func() {
		By("Creating the Zone resource")
		zone.SetResourceVersion("")
		_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, zone, func() error {
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, zoneLookupKey, zone)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("Ensure the resource does not already exists")
		err = k8sClient.Get(ctx, rssetLookupKey, resource)
		Expect(err).To(HaveOccurred())

		By("Creating the RRset resource")
		resource.SetResourceVersion("")
		_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(1 * time.Second)
		By("Getting the created RRset resource")
		createdResource := &dnsv1alpha1.RRset{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, rssetLookupKey, createdResource)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("Verifying the created RRset resource")
		Expect(createdResource.Spec.Records).To(Equal(resourceRecords))
		Expect(createdResource.Spec.Type).To(Equal(resourceType))
		Expect(createdResource.Spec.TTL).To(Equal(resourceTTL))
		Expect(createdResource.Spec.ZoneRef.Name).To(Equal(zoneIdRef))
		Expect(createdResource.GetOwnerReferences()).NotTo(BeEmpty(), "RRset should have setOwnerReference")
		Expect(createdResource.GetOwnerReferences()[0].Name).To(Equal(zoneIdRef), "RRset should have setOwnerReference to Zone")
		Expect(createdResource.GetFinalizers()).To(ContainElement(FINALIZER_NAME), "RRset should contain the finalizer")
	})

	AfterEach(func() {
		err := k8sClient.Get(ctx, rssetLookupKey, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleanup the specific resource instance RRset")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		By("Verifying the resource has been deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, rssetLookupKey, resource)
			return err != nil
		}, timeout, interval).Should(BeTrue())

		By("Cleanup the specific resource instance Zone")
		Expect(k8sClient.Delete(ctx, zone)).To(Succeed())
	})

	Context("Updating RRset", func() {
		It("should successfully reconcile the resource", Label("rrset-modification"), func() {
			By("Getting the existing resource")
			rrset := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, rrset)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(rrset.Spec.Records).To(Equal(resourceRecords))

			By("Getting the initial Serial of the zone")
			zone := &dnsv1alpha1.Zone{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, zone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			initialSerial := zone.Status.Serial

			By("Updating RRset records")
			updatedRecords := []string{testRecord3}
			rrset.Spec.Records = updatedRecords
			Expect(k8sClient.Update(ctx, rrset)).To(Succeed())

			By("Getting the updated resource")
			updatedRRset := &dnsv1alpha1.RRset{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rssetLookupKey, updatedRRset)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(updatedRRset.Spec.Records).To(Equal(updatedRecords))
			Expect(updatedRRset.Spec.Type).To(Equal(resourceType))
			Expect(updatedRRset.Spec.TTL).To(Equal(resourceTTL))
			Expect(updatedRRset.Spec.ZoneRef.Name).To(Equal(zoneIdRef))
			Expect(updatedRRset.GetOwnerReferences()).NotTo(BeEmpty(), "rrset should have setOwnerReference")
			Expect(updatedRRset.GetOwnerReferences()[0].Name).To(Equal(zoneIdRef), "updatedRRset should have setOwnerReference to Zone")
			Expect(updatedRRset.GetFinalizers()).To(ContainElement(FINALIZER_NAME), "RRset should contain the finalizer")

			By("Getting the modified zone")
			modifiedZone := &dnsv1alpha1.Zone{}
			// Waiting for the resource to be fully modified
			time.Sleep(2 * time.Second)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneLookupKey, modifiedZone)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			//
			expectedSerial := *initialSerial + uint32(1)
			Expect(*(modifiedZone.Status.Serial)).To(Equal(expectedSerial), "Serial should be incremented")
		})
	})
})
