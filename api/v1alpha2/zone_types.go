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

// ZoneSpec defines the desired state of Zone
type ZoneSpec struct {
	// Kind of the zone, one of "Native", "Master", "Slave", "Producer", "Consumer".
	// +kubebuilder:validation:Enum:=Native;Master;Slave;Producer;Consumer
	Kind string `json:"kind"`
	// List of the nameservers of the zone.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:Pattern=`^([a-zA-Z0-9-]+\.)*[a-zA-Z0-9-]+$`
	Nameservers []string `json:"nameservers"`
	// The catalog this zone is a member of
	// +optional
	Catalog *string `json:"catalog,omitempty"`
	// The SOA-EDIT-API metadata item, one of "DEFAULT", "INCREASE", "EPOCH", defaults to "DEFAULT"
	// +kubebuilder:validation:Enum:=DEFAULT;INCREASE;EPOCH
	// +kubebuilder:default:="DEFAULT"
	// +optional
	SOAEditAPI *string `json:"soa_edit_api,omitempty"`
}

// ZoneStatus defines the observed state of Zone
type ZoneStatus struct {
	// ID define the opaque zone id.
	// +optional
	ID *string `json:"id,omitempty"`
	// Name of the zone (e.g. "example.com.")
	// +optional
	Name *string `json:"name,omitempty"`
	// Kind of the zone, one of "Native", "Master", "Slave", "Producer", "Consumer".
	// +optional
	Kind *string `json:"kind,omitempty"`
	// The SOA serial number.
	// +optional
	Serial *uint32 `json:"serial,omitempty"`
	// The SOA serial notifications have been sent out for
	// +optional
	NotifiedSerial *uint32 `json:"notified_serial,omitempty"`
	// The SOA serial as seen in query responses.
	// +optional
	EditedSerial *uint32 `json:"edited_serial,omitempty"`
	// List of IP addresses configured as a master for this zone ("Slave" type zones only).
	// +optional
	Masters []string `json:"masters,omitempty"`
	// Whether or not this zone is DNSSEC signed.
	// +optional
	DNSsec *bool `json:"dnssec,omitempty"`
	// The catalog this zone is a member of.
	// +optional
	Catalog            *string            `json:"catalog,omitempty"`
	SyncStatus         *string            `json:"syncStatus,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration *int64             `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// +kubebuilder:printcolumn:name="Serial",type="integer",JSONPath=".status.serial"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// Zone is the Schema for the zones API
type Zone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZoneSpec   `json:"spec,omitempty"`
	Status ZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ZoneList contains a list of Zone
type ZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Zone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Zone{}, &ZoneList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (z *Zone) IsInExpectedStatus(expectedMinimumObservedGeneration int64, expectedSyncStatus string) bool {
	return z.Status.ObservedGeneration != nil &&
		*z.Status.ObservedGeneration >= expectedMinimumObservedGeneration &&
		z.Status.SyncStatus != nil &&
		*z.Status.SyncStatus == expectedSyncStatus
}
