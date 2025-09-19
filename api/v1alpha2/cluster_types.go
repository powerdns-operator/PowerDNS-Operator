/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package v1alpha2

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterTLSConfig defines TLS configuration for PowerDNS API connection
type ClusterTLSConfig struct {
	// Insecure enables insecure connections to PowerDNS API (skip TLS verification)
	// +kubebuilder:default:=false
	// +optional
	Insecure *bool `json:"insecure,omitempty"`

	// CABundleRef is a reference to a ConfigMap or Secret containing a CA bundle
	// +optional
	CABundleRef *ClusterCABundleRef `json:"caBundleRef,omitempty"`
}

// ClusterCABundleRef defines a reference to a CA bundle in a ConfigMap or Secret
type ClusterCABundleRef struct {
	// Name is the name of the ConfigMap or Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the ConfigMap or Secret
	// If not specified, defaults to the operator namespace
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Kind is the kind of resource (ConfigMap or Secret)
	// +kubebuilder:validation:Enum=ConfigMap;Secret
	// +kubebuilder:default:="ConfigMap"
	// +optional
	Kind *string `json:"kind,omitempty"`

	// Key is the key in the ConfigMap or Secret containing the CA bundle
	// +kubebuilder:default:="ca.crt"
	// +optional
	Key *string `json:"key,omitempty"`
}

// ClusterCredentials defines credentials configuration for PowerDNS API
type ClusterCredentials struct {
	// SecretRef is a reference to a Kubernetes Secret containing the PowerDNS API key
	// +kubebuilder:validation:Required
	SecretRef ClusterSecretRef `json:"secretRef"`
}

// ClusterSecretRef defines a reference to a Secret containing API credentials
type ClusterSecretRef struct {
	// Name is the name of the Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Secret
	// If not specified, defaults to the cluster resource namespace
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Key is the key in the Secret containing the API key
	// +kubebuilder:default:="apiKey"
	// +optional
	Key *string `json:"key,omitempty"`
}

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// Interval is the reconciliation interval to check the connection to the PowerDNS API
	// +kubebuilder:default:="5m"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// URL is the URL of the PowerDNS API
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// Vhost is the vhost/server ID of the PowerDNS API, defaults to "localhost"
	// +kubebuilder:default:="localhost"
	// +optional
	Vhost *string `json:"vhost,omitempty"`

	// Timeout is the timeout for PowerDNS API requests, defaults to 10s
	// +kubebuilder:default:="10s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Proxy is the URL of the HTTP/HTTPS proxy to use for connecting to PowerDNS API
	// Format: http://proxy.example.com:8080 or https://proxy.example.com:8080
	// +optional
	Proxy *string `json:"proxy,omitempty"`

	// TLS defines TLS configuration for PowerDNS API connection
	// +optional
	TLS *ClusterTLSConfig `json:"tls,omitempty"`

	// Credentials defines credentials configuration for PowerDNS API
	// +kubebuilder:validation:Required
	Credentials ClusterCredentials `json:"credentials"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// ConnectionStatus indicates the status of the connection to the PowerDNS API
	// +optional
	ConnectionStatus *string `json:"connectionStatus,omitempty"`

	// PowerDNSVersion is the version of the connected PowerDNS server
	// +optional
	PowerDNSVersion *string `json:"powerDNSVersion,omitempty"`

	// DaemonType is the type of PowerDNS daemon (should be "authoritative")
	// +optional
	DaemonType *string `json:"daemonType,omitempty"`

	// ServerID is the ID of the PowerDNS server
	// +optional
	ServerID *string `json:"serverID,omitempty"`

	// LastConnectionTime is the last time a successful connection was established
	// +optional
	LastConnectionTime *metav1.Time `json:"lastConnectionTime,omitempty"`

	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed for this cluster
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
// +kubebuilder:printcolumn:name="Connection Status",type="string",JSONPath=".status.connectionStatus"
// +kubebuilder:printcolumn:name="PowerDNS Version",type="string",JSONPath=".status.powerDNSVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Cluster
	// +required
	Spec ClusterSpec `json:"spec"`

	// status defines the observed state of Cluster
	// +optional
	Status ClusterStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

// IsConnectionHealthy returns true if the cluster has a healthy connection to PowerDNS
func (c *Cluster) IsConnectionHealthy() bool {
	return c.Status.ConnectionStatus != nil && *c.Status.ConnectionStatus == "Connected"
}

// GetVhost returns the API vhost, defaulting to "localhost" if not specified
func (c *Cluster) GetVhost() string {
	if c.Spec.Vhost != nil {
		return *c.Spec.Vhost
	}
	return "localhost"
}

// GetTimeout returns the API timeout, defaulting to 10 seconds if not specified
func (c *Cluster) GetTimeout() time.Duration {
	if c.Spec.Timeout != nil {
		return c.Spec.Timeout.Duration
	}
	return 10 * time.Second
}

// GetInterval returns the reconciliation interval, defaulting to 5 minutes if not specified
func (c *Cluster) GetInterval() time.Duration {
	if c.Spec.Interval != nil {
		return c.Spec.Interval.Duration
	}
	return 5 * time.Minute
}

// GetTLSInsecure returns the TLS insecure setting, defaulting to false if not specified
func (c *Cluster) GetTLSInsecure() bool {
	if c.Spec.TLS != nil && c.Spec.TLS.Insecure != nil {
		return *c.Spec.TLS.Insecure
	}
	return false
}

// GetCredentialsSecretName returns the credentials secret name
func (c *Cluster) GetCredentialsSecretName() string {
	return c.Spec.Credentials.SecretRef.Name
}

// GetCredentialsSecretNamespace returns the credentials secret namespace, defaulting to cluster namespace if not specified
func (c *Cluster) GetCredentialsSecretNamespace() string {
	if c.Spec.Credentials.SecretRef.Namespace != nil {
		return *c.Spec.Credentials.SecretRef.Namespace
	}
	return c.Namespace
}

// GetCredentialsSecretKey returns the credentials secret key, defaulting to "apiKey" if not specified
func (c *Cluster) GetCredentialsSecretKey() string {
	if c.Spec.Credentials.SecretRef.Key != nil {
		return *c.Spec.Credentials.SecretRef.Key
	}
	return "apiKey"
}

// GetCABundleRefKind returns the CA bundle reference kind, defaulting to "ConfigMap" if not specified
func (c *Cluster) GetCABundleRefKind() string {
	if c.Spec.TLS != nil && c.Spec.TLS.CABundleRef != nil && c.Spec.TLS.CABundleRef.Kind != nil {
		return *c.Spec.TLS.CABundleRef.Kind
	}
	return "ConfigMap"
}

// GetCABundleRefKey returns the CA bundle reference key, defaulting to "ca.crt" if not specified
func (c *Cluster) GetCABundleRefKey() string {
	if c.Spec.TLS != nil && c.Spec.TLS.CABundleRef != nil && c.Spec.TLS.CABundleRef.Key != nil {
		return *c.Spec.TLS.CABundleRef.Key
	}
	return "ca.crt"
}

// GetCABundleRefNamespace returns the CA bundle reference namespace, defaulting to cluster namespace if not specified
func (c *Cluster) GetCABundleRefNamespace() string {
	if c.Spec.TLS != nil && c.Spec.TLS.CABundleRef != nil && c.Spec.TLS.CABundleRef.Namespace != nil {
		return *c.Spec.TLS.CABundleRef.Namespace
	}
	return c.Namespace
}
