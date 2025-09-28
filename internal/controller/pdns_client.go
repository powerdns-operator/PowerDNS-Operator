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

	dnsv1alpha3 "github.com/powerdns-operator/powerdns-operator/api/v1alpha3"
)

// newPDNSClientFunc allows overriding the client creation for tests
var newPDNSClientFunc = newPDNSClientFromProvider

// GetPDNSClient returns a PDNS client for the specified provider
func GetPDNSClient(ctx context.Context, kubeClient client.Client, providerRef string) (PdnsClienter, error) {
	pdnsClient, err := newPDNSClientFunc(ctx, kubeClient, providerRef)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to create PDNS client for provider '%s': %w", providerRef, err)
	}
	return pdnsClient, nil
}

// newPDNSClientFromProvider creates a PDNS client from a PDNSProvider resource
func newPDNSClientFromProvider(ctx context.Context, kubeClient client.Client, providerName string) (PdnsClienter, error) {
	// Validate pdnsprovider name
	if providerName == "" {
		return PdnsClienter{}, fmt.Errorf("pdnsprovider name cannot be empty")
	}

	// Get PDNSProvider resource from Kubernetes
	provider := &dnsv1alpha3.PDNSProvider{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: providerName}, provider); err != nil {
		return PdnsClienter{}, fmt.Errorf("pdnsprovider '%s' not found: %w", providerName, err)
	}

	// Validate provider configuration
	if provider.Spec.URL == "" {
		return PdnsClienter{}, fmt.Errorf("pdnsprovider '%s' has no API URL configured", providerName)
	}

	// Get API key from Kubernetes secret
	apiKey, err := extractAPIKey(ctx, kubeClient, provider)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to get API key for pdnsprovider '%s': %w", providerName, err)
	}

	// Configure HTTP client with TLS settings
	httpClient, err := newHTTPClient(ctx, kubeClient, provider)
	if err != nil {
		return PdnsClienter{}, fmt.Errorf("failed to create HTTP client for pdnsprovider '%s': %w", providerName, err)
	}

	// Create PowerDNS client using the go-powerdns library
	powerDNSClient := powerdns.New(provider.Spec.URL, provider.GetVhost(),
		powerdns.WithAPIKey(apiKey), powerdns.WithHTTPClient(httpClient))

	return PdnsClienter{Records: powerDNSClient.Records, Zones: powerDNSClient.Zones}, nil
}

func extractAPIKey(ctx context.Context, kubeClient client.Client, provider *dnsv1alpha3.PDNSProvider) (string, error) {
	secret := &corev1.Secret{}
	secretName := provider.GetCredentialsSecretName()
	secretNamespace := provider.GetCredentialsSecretNamespace()
	secretKey := provider.GetCredentialsSecretKey()

	if secretName == "" {
		return "", fmt.Errorf("no secret reference configured")
	}

	if err := kubeClient.Get(ctx, client.ObjectKey{
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

func newHTTPClient(ctx context.Context, kubeClient client.Client, provider *dnsv1alpha3.PDNSProvider) (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: provider.GetTLSInsecure()}

	// Handle CA certificate if provided via CA bundle reference
	if provider.Spec.TLS != nil && provider.Spec.TLS.CABundleRef != nil {
		caBundleData, err := getCABundleData(ctx, kubeClient, provider)
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
	if provider.Spec.Proxy != nil && *provider.Spec.Proxy != "" {
		proxyURL, err := url.Parse(*provider.Spec.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL '%s': %w", *provider.Spec.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   provider.GetTimeout(),
	}, nil
}

func getCABundleData(ctx context.Context, kubeClient client.Client, provider *dnsv1alpha3.PDNSProvider) ([]byte, error) {
	if provider.Spec.TLS == nil || provider.Spec.TLS.CABundleRef == nil {
		return nil, fmt.Errorf("CA bundle reference is nil")
	}

	caBundleRef := provider.Spec.TLS.CABundleRef
	kind := provider.GetCABundleRefKind()
	key := provider.GetCABundleRefKey()

	// Use the namespace from the CA bundle ref if specified, otherwise use operator namespace
	// Note: PDNSProvider is cluster-scoped so it has no namespace
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
		err := kubeClient.Get(ctx, objKey, secret)
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
		err := kubeClient.Get(ctx, objKey, configMap)
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
