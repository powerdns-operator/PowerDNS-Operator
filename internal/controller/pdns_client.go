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
	"time"

	"github.com/joeig/go-powerdns/v3"
	corev1 "k8s.io/api/core/v1"
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
	if cluster.Spec.ApiURL == "" {
		return PdnsClienter{}, fmt.Errorf("cluster '%s' has no API URL configured", clusterName)
	}

	// Get API secret with better error handling
	apiKey, err := getAPIKey(ctx, k8sClient, cluster)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to get API key for cluster '%s': %w", clusterName, err)
	}

	// Create PowerDNS client
	httpClient, err := createHTTPClient(cluster)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to create HTTP client for cluster '%s': %w", clusterName, err)
	}

	pdnsClient := powerdns.New(cluster.Spec.ApiURL, cluster.GetApiVhost(),
		powerdns.WithAPIKey(apiKey), powerdns.WithHTTPClient(httpClient))

	return PdnsClienter{Records: pdnsClient.Records, Zones: pdnsClient.Zones}, nil
}

func getAPIKey(ctx context.Context, k8sClient client.Client, cluster *dnsv1alpha2.Cluster) (string, error) {
	secret := &corev1.Secret{}
	secretRef := cluster.Spec.ApiSecretRef

	if secretRef.Name == "" {
		return "", fmt.Errorf("no secret reference configured")
	}

	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: secretRef.Namespace,
	}, secret); err != nil {
		return "", fmt.Errorf("failed to get secret '%s/%s': %w", secretRef.Namespace, secretRef.Name, err)
	}

	apiKey, exists := secret.Data["apiKey"]
	if !exists {
		return "", fmt.Errorf("'apiKey' field not found in secret '%s/%s'", secretRef.Namespace, secretRef.Name)
	}

	if len(apiKey) == 0 {
		return "", fmt.Errorf("'apiKey' field is empty in secret '%s/%s'", secretRef.Namespace, secretRef.Name)
	}

	return string(apiKey), nil
}

func createHTTPClient(cluster *dnsv1alpha2.Cluster) (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cluster.GetApiInsecure()}

	// Handle CA certificate if provided
	if cluster.Spec.ApiCAPath != nil && *cluster.Spec.ApiCAPath != "" {
		caCert, err := os.ReadFile(*cluster.Spec.ApiCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file '%s': %w", *cluster.Spec.ApiCAPath, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from file '%s': invalid PEM format", *cluster.Spec.ApiCAPath)
		}

		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Handle proxy configuration if provided
	if cluster.Spec.ProxyURL != nil && *cluster.Spec.ProxyURL != "" {
		proxyURL, err := url.Parse(*cluster.Spec.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL '%s': %w", *cluster.Spec.ProxyURL, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cluster.GetApiTimeout()) * time.Second,
	}, nil
}
