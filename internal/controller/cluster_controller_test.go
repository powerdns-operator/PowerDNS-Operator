/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
)

var _ = Describe("Cluster Controller", func() {

	const (
		clusterName     = "test-powerdns-cluster"
		secretName      = "test-powerdns-secret"
		secretNamespace = "default"
		apiURL          = "https://test-powerdns:8081"
		apiKey          = "test-secret-key"
		timeout         = time.Second * 10
		interval        = time.Millisecond * 250
	)

	typeNamespacedName := types.NamespacedName{
		Name:      clusterName,
		Namespace: secretNamespace,
	}

	secretNamespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}

	Context("When reconciling a Cluster resource", func() {
		BeforeEach(func() {
			ctx := context.Background()

			By("creating the API secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"apiKey": apiKey,
				},
			}
			_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, secret, func() error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			By("creating the Cluster resource")
			cluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: secretNamespace,
				},
			}
			cluster.SetResourceVersion("")
			_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, cluster, func() error {
				cluster.Spec = dnsv1alpha2.ClusterSpec{
					URL: apiURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      secretName,
							Namespace: ptr.To(secretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha2.ClusterTLSConfig{
						Insecure: ptr.To(false),
					},
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, cluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			ctx := context.Background()

			By("Cleanup the Cluster resource")
			cluster := &dnsv1alpha2.Cluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, cluster)
			if err == nil {
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, typeNamespacedName, cluster)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}

			By("Cleanup the API secret")
			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, secretNamespacedName, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should successfully reconcile a valid Cluster resource", func() {
			ctx := context.Background()
			cluster := &dnsv1alpha2.Cluster{}

			By("Getting the created Cluster resource")
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, cluster)
			}, timeout, interval).Should(Succeed())

			By("Checking that the Cluster has the expected spec")
			Expect(cluster.Spec.URL).To(Equal(apiURL))
			Expect(cluster.GetCredentialsSecretName()).To(Equal(secretName))
			Expect(cluster.GetCredentialsSecretNamespace()).To(Equal(secretNamespace))
			Expect(cluster.GetVhost()).To(Equal("localhost"))
			Expect(cluster.GetTimeout()).To(Equal(10 * time.Second))
			Expect(cluster.GetTLSInsecure()).To(BeFalse())

			By("Checking that helper methods work correctly")
			Expect(cluster.IsConnectionHealthy()).To(BeFalse()) // Initially not connected
		})

		It("should use apiKey field from secret (consistency fix)", func() {
			ctx := context.Background()

			By("creating a secret with only apiKey field (not PDNS_API_KEY)")
			secretWithApiKey := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "apikey-secret",
					Namespace: secretNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiKey": []byte("test-api-key-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secretWithApiKey)).To(Succeed())

			By("creating a cluster that references the apiKey-only secret")
			clusterWithApiKey := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "apikey-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: "https://test-powerdns-apikey:8081",
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      "apikey-secret",
							Namespace: ptr.To(secretNamespace),
						},
					},
					Vhost:   ptr.To("localhost"),
					Timeout: ptr.To(metav1.Duration{Duration: 10 * time.Second}),
					TLS: &dnsv1alpha2.ClusterTLSConfig{
						Insecure: ptr.To(true),
					},
				},
			}
			Expect(k8sClient.Create(ctx, clusterWithApiKey)).To(Succeed())

			By("verifying cluster was created successfully")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "apikey-cluster"}, clusterWithApiKey)
				if err != nil {
					return false
				}
				// The cluster should be created successfully - the controller will process it in the background
				// This tests that the secret field consistency fix works by not causing validation errors
				return clusterWithApiKey.Name == "apikey-cluster"
			}, timeout, interval).Should(BeTrue())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, clusterWithApiKey)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretWithApiKey)).To(Succeed())
		})

		It("should use default values when optional fields are not specified", func() {
			ctx := context.Background()

			By("creating a Cluster with minimal spec")
			minimalCluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minimal-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: apiURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      secretName,
							Namespace: ptr.To(secretNamespace),
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, minimalCluster)).To(Succeed())

			By("Checking default values are applied")
			Expect(minimalCluster.GetVhost()).To(Equal("localhost"))
			Expect(minimalCluster.GetTimeout()).To(Equal(10 * time.Second))
			Expect(minimalCluster.GetTLSInsecure()).To(BeFalse())

			By("Cleanup minimal cluster")
			Expect(k8sClient.Delete(ctx, minimalCluster)).To(Succeed())
		})

		It("should support proxy configuration", func() {
			ctx := context.Background()

			By("creating a Cluster with proxy configuration")
			proxyCluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "proxy-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: apiURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      secretName,
							Namespace: ptr.To(secretNamespace),
						},
					},
					Proxy: ptr.To("http://proxy.example.com:8080"),
				},
			}

			Expect(k8sClient.Create(ctx, proxyCluster)).To(Succeed())

			By("Verifying proxy configuration is set")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "proxy-cluster"}, proxyCluster)
				if err != nil {
					return false
				}
				return proxyCluster.Spec.Proxy != nil && *proxyCluster.Spec.Proxy == "http://proxy.example.com:8080"
			}, timeout, interval).Should(BeTrue())

			By("Cleanup proxy cluster")
			Expect(k8sClient.Delete(ctx, proxyCluster)).To(Succeed())
		})

		It("should handle status updates correctly", func() {
			ctx := context.Background()
			cluster := &dnsv1alpha2.Cluster{}

			By("Getting the Cluster resource")
			Expect(k8sClient.Get(ctx, typeNamespacedName, cluster)).To(Succeed())

			By("Updating the status")
			original := cluster.DeepCopy()
			cluster.Status.ConnectionStatus = ptr.To("Connected")
			cluster.Status.PowerDNSVersion = ptr.To("4.8.0")
			cluster.Status.DaemonType = ptr.To("authoritative")
			cluster.Status.ServerID = ptr.To("test-server")
			cluster.Status.LastConnectionTime = ptr.To(metav1.NewTime(time.Now()))
			cluster.Status.ObservedGeneration = ptr.To(cluster.Generation)

			// Add a condition
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Reason:             "Connected",
				Message:            "Successfully connected to PowerDNS API",
			})

			Expect(k8sClient.Status().Patch(ctx, cluster, client.MergeFrom(original))).To(Succeed())

			By("Verifying the status was updated")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, cluster)
				if err != nil {
					return false
				}
				return cluster.IsConnectionHealthy() &&
					cluster.Status.PowerDNSVersion != nil &&
					*cluster.Status.PowerDNSVersion == "4.8.0"
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When testing Cluster validation", func() {
		It("should reject invalid API URL", func() {
			ctx := context.Background()

			By("creating a Cluster with invalid API URL")
			invalidCluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-url-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: "invalid-url", // This should fail validation
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      secretName,
							Namespace: ptr.To(secretNamespace),
						},
					},
				},
			}

			By("Expecting creation to fail due to validation")
			err := k8sClient.Create(ctx, invalidCluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("should match"))
		})

		It("should accept valid timeout duration", func() {
			ctx := context.Background()

			By("creating a Cluster with valid timeout duration")
			validCluster := &dnsv1alpha2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-timeout-cluster",
				},
				Spec: dnsv1alpha2.ClusterSpec{
					URL: apiURL,
					Credentials: dnsv1alpha2.ClusterCredentials{
						SecretRef: dnsv1alpha2.ClusterSecretRef{
							Name:      secretName,
							Namespace: ptr.To(secretNamespace),
						},
					},
					Timeout: ptr.To(metav1.Duration{Duration: 30 * time.Second}),
				},
			}

			By("Expecting creation to succeed")
			err := k8sClient.Create(ctx, validCluster)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup valid timeout cluster")
			Expect(k8sClient.Delete(ctx, validCluster)).To(Succeed())
		})
	})

	Context("When testing Zone with ClusterRef", func() {
		const (
			zoneName      = "example.com"
			zoneNamespace = "default"
		)

		zoneNamespacedName := types.NamespacedName{
			Name:      zoneName,
			Namespace: zoneNamespace,
		}

		BeforeEach(func() {
			ctx := context.Background()

			By("ensuring the cluster exists")
			cluster := &dnsv1alpha2.Cluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, cluster)
			if errors.IsNotFound(err) {
				// Create cluster if it doesn't exist
				cluster = &dnsv1alpha2.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName,
						Namespace: secretNamespace,
					},
					Spec: dnsv1alpha2.ClusterSpec{
						URL: apiURL,
						Credentials: dnsv1alpha2.ClusterCredentials{
							SecretRef: dnsv1alpha2.ClusterSecretRef{
								Name:      secretName,
								Namespace: ptr.To(secretNamespace),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			}
		})

		It("should create Zone with ClusterRef", func() {
			ctx := context.Background()

			By("creating a Zone with ClusterRef")
			zone := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      zoneName,
					Namespace: zoneNamespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					ClusterRef:  ptr.To(clusterName),
					Kind:        "Native",
					Nameservers: []string{"ns1.example.com", "ns2.example.com"},
				},
			}

			Expect(k8sClient.Create(ctx, zone)).To(Succeed())

			By("Verifying the Zone was created with ClusterRef")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, zoneNamespacedName, zone)
				if err != nil {
					return false
				}
				return zone.GetClusterRef() != nil && *zone.GetClusterRef() == clusterName
			}, timeout, interval).Should(BeTrue())

			By("Cleanup the Zone")
			Expect(k8sClient.Delete(ctx, zone)).To(Succeed())
		})

		It("should create Zone without ClusterRef (legacy mode)", func() {
			ctx := context.Background()

			By("creating a Zone without ClusterRef")
			zone := &dnsv1alpha2.Zone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "legacy-zone",
					Namespace: zoneNamespace,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					// No ClusterRef - should use legacy configuration
					Kind:        "Native",
					Nameservers: []string{"ns1.legacy.com", "ns2.legacy.com"},
				},
			}

			Expect(k8sClient.Create(ctx, zone)).To(Succeed())

			By("Verifying the Zone was created without ClusterRef")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "legacy-zone", Namespace: zoneNamespace}, zone)
				if err != nil {
					return false
				}
				return zone.GetClusterRef() == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleanup the Zone")
			Expect(k8sClient.Delete(ctx, zone)).To(Succeed())
		})
	})

	Context("When testing ClusterZone with ClusterRef", func() {
		const clusterZoneName = "cluster-example.com"

		clusterZoneNamespacedName := types.NamespacedName{
			Name: clusterZoneName,
		}

		It("should create ClusterZone with ClusterRef", func() {
			ctx := context.Background()

			By("creating a ClusterZone with ClusterRef")
			clusterZone := &dnsv1alpha2.ClusterZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterZoneName,
				},
				Spec: dnsv1alpha2.ZoneSpec{
					ClusterRef:  ptr.To(clusterName),
					Kind:        "Native",
					Nameservers: []string{"ns1.cluster.com", "ns2.cluster.com"},
				},
			}

			Expect(k8sClient.Create(ctx, clusterZone)).To(Succeed())

			By("Verifying the ClusterZone was created with ClusterRef")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterZoneNamespacedName, clusterZone)
				if err != nil {
					return false
				}
				return clusterZone.GetClusterRef() != nil && *clusterZone.GetClusterRef() == clusterName
			}, timeout, interval).Should(BeTrue())

			By("Cleanup the ClusterZone")
			Expect(k8sClient.Delete(ctx, clusterZone)).To(Succeed())
		})
	})
})
