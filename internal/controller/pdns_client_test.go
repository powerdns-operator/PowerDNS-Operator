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

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
)

var _ = Describe("PowerDNS Client Selection", func() {
	var (
		ctx                 context.Context
		testClusterName     = "test-cluster"
		testSecretName      = "test-secret"
		testSecretNamespace = "default"
		testAPIKey          = "test-api-key"
		testAPIURL          = "https://test-powerdns:8081"
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("GetPowerDNSClient function", func() {
		It("should return cluster client when clusterRef is provided and cluster exists", func() {
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

			By("creating a test cluster")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testClusterName,
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      testSecretName,
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha2.ClusterTLSConfig{
						Insecure: ptr.To(true),
					},
				},
				Status: dnsv1alpha2.ClusterStatus{
					ConnectionStatus: &[]string{"Connected"}[0],
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("calling GetPowerDNSClient with cluster reference")
			pdnsClient, err := GetPowerDNSClient(ctx, k8sClient, &testClusterName, PdnsClienter{})

			Expect(err).NotTo(HaveOccurred())
			Expect(pdnsClient.Records).NotTo(BeNil())
			Expect(pdnsClient.Zones).NotTo(BeNil())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			// Wait for cluster deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name}, &dnsv1alpha2.Cluster{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should return error when cluster does not exist", func() {
			nonExistentCluster := "non-existent-cluster"
			By("calling GetPowerDNSClient with non-existent cluster")
			_, err := GetPowerDNSClient(ctx, k8sClient, &nonExistentCluster, PdnsClienter{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cluster 'non-existent-cluster' not found"))
		})

		It("should return error when cluster exists but secret is missing", func() {
			By("creating a cluster without secret")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testClusterName + "-missing-secret",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      "missing-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("calling GetPowerDNSClient")
			clusterName := testClusterName + "-missing-secret"
			_, err := GetPowerDNSClient(ctx, k8sClient, &clusterName, PdnsClienter{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get secret"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name}, &dnsv1alpha2.Cluster{})
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

			By("creating a cluster")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testClusterName + "-no-apikey",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      testSecretName + "-no-apikey",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("calling GetPowerDNSClient")
			clusterName := testClusterName + "-no-apikey"
			_, err := GetPowerDNSClient(ctx, k8sClient, &clusterName, PdnsClienter{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'apiKey' field not found"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			// Wait for deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name}, &dnsv1alpha2.Cluster{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should handle cluster with proxy URL", func() {
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

			By("creating a cluster with proxy URL")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "proxy-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      "proxy-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha2.ClusterTLSConfig{
						Insecure: ptr.To(true),
					},
					Proxy: ptr.To("http://proxy.example.com:8080"),
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("calling GetPowerDNSClient with proxy cluster")
			proxyClusterName := "proxy-cluster"
			pdnsClient, err := GetPowerDNSClient(ctx, k8sClient, &proxyClusterName, PdnsClienter{})

			Expect(err).NotTo(HaveOccurred())
			Expect(pdnsClient.Records).NotTo(BeNil())
			Expect(pdnsClient.Zones).NotTo(BeNil())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			// Wait for cluster deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name}, &dnsv1alpha2.Cluster{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			// Wait for secret deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
				return err != nil
			}, time.Second*10, time.Millisecond*100).Should(BeTrue())
		})

		It("should handle cluster with empty API URL", func() {
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

			By("attempting to create a cluster with empty API URL should fail validation")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-url-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: "", // Empty URL
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      "empty-url-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
				},
			}

			By("verifying that empty URL is rejected by CRD validation")
			err := k8sClient.Create(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.url in body should match"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})
	})

	Context("isValidClient function", func() {
		It("should return true for valid client", func() {
			validClient := PdnsClienter{
				Records: &powerdns.RecordsService{},
				Zones:   &powerdns.ZonesService{},
			}
			Expect(isValidClient(validClient)).To(BeTrue())
		})

		It("should return false for client with missing Records", func() {
			invalidClient := PdnsClienter{
				Zones: &powerdns.ZonesService{},
			}
			Expect(isValidClient(invalidClient)).To(BeFalse())
		})

		It("should return false for client with missing Zones", func() {
			invalidClient := PdnsClienter{
				Records: &powerdns.RecordsService{},
			}
			Expect(isValidClient(invalidClient)).To(BeFalse())
		})

		It("should return false for empty client", func() {
			emptyClient := PdnsClienter{}
			Expect(isValidClient(emptyClient)).To(BeFalse())
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

			By("creating a cluster with invalid proxy URL")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-proxy-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: testAPIURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      "invalid-proxy-secret",
							Namespace: ptr.To(testSecretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha2.ClusterTLSConfig{
						Insecure: ptr.To(true),
					},
					Proxy: ptr.To("://invalid-proxy-url"), // Invalid URL format
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("calling GetPowerDNSClient should fail with proxy URL error")
			invalidProxyCluster := "invalid-proxy-cluster"
			_, err := GetPowerDNSClient(ctx, k8sClient, &invalidProxyCluster, PdnsClienter{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse proxy URL"))
			Expect(err.Error()).To(ContainSubstring("://invalid-proxy-url"))

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			// Wait for cluster deletion to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name}, &dnsv1alpha2.Cluster{})
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
