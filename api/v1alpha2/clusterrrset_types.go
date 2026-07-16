/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) PowerDNS-Operator contributors
 * SPDX-FileCopyrightText: Copyright (c) 2025 Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package v1alpha2

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.zoneRef.name"
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.dnsEntryName"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="TTL",type="integer",JSONPath=".spec.ttl"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// +kubebuilder:printcolumn:name="Records",type="string",JSONPath=".spec.records"
// ClusterRRset is the Schema for the clusterrrsets API
type ClusterRRset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterRRset
	// +required
	Spec RRsetSpec `json:"spec"`

	// status defines the observed state of ClusterRRset
	// +optional
	Status RRsetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterRRsetList contains a list of ClusterRRset
type ClusterRRsetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterRRset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterRRset{}, &ClusterRRsetList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (r *ClusterRRset) IsInExpectedStatus(
	expectedMinimumObservedGeneration int64,
	expectedSyncStatus string,
	expectedConditionStatus metav1.ConditionStatus,
) bool {
	currentAvailableCondition := meta.FindStatusCondition(r.Status.Conditions, "Available")
	return r.Status.ObservedGeneration != nil &&
		*r.Status.ObservedGeneration >= expectedMinimumObservedGeneration &&
		r.Status.SyncStatus != nil &&
		*r.Status.SyncStatus == expectedSyncStatus &&
		currentAvailableCondition != nil &&
		currentAvailableCondition.Status == expectedConditionStatus
}

var _ GenericRRset = &ClusterRRset{}

func (c *ClusterRRset) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
}

func (c *ClusterRRset) GetKind() string {
	return "ClusterRRset"
}

func (c *ClusterRRset) GetTypeMeta() *metav1.TypeMeta {
	return &c.TypeMeta
}

func (c *ClusterRRset) GetSpec() *RRsetSpec {
	return &c.Spec
}

func (c *ClusterRRset) GetStatus() RRsetStatus {
	return c.Status
}

func (c *ClusterRRset) SetStatus(status RRsetStatus) {
	c.Status = status
}

func (c *ClusterRRset) Copy() GenericRRset {
	return c.DeepCopy()
}

func (c *ClusterRRset) GetDomain() string {
	return fmt.Sprintf("%s.", strings.TrimSuffix(c.Spec.ZoneRef.Name, "."))
}

func (c *ClusterRRset) SetMissingZone() {
	setMissingZone(&c.Status, c.Generation)
}

func (c *ClusterRRset) SetZoneNotAvailable(zoneName string) {
	setZoneNotAvailable(&c.Status, c.Generation, zoneName)
}

func (c *ClusterRRset) SetDuplicated() {
	setRRsetDuplicated(&c.Status, c.Generation)
}

func (c *ClusterRRset) SetValidated() {
	setRRsetValidated(&c.Status, c.Generation)
}

func (c *ClusterRRset) SetUnprocessable(stage string, err error) {
	setRRsetUnprocessable(stage, &c.Status, c.Generation, err)
}

func (c *ClusterRRset) SetBadRequest(stage string, err error) {
	setRRsetBadRequest(stage, &c.Status, c.Generation, err)
}

func (c *ClusterRRset) SetSynchronizationFailed(stage string, err error) {
	setRRsetSynchronizationFailed(stage, &c.Status, c.Generation, err)
}

func (c *ClusterRRset) SetProcessed() {
	setRRsetProcessed(&c.Status, c.Generation)
}

func (c *ClusterRRset) SetAvailable(name string) {
	setRRsetAvailable(&c.Status, c.Generation, name)
}

func (c *ClusterRRset) SetSyncStatus(name string) {
	setRRsetSyncStatus(&c.Status, c.Generation, name)
}
