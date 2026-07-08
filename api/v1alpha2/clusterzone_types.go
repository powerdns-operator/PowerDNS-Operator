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
	"github.com/joeig/go-powerdns/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="Serial",type="integer",JSONPath=".status.serial"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.syncStatus"
// ClusterZone is the Schema for the clusterzones API
type ClusterZone struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterZone
	// +required
	Spec ZoneSpec `json:"spec"`

	// status defines the observed state of ClusterZone
	// +optional
	Status ZoneStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterZoneList contains a list of ClusterZone
type ClusterZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterZone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterZone{}, &ClusterZoneList{})
}

// IsInExpectedStatus returns true if Status.SyncStatus and Status.ObservedGeneration are, at least, at expected value
func (z *ClusterZone) IsInExpectedStatus(
	expectedMinimumObservedGeneration int64,
	expectedSyncStatus string,
	expectedConditionStatus metav1.ConditionStatus,
) bool {
	currentAvailableCondition := meta.FindStatusCondition(z.Status.Conditions, "Available")
	return z.Status.ObservedGeneration != nil &&
		*z.Status.ObservedGeneration >= expectedMinimumObservedGeneration &&
		z.Status.SyncStatus != nil &&
		*z.Status.SyncStatus == expectedSyncStatus &&
		currentAvailableCondition != nil &&
		currentAvailableCondition.Status == expectedConditionStatus
}

var _ GenericZone = &ClusterZone{}

func (c *ClusterZone) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
}

func (c *ClusterZone) GetKind() string {
	return "ClusterZone"
}

func (c *ClusterZone) GetTypeMeta() *metav1.TypeMeta {
	return &c.TypeMeta
}

func (c *ClusterZone) GetSpec() *ZoneSpec {
	return &c.Spec
}

func (c *ClusterZone) GetStatus() ZoneStatus {
	return c.Status
}

func (c *ClusterZone) SetStatus(status ZoneStatus) {
	c.Status = status
}

func (c *ClusterZone) Copy() GenericZone {
	return c.DeepCopy()
}

func (c *ClusterZone) SetDuplicated() {
	setZoneDuplicated(&c.Status, c.Generation)
}

func (c *ClusterZone) SetAvailable(zoneRes *powerdns.Zone) {
	setZoneAvailable(&c.Status, c.Generation, zoneRes)
}

func (c *ClusterZone) SetValidated() {
	setZoneValidated(&c.Status, c.Generation)
}

func (c *ClusterZone) SetUnprocessable(stage string, err error) {
	setZoneUnprocessable(stage, &c.Status, c.Generation, err)
}

func (c *ClusterZone) SetBadRequest(stage string, err error) {
	setZoneBadRequest(stage, &c.Status, c.Generation, err)
}

func (c *ClusterZone) SetSynchronizationFailed(stage string, err error) {
	setZoneSynchronizationFailed(stage, &c.Status, c.Generation, err)
}

func (c *ClusterZone) SetProcessed() {
	setZoneProcessed(&c.Status, c.Generation)
}

func (c *ClusterZone) UnsetProcessed() {
	unsetZoneProcessed(&c.Status)
}

func (c *ClusterZone) UnsetAvailable() {
	unsetZoneAvailable(&c.Status)
}

func (c *ClusterZone) SetSyncStatus() {
	setZoneSyncStatusAndGeneration(&c.Status, c.Generation)
}
