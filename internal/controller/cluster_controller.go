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
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
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
	ClusterReasonConnected        = "Connected"
	ClusterMessageConnected       = "Successfully connected to PowerDNS API"
	ClusterReasonConnectionFailed = "ConnectionFailed"
	ClusterReasonSecretNotFound   = "SecretNotFound"
	ClusterMessageSecretNotFound  = "Referenced secret not found"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile Cluster", "Cluster.Name", req.Name)

	// Get Cluster
	cluster := &dnsv1alpha2.Cluster{}
	err := r.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize variables
	isDeleted := !cluster.DeletionTimestamp.IsZero()

	// Handle finalizer
	if !isDeleted {
		if !controllerutil.ContainsFinalizer(cluster, RESOURCES_FINALIZER_NAME) {
			controllerutil.AddFinalizer(cluster, RESOURCES_FINALIZER_NAME)
			if err := r.Update(ctx, cluster); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(cluster, RESOURCES_FINALIZER_NAME) {
			controllerutil.RemoveFinalizer(cluster, RESOURCES_FINALIZER_NAME)
			if err := r.Update(ctx, cluster); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	return r.reconcileCluster(ctx, cluster)
}

func (r *ClusterReconciler) reconcileCluster(ctx context.Context, cluster *dnsv1alpha2.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get API key from secret
	apiKey, err := r.getAPIKeyFromSecret(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to get API key from secret")
		r.updateClusterStatus(ctx, cluster, "Failed", nil, ClusterReasonSecretNotFound, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check PowerDNS connection
	serverInfo, err := r.checkPowerDNSConnection(ctx, cluster, apiKey)
	if err != nil {
		log.Error(err, "Failed to connect to PowerDNS")
		r.updateClusterStatus(ctx, cluster, "Failed", nil, ClusterReasonConnectionFailed, err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Update status with success
	r.updateClusterStatus(ctx, cluster, "Connected", serverInfo, ClusterReasonConnected, ClusterMessageConnected)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *ClusterReconciler) getAPIKeyFromSecret(ctx context.Context, cluster *dnsv1alpha2.Cluster) (string, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      cluster.Spec.ApiSecretRef.Name,
		Namespace: cluster.Spec.ApiSecretRef.Namespace,
	}

	err := r.Get(ctx, secretKey, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", secretKey.Namespace, secretKey.Name, err)
	}

	apiKey, exists := secret.Data["apiKey"]
	if !exists {
		return "", fmt.Errorf("apiKey not found in secret %s/%s", secretKey.Namespace, secretKey.Name)
	}

	return string(apiKey), nil
}

func (r *ClusterReconciler) checkPowerDNSConnection(ctx context.Context, cluster *dnsv1alpha2.Cluster, apiKey string) (map[string]*string, error) {
	// Create HTTP client
	tlsConfig := &tls.Config{InsecureSkipVerify: cluster.GetApiInsecure()}

	if cluster.Spec.ApiCAPath != nil && *cluster.Spec.ApiCAPath != "" {
		caCert, err := os.ReadFile(*cluster.Spec.ApiCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Configure proxy if specified
	if cluster.Spec.ProxyURL != nil && *cluster.Spec.ProxyURL != "" {
		proxyURL, err := url.Parse(*cluster.Spec.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cluster.GetApiTimeout()) * time.Second,
	}

	// Create PowerDNS client and check connection
	pdnsClient := powerdns.New(cluster.Spec.ApiURL, cluster.GetApiVhost(),
		powerdns.WithAPIKey(apiKey), powerdns.WithHTTPClient(httpClient))

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cluster.GetApiTimeout())*time.Second)
	defer cancel()

	server, err := pdnsClient.Servers.Get(timeoutCtx, cluster.GetApiVhost())
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

func (r *ClusterReconciler) updateClusterStatus(ctx context.Context, cluster *dnsv1alpha2.Cluster,
	status string, serverInfo map[string]*string, reason, message string) {

	original := cluster.DeepCopy()
	now := metav1.NewTime(time.Now())

	// Update basic status
	cluster.Status.ConnectionStatus = &status
	cluster.Status.ObservedGeneration = &cluster.Generation

	// Update server info if available
	if serverInfo != nil {
		cluster.Status.PowerDNSVersion = serverInfo["version"]
		cluster.Status.DaemonType = serverInfo["daemonType"]
		cluster.Status.ServerID = serverInfo["serverID"]

		// Only update LastConnectionTime if connection status changed or significant time passed
		shouldUpdateTime := cluster.Status.ConnectionStatus == nil ||
			*cluster.Status.ConnectionStatus != status ||
			cluster.Status.LastConnectionTime == nil ||
			time.Since(cluster.Status.LastConnectionTime.Time) > 4*time.Minute

		if shouldUpdateTime {
			cluster.Status.LastConnectionTime = &now
		}
	}

	// Update condition
	conditionStatus := metav1.ConditionTrue
	if status != "Connected" {
		conditionStatus = metav1.ConditionFalse
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cluster.Generation,
	})

	if err := r.Status().Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "Failed to update cluster status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha2.Cluster{}).
		Named("cluster").
		Complete(r)
}
