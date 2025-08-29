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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	// ApiURL is the URL of the PowerDNS API
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.*`
	ApiURL string `json:"apiUrl"`

	// ApiSecretRef is a reference to a Kubernetes Secret containing the PowerDNS API key
	// The secret must contain a key named "apiKey"
	// +kubebuilder:validation:Required
	ApiSecretRef corev1.SecretReference `json:"apiSecretRef"`

	// ApiVhost is the vhost of the PowerDNS API, defaults to "localhost"
	// +kubebuilder:default:="localhost"
	// +optional
	ApiVhost *string `json:"apiVhost,omitempty"`

	// ApiTimeout is the timeout for PowerDNS API requests in seconds, defaults to 10
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +optional
	ApiTimeout *int `json:"apiTimeout,omitempty"`

	// ApiInsecure enables insecure connections to PowerDNS API (skip TLS verification)
	// +kubebuilder:default:=false
	// +optional
	ApiInsecure *bool `json:"apiInsecure,omitempty"`

	// ApiCAPath is the path to the certificate authority file for TLS verification
	// This should be a path to a mounted secret or configmap in the operator pod
	// +optional
	ApiCAPath *string `json:"apiCAPath,omitempty"`

	// ProxyURL is the URL of the HTTP/HTTPS proxy to use for connecting to PowerDNS API
	// Format: http://proxy.example.com:8080 or https://proxy.example.com:8080
	// +optional
	ProxyURL *string `json:"proxyUrl,omitempty"`
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

// +kubebuilder:printcolumn:name="API URL",type="string",JSONPath=".spec.apiUrl"
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

// GetApiVhost returns the API vhost, defaulting to "localhost" if not specified
func (c *Cluster) GetApiVhost() string {
	if c.Spec.ApiVhost != nil {
		return *c.Spec.ApiVhost
	}
	return "localhost"
}

// GetApiTimeout returns the API timeout, defaulting to 10 seconds if not specified
func (c *Cluster) GetApiTimeout() int {
	if c.Spec.ApiTimeout != nil {
		return *c.Spec.ApiTimeout
	}
	return 10
}

// GetApiInsecure returns the API insecure setting, defaulting to false if not specified
func (c *Cluster) GetApiInsecure() bool {
	if c.Spec.ApiInsecure != nil {
		return *c.Spec.ApiInsecure
	}
	return false
}
