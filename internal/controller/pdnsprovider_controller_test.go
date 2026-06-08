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

//nolint:goconst
package controller

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dnsv1alpha3 "github.com/powerdns-operator/powerdns-operator/api/v1alpha3"
)

var _ = Describe("PDNSProvider Controller", func() {

	const (
		resourceName      = "test-pdnsprovider"
		resourceURL       = "http://localhost:8081/api/v1"
		resourceSecretRef = "test-powerdns-secret"
		resourceNamespace = "default"
		resourceAPIKeyRef = "apikey"

		timeout  = time.Second * 5
		interval = time.Millisecond * 250
	)

	typeNamespacedName := types.NamespacedName{
		Name: resourceName,
		// PDNSProvider is cluster-scoped, so no namespace
	}

	Context("When reconciling a resource", func() {
		BeforeEach(func() {
			ctx := context.Background()
			By("creating the PDNSProvider resource")
			resource := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
			}
			resource.SetResourceVersion("")
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, resource, func() error {
				resource.Spec = dnsv1alpha3.PDNSProviderSpec{
					URL: resourceURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      resourceSecretRef,
							Namespace: ptr.To(resourceNamespace),
							Key:       ptr.To(resourceAPIKeyRef),
						},
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			ctx := context.Background()
			By("Cleanup the specific resource instance PDNSProvider")
			resource := &dnsv1alpha3.PDNSProvider{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				// Resource exists, delete it
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespacedName, resource)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			} else if !errors.IsNotFound(err) {
				// Unexpected error
				Expect(err).NotTo(HaveOccurred())
			}
			// If resource is already deleted (NotFound), nothing to do
		})
		It("should successfully reconcile the resource", func() {
			ctx := context.Background()
			By("Reconciling the created resource")
			controllerReconciler := &PDNSProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// First reconcile might fail due to finalizer addition, retry if needed
			const retryDelay = 100 * time.Millisecond
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			if err != nil {
				// If the error is about object modification, retry once
				if strings.Contains(err.Error(), "the object has been modified") {
					// Wait briefly before retry to avoid race conditions
					time.Sleep(retryDelay)
					result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource was processed correctly")
			updatedProvider := &dnsv1alpha3.PDNSProvider{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedProvider)
				return err == nil && controllerutil.ContainsFinalizer(updatedProvider, RESOURCES_FINALIZER_NAME)
			}, timeout, interval).Should(BeTrue())

			By("Verifying the resource status is updated")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, updatedProvider)
				return err == nil && updatedProvider.Status.Conditions != nil && len(updatedProvider.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			_ = result // Use the result to avoid unused variable warnings
		})

		It("should handle resource deletion", func() {
			ctx := context.Background()
			By("Getting the resource")
			resource := &dnsv1alpha3.PDNSProvider{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Verifying the resource is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
