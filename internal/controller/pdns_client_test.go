/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1alpha3 "github.com/powerdns-operator/powerdns-operator/api/v1alpha3"
)

var _ = Describe("PDNS Client Selection", func() {
	var (
		ctx                   context.Context
		originalNewPDNSClient func(context.Context, client.Client, string) (PdnsClienter, error)
		testPDNSProviderName  = "test-pdnsprovider"
		testSecretName        = "test-secret"
		testSecretNamespace   = "default"
		testAPIKey            = "test-api-key"
		testAPIURL            = "https://test-powerdns:8081"
	)

	BeforeEach(func() {
		ctx = context.Background()
		// Save the current function and restore the real implementation for these tests
		originalNewPDNSClient = newPDNSClientFunc
		newPDNSClientFunc = newPDNSClientFromProvider
	})

	AfterEach(func() {
		// Restore the original function (which is the mock for other tests)
		newPDNSClientFunc = originalNewPDNSClient
	})

	Context("GetPDNSClient function", func() {
		It("should return pdns client when providerRef is provided and pdnsprovider exists", func() {
			By("creating a test secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testSecretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiKey": []byte(testAPIKey),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating a test pdnsprovider")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: testPDNSProviderName,
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      testSecretName,
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha3.PDNSProviderTLSConfig{
						Insecure: ptr.To(true),
					},
				},
				Status: dnsv1alpha3.PDNSProviderStatus{
					ConnectionStatus: &[]string{"Connected"}[0],
				},
			}
			Expect(k8sClient.Create(ctx, pdnsprovider)).To(Succeed())

			By("calling GetPDNSClient with pdnsprovider reference")
			pdnsClient, err := GetPDNSClient(ctx, k8sClient, testPDNSProviderName)

			Expect(err).NotTo(HaveOccurred())
			Expect(pdnsClient.Records).NotTo(BeNil())
			Expect(pdnsClient.Zones).NotTo(BeNil())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, pdnsprovider)).To(Succeed())
			// Wait for pdnsprovider deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdnsprovider.Name}, &dnsv1alpha3.PDNSProvider{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should return error when pdnsprovider does not exist", func() {
			nonExistentPDNSProvider := "non-existent-pdnsprovider"
			By("calling GetPDNSClient with non-existent pdnsprovider")
			_, err := GetPDNSClient(ctx, k8sClient, nonExistentPDNSProvider)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pdnsprovider 'non-existent-pdnsprovider' not found"))
		})

		It("should return error when pdnsprovider exists but secret is missing", func() {
			By("creating a pdnsprovider without secret")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: testPDNSProviderName + "-missing-secret",
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      "missing-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pdnsprovider)).To(Succeed())

			By("calling GetPDNSClient")
			pdnsproviderName := testPDNSProviderName + "-missing-secret"
			_, err := GetPDNSClient(ctx, k8sClient, pdnsproviderName)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get secret"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, pdnsprovider)).To(Succeed())
			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdnsprovider.Name}, &dnsv1alpha3.PDNSProvider{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should return error when secret exists but apiKey is missing", func() {
			By("creating a secret without apiKey")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName + "-no-apikey",
					Namespace: testSecretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"wrongKey": []byte("some-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating a pdnsprovider")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: testPDNSProviderName + "-no-apikey",
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      testSecretName + "-no-apikey",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pdnsprovider)).To(Succeed())

			By("calling GetPDNSClient")
			pdnsproviderName := testPDNSProviderName + "-no-apikey"
			_, err := GetPDNSClient(ctx, k8sClient, pdnsproviderName)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'apiKey' field not found"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, pdnsprovider)).To(Succeed())
			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdnsprovider.Name}, &dnsv1alpha3.PDNSProvider{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should handle pdnsprovider with proxy URL", func() {
			By("creating a secret with apiKey")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "proxy-secret",
					Namespace: testSecretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiKey": []byte(testAPIKey),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating a pdnsprovider with proxy URL")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "proxy-pdnsprovider",
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      "proxy-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha3.PDNSProviderTLSConfig{
						Insecure: ptr.To(true),
					},
					Proxy: ptr.To("http://proxy.example.com:8080"),
				},
			}
			Expect(k8sClient.Create(ctx, pdnsprovider)).To(Succeed())

			By("calling GetPDNSClient with proxy pdnsprovider")
			proxyPDNSProviderName := "proxy-pdnsprovider"
			pdnsClient, err := GetPDNSClient(ctx, k8sClient, proxyPDNSProviderName)

			Expect(err).NotTo(HaveOccurred())
			Expect(pdnsClient.Records).NotTo(BeNil())
			Expect(pdnsClient.Zones).NotTo(BeNil())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, pdnsprovider)).To(Succeed())
			// Wait for pdnsprovider deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdnsprovider.Name}, &dnsv1alpha3.PDNSProvider{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should handle pdnsprovider with empty API URL", func() {
			By("creating a secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-url-secret",
					Namespace: testSecretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiKey": []byte(testAPIKey),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("attempting to create a pdnsprovider with empty API URL should fail validation")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-url-pdnsprovider",
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: "", // Empty URL
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      "empty-url-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}

			By("verifying that empty URL is rejected by CRD validation")
			err := k8sClient.Create(ctx, pdnsprovider)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.url in body should match"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})
	})

	Context("Error handling for invalid configurations", func() {
		It("should return error for invalid proxy URL", func() {
			By("creating a secret with apiKey")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-proxy-secret",
					Namespace: testSecretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiKey": []byte(testAPIKey),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("creating a pdnsprovider with invalid proxy URL")
			pdnsprovider := &dnsv1alpha3.PDNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-proxy-pdnsprovider",
				},
				Spec: dnsv1alpha3.PDNSProviderSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha3.PDNSProviderCredentials{
						SecretRef: dnsv1alpha3.PDNSProviderSecretRef{
							Name:      "invalid-proxy-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha3.PDNSProviderTLSConfig{
						Insecure: ptr.To(true),
					},
					Proxy: ptr.To("://invalid-proxy-url"), // Invalid URL format
				},
			}
			Expect(k8sClient.Create(ctx, pdnsprovider)).To(Succeed())

			By("calling GetPDNSClient should fail with proxy URL error")
			invalidProxyPDNSProviderName := "invalid-proxy-pdnsprovider"
			_, err := GetPDNSClient(ctx, k8sClient, invalidProxyPDNSProviderName)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse proxy URL"))
			Expect(err.Error()).To(ContainSubstring("://invalid-proxy-url"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, pdnsprovider)).To(Succeed())
			// Wait for pdnsprovider deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: pdnsprovider.Name}, &dnsv1alpha3.PDNSProvider{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})
	})
})
