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

	"time"

	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha3 "github.com/powerdns-operator/powerdns-operator/api/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PDNSProviderReasonConnected        = "Connected"
	PDNSProviderMessageConnected       = "Successfully connected to PowerDNS API"
	PDNSProviderReasonConnectionFailed = "ConnectionFailed"
	PDNSProviderReasonSecretNotFound   = "SecretNotFound"
	PDNSProviderMessageSecretNotFound  = "Referenced secret not found"
)

// PDNSProviderReconciler reconciles a PDNSProvider object
type PDNSProviderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=pdnsproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=pdnsproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=pdnsproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *PDNSProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile PDNSProvider", "PDNSProvider.Name", req.Name)

	// Get PDNSProvider
	pdnsprovider := &dnsv1alpha3.PDNSProvider{}
	err := r.Get(ctx, req.NamespacedName, pdnsprovider)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize variables
	isDeleted := !pdnsprovider.DeletionTimestamp.IsZero()

	// Handle finalizer
	if !isDeleted {
		if !controllerutil.ContainsFinalizer(pdnsprovider, RESOURCES_FINALIZER_NAME) {
			controllerutil.AddFinalizer(pdnsprovider, RESOURCES_FINALIZER_NAME)
			if err := r.Update(ctx, pdnsprovider); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(pdnsprovider, RESOURCES_FINALIZER_NAME) {
			controllerutil.RemoveFinalizer(pdnsprovider, RESOURCES_FINALIZER_NAME)
			if err := r.Update(ctx, pdnsprovider); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	return r.reconcilePDNSProvider(ctx, pdnsprovider)
}

func (r *PDNSProviderReconciler) reconcilePDNSProvider(ctx context.Context, pdnsprovider *dnsv1alpha3.PDNSProvider) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get API key from secret
	apiKey, err := r.getAPIKeyFromSecret(ctx, pdnsprovider)
	if err != nil {
		log.Error(err, "Failed to get API key from secret")
		r.updatePDNSProviderStatus(ctx, pdnsprovider, "Failed", nil, PDNSProviderReasonSecretNotFound, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check PowerDNS connection
	serverInfo, err := r.checkPowerDNSConnection(ctx, pdnsprovider, apiKey)
	if err != nil {
		log.Error(err, "Failed to connect to PowerDNS")
		r.updatePDNSProviderStatus(ctx, pdnsprovider, "Failed", nil, PDNSProviderReasonConnectionFailed, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Update status with success
	r.updatePDNSProviderStatus(ctx, pdnsprovider, "Connected", serverInfo, PDNSProviderReasonConnected, PDNSProviderMessageConnected)
	return ctrl.Result{RequeueAfter: pdnsprovider.GetInterval()}, nil
}

func (r *PDNSProviderReconciler) getAPIKeyFromSecret(ctx context.Context, pdnsprovider *dnsv1alpha3.PDNSProvider) (string, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      pdnsprovider.GetCredentialsSecretName(),
		Namespace: pdnsprovider.GetCredentialsSecretNamespace(),
	}

	err := r.Get(ctx, secretKey, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", secretKey.Namespace, secretKey.Name, err)
	}

	keyName := pdnsprovider.GetCredentialsSecretKey()
	apiKey, exists := secret.Data[keyName]
	if !exists {
		return "", fmt.Errorf("%s not found in secret %s/%s", keyName, secretKey.Namespace, secretKey.Name)
	}

	return string(apiKey), nil
}

func (r *PDNSProviderReconciler) checkPowerDNSConnection(ctx context.Context, pdnsprovider *dnsv1alpha3.PDNSProvider, apiKey string) (map[string]*string, error) {
	// Create HTTP client
	tlsConfig := &tls.Config{InsecureSkipVerify: pdnsprovider.GetTLSInsecure()}

	// Handle CA bundle if specified
	if pdnsprovider.Spec.TLS != nil && pdnsprovider.Spec.TLS.CABundleRef != nil {
		caBundleData, err := r.getCABundleData(ctx, pdnsprovider)
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

	// Configure proxy if specified
	if pdnsprovider.Spec.Proxy != nil && *pdnsprovider.Spec.Proxy != "" {
		proxyURL, err := url.Parse(*pdnsprovider.Spec.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   pdnsprovider.GetTimeout(),
	}

	// Create PowerDNS client and check connection
	pdnsClient := powerdns.New(pdnsprovider.Spec.URL, pdnsprovider.GetVhost(),
		powerdns.WithAPIKey(apiKey), powerdns.WithHTTPClient(httpClient))

	timeoutCtx, cancel := context.WithTimeout(ctx, pdnsprovider.GetTimeout())
	defer cancel()

	server, err := pdnsClient.Servers.Get(timeoutCtx, pdnsprovider.GetVhost())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PowerDNS: %w", err)
	}

	// Validate authoritative server
	if server.DaemonType != nil && *server.DaemonType != "authoritative" {
		return nil, fmt.Errorf("PowerDNS server is not authoritative, got: %s", *server.DaemonType)
	}

	return map[string]*string{
		"version":    server.Version,
		"daemonType": server.DaemonType,
		"serverID":   server.ID,
	}, nil
}

func (r *PDNSProviderReconciler) getCABundleData(ctx context.Context, pdnsprovider *dnsv1alpha3.PDNSProvider) ([]byte, error) {
	if pdnsprovider.Spec.TLS == nil || pdnsprovider.Spec.TLS.CABundleRef == nil {
		return nil, fmt.Errorf("CA bundle reference is nil")
	}

	caBundleRef := pdnsprovider.Spec.TLS.CABundleRef
	kind := pdnsprovider.GetCABundleRefKind()
	key := pdnsprovider.GetCABundleRefKey()
	namespace := pdnsprovider.GetCABundleRefNamespace()

	objKey := types.NamespacedName{
		Name:      caBundleRef.Name,
		Namespace: namespace,
	}

	if kind == "Secret" {
		secret := &corev1.Secret{}
		err := r.Get(ctx, objKey, secret)
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
		err := r.Get(ctx, objKey, configMap)
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

func (r *PDNSProviderReconciler) updatePDNSProviderStatus(ctx context.Context, pdnsprovider *dnsv1alpha3.PDNSProvider,
	status string, serverInfo map[string]*string, reason, message string) {

	original := pdnsprovider.DeepCopy()
	now := metav1.NewTime(time.Now())

	// Update basic status
	pdnsprovider.Status.ConnectionStatus = &status
	pdnsprovider.Status.ObservedGeneration = &pdnsprovider.Generation

	// Update server info if available
	if serverInfo != nil {
		pdnsprovider.Status.PowerDNSVersion = serverInfo["version"]
		pdnsprovider.Status.DaemonType = serverInfo["daemonType"]
		pdnsprovider.Status.ServerID = serverInfo["serverID"]

		// Only update LastConnectionTime if connection status changed or significant time passed
		shouldUpdateTime := pdnsprovider.Status.ConnectionStatus == nil ||
			*pdnsprovider.Status.ConnectionStatus != status ||
			pdnsprovider.Status.LastConnectionTime == nil ||
			time.Since(pdnsprovider.Status.LastConnectionTime.Time) > 4*time.Minute

		if shouldUpdateTime {
			pdnsprovider.Status.LastConnectionTime = &now
		}
	}

	// Update condition
	conditionStatus := metav1.ConditionTrue
	if status != "Connected" {
		conditionStatus = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&pdnsprovider.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pdnsprovider.Generation,
	})

	if err := r.Status().Patch(ctx, pdnsprovider, client.MergeFrom(original)); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "Failed to update pdnsprovider status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PDNSProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha3.PDNSProvider{}).
		Owns(&dnsv1alpha3.ClusterZone{}).
		Owns(&dnsv1alpha3.Zone{}).
		Complete(r)
}
