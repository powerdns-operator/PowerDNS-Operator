/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.zoneRef.name"
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.dnsEntryName"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="TTL",type="integer",JSONPath=".spec.ttl"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// +kubebuilder:printcolumn:name="Records",type="string",JSONPath=".spec.records"
// ClusterRRset is the Schema for the clusterrrsets API
type ClusterRRset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RRsetSpec   `json:"spec,omitempty"`
	Status RRsetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterRRsetList contains a list of ClusterRRset
type ClusterRRsetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRRset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterRRset{}, &ClusterRRsetList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (r *ClusterRRset) IsInExpectedStatus(expectedMinimumObservedGeneration int64, expectedSyncStatus string) bool {
	return r.Status.ObservedGeneration != nil &&
		*r.Status.ObservedGeneration >= expectedMinimumObservedGeneration &&
		r.Status.SyncStatus != nil &&
		*r.Status.SyncStatus == expectedSyncStatus
}
