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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/joeig/go-powerdns/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
)

// GetPowerDNSClient returns the PowerDNS client for a zone
func GetPowerDNSClient(ctx context.Context, k8sClient client.Client, clusterRef *string, legacyClient PdnsClienter) (PdnsClienter, error) {
	// Use cluster-specific client if clusterRef is provided
	if clusterRef != nil && *clusterRef != "" {
		client, err := getClusterClient(ctx, k8sClient, *clusterRef)
		if err != nil {
			return PdnsClienter{}, fmt.Errorf("failed to get cluster client for '%s': %w", *clusterRef, err)
		}
		return client, nil
	}

	// Fall back to legacy client
	if isValidClient(legacyClient) {
		return legacyClient, nil
	}

	return PdnsClienter{}, fmt.Errorf("no PowerDNS client available: either set spec.clusterRef to reference a Cluster resource, or provide legacy configuration via environment variables")
}

// isValidClient checks if a PowerDNS client is properly configured
func isValidClient(client PdnsClienter) bool {
	return client.Records != nil && client.Zones != nil
}

func getClusterClient(ctx context.Context, k8sClient client.Client, clusterName string) (PdnsClienter, error) {
	// Validate cluster name
	if clusterName == "" {
		return PdnsClienter{}, fmt.Errorf("cluster name cannot be empty")
	}

	// Get cluster resource
	cluster := &dnsv1alpha2.Cluster{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: clusterName}, cluster); err != nil {
		return PdnsClienter{}, fmt.Errorf("cluster '%s' not found: %w", clusterName, err)
	}

	// Validate cluster configuration
	if cluster.Spec.URL == "" {
		return PdnsClienter{}, fmt.Errorf("cluster '%s' has no API URL configured", clusterName)
	}

	// Get API secret with better error handling
	apiKey, err := getAPIKey(ctx, k8sClient, cluster)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to get API key for cluster '%s': %w", clusterName, err)
	}

	// Create PowerDNS client
	httpClient, err := createHTTPClientWithContext(ctx, k8sClient, cluster)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to create HTTP client for cluster '%s': %w", clusterName, err)
	}

	pdnsClient := powerdns.New(cluster.Spec.URL, cluster.GetVhost(),
		powerdns.WithAPIKey(apiKey), powerdns.WithHTTPClient(httpClient))

	return PdnsClienter{Records: pdnsClient.Records, Zones: pdnsClient.Zones}, nil
}

func getAPIKey(ctx context.Context, k8sClient client.Client, cluster *dnsv1alpha2.Cluster) (string, error) {
	secret := &corev1.Secret{}
	secretName := cluster.GetCredentialsSecretName()
	secretNamespace := cluster.GetCredentialsSecretNamespace()
	secretKey := cluster.GetCredentialsSecretKey()

	if secretName == "" {
		return "", fmt.Errorf("no secret reference configured")
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}, secret); err != nil {
		return "", fmt.Errorf("failed to get secret '%s/%s': %w", secretNamespace, secretName, err)
	}

	apiKey, exists := secret.Data[secretKey]
	if !exists {
		return "", fmt.Errorf("'%s' field not found in secret '%s/%s'", secretKey, secretNamespace, secretName)
	}

	if len(apiKey) == 0 {
		return "", fmt.Errorf("'%s' field is empty in secret '%s/%s'", secretKey, secretNamespace, secretName)
	}

	return string(apiKey), nil
}

func createHTTPClientWithContext(ctx context.Context, k8sClient client.Client, cluster *dnsv1alpha2.Cluster) (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cluster.GetTLSInsecure()}

	// Handle CA certificate if provided via CA bundle reference
	if cluster.Spec.TLS != nil && cluster.Spec.TLS.CABundleRef != nil {
		caBundleData, err := getCABundleData(ctx, k8sClient, cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get CA bundle: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caBundleData) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Handle proxy configuration if provided
	if cluster.Spec.Proxy != nil && *cluster.Spec.Proxy != "" {
		proxyURL, err := url.Parse(*cluster.Spec.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL '%s': %w", *cluster.Spec.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cluster.GetTimeout(),
	}, nil
}

func getCABundleData(ctx context.Context, k8sClient client.Client, cluster *dnsv1alpha2.Cluster) ([]byte, error) {
	if cluster.Spec.TLS == nil || cluster.Spec.TLS.CABundleRef == nil {
		return nil, fmt.Errorf("CA bundle reference is nil")
	}

	caBundleRef := cluster.Spec.TLS.CABundleRef
	kind := cluster.GetCABundleRefKind()
	key := cluster.GetCABundleRefKey()

	// Use the namespace from the CA bundle ref if specified, otherwise use operator namespace
	// Note: Cluster is cluster-scoped so it has no namespace
	namespace := getOperatorNamespace()
	if caBundleRef.Namespace != nil {
		namespace = *caBundleRef.Namespace
	}

	objKey := types.NamespacedName{
		Name:      caBundleRef.Name,
		Namespace: namespace,
	}

	if kind == "Secret" {
		secret := &corev1.Secret{}
		err := k8sClient.Get(ctx, objKey, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %s/%s: %w", objKey.Namespace, objKey.Name, err)
		}
		data, exists := secret.Data[key]
		if !exists {
			return nil, fmt.Errorf("%s not found in secret %s/%s", key, objKey.Namespace, objKey.Name)
		}
		return data, nil
	} else {
		configMap := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, objKey, configMap)
		if err != nil {
			return nil, fmt.Errorf("failed to get configmap %s/%s: %w", objKey.Namespace, objKey.Name, err)
		}
		data, exists := configMap.Data[key]
		if !exists {
			return nil, fmt.Errorf("%s not found in configmap %s/%s", key, objKey.Namespace, objKey.Name)
		}
		return []byte(data), nil
	}
}

// getOperatorNamespace returns the operator's namespace
// It first tries to read from OPERATOR_NAMESPACE environment variable,
// then falls back to the default operator namespace
func getOperatorNamespace() string {
	if ns := os.Getenv("OPERATOR_NAMESPACE"); ns != "" {
		return ns
	}
	// Default operator namespace
	return "powerdns-operator-system"
}
