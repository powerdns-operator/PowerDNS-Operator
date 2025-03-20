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
// +kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="Serial",type="integer",JSONPath=".status.serial"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// ClusterZone is the Schema for the clusterzones API
type ClusterZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZoneSpec   `json:"spec,omitempty"`
	Status ZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterZoneList contains a list of ClusterZone
type ClusterZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterZone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterZone{}, &ClusterZoneList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (z *ClusterZone) IsInExpectedStatus(expectedMinimumObservedGeneration int64, expectedSyncStatus string) bool {
	return z.Status.ObservedGeneration != nil &&
		*z.Status.ObservedGeneration >= expectedMinimumObservedGeneration &&
		z.Status.SyncStatus != nil &&
		*z.Status.SyncStatus == expectedSyncStatus
}
