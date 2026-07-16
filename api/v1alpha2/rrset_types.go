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

// RRsetSpec defines the desired state of RRset
type RRsetSpec struct {
	// Type of the record (e.g. "A", "PTR", "MX").
	Type string `json:"type"`
	// Name of the record
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Name string `json:"name"`
	// DNS TTL of the records, in seconds.
	TTL uint32 `json:"ttl"`
	// All records in this Resource Record Set.
	Records []string `json:"records"`
	// Comment on RRSet.
	// +optional
	Comment *string `json:"comment,omitempty"`
	// ZoneRef reference the zone the RRSet depends on.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ZoneRef ZoneRef `json:"zoneRef"`
}

type ZoneRef struct {
	// Name of the zone.
	Name string `json:"name"`
	// Kind of the Zone resource (Zone or ClusterZone)
	// +kubebuilder:validation:Enum:=Zone;ClusterZone
	Kind string `json:"kind"`
}

// RRsetStatus defines the observed state of RRset.
type RRsetStatus struct {
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
	DnsEntryName   *string      `json:"dnsEntryName,omitempty"`
	SyncStatus     *string      `json:"syncStatus,omitempty"`
	SyncGeneration *int64       `json:"syncGeneration,omitempty"`
	// conditions represent the current state of the RRset resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration *int64             `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:conversion:hub
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.zoneRef.name"
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.dnsEntryName"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="TTL",type="integer",JSONPath=".spec.ttl"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// +kubebuilder:printcolumn:name="Records",type="string",JSONPath=".spec.records"
// RRset is the Schema for the rrsets API
type RRset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RRset
	// +required
	Spec RRsetSpec `json:"spec"`

	// status defines the observed state of RRset
	// +optional
	Status RRsetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RRsetList contains a list of RRset
type RRsetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RRset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RRset{}, &RRsetList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (r *RRset) IsInExpectedStatus(
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

var _ GenericRRset = &RRset{}

func (c *RRset) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
}

func (c *RRset) GetKind() string {
	return "RRset"
}

func (c *RRset) GetTypeMeta() *metav1.TypeMeta {
	return &c.TypeMeta
}

func (c *RRset) GetSpec() *RRsetSpec {
	return &c.Spec
}

func (c *RRset) GetStatus() RRsetStatus {
	return c.Status
}

func (c *RRset) SetStatus(status RRsetStatus) {
	c.Status = status
}

func (c *RRset) Copy() GenericRRset {
	return c.DeepCopy()
}

func (c *RRset) GetDomain() string {
	return fmt.Sprintf("%s.", strings.TrimSuffix(c.Spec.ZoneRef.Name, "."))
}

func (c *RRset) SetMissingZone() {
	setMissingZone(&c.Status, c.Generation)
}

func (c *RRset) SetZoneNotAvailable(zoneName string) {
	setZoneNotAvailable(&c.Status, c.Generation, zoneName)
}

func (c *RRset) SetDuplicated() {
	setRRsetDuplicated(&c.Status, c.Generation)
}

func (c *RRset) SetValidated() {
	setRRsetValidated(&c.Status, c.Generation)
}

func (c *RRset) SetUnprocessable(stage string, err error) {
	setRRsetUnprocessable(stage, &c.Status, c.Generation, err)
}

func (c *RRset) SetBadRequest(stage string, err error) {
	setRRsetBadRequest(stage, &c.Status, c.Generation, err)
}

func (c *RRset) SetSynchronizationFailed(stage string, err error) {
	setRRsetSynchronizationFailed(stage, &c.Status, c.Generation, err)
}

func (c *RRset) SetProcessed() {
	setRRsetProcessed(&c.Status, c.Generation)
}

func (c *RRset) SetAvailable(name string) {
	setRRsetAvailable(&c.Status, c.Generation, name)
}

func (c *RRset) SetSyncStatus(name string) {
	setRRsetSyncStatus(&c.Status, c.Generation, name)
}
