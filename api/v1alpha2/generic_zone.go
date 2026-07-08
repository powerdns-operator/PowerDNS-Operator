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

//nolint:dupl
package v1alpha2

import (
	"time"

	"github.com/joeig/go-powerdns/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// +kubebuilder:object:root=false
// +kubebuilder:object:generate:false
// +k8s:deepcopy-gen:interfaces=nil
// +k8s:deepcopy-gen=nil

// GenericZone is a common interface for interacting with ClusterZone
// or a namespaced Zone.
type GenericZone interface {
	runtime.Object
	metav1.Object

	GetObjectMeta() *metav1.ObjectMeta
	GetKind() string
	GetTypeMeta() *metav1.TypeMeta

	GetSpec() *ZoneSpec
	GetStatus() ZoneStatus
	SetStatus(status ZoneStatus)
	Copy() GenericZone

	// Set Status functions
	SetDuplicated()
	SetValidated()
	SetUnprocessable(stage string, err error)
	SetBadRequest(stage string, err error)
	SetSynchronizationFailed(stage string, err error)
	SetProcessed()
	SetAvailable(zoneRes *powerdns.Zone)
	UnsetProcessed()
	UnsetAvailable()
	SetSyncStatus()
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
var _ GenericZone = &Zone{}

func (c *Zone) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
}

func (c *Zone) GetKind() string {
	return "Zone"
}

func (c *Zone) GetTypeMeta() *metav1.TypeMeta {
	return &c.TypeMeta
}

func (c *Zone) GetSpec() *ZoneSpec {
	return &c.Spec
}

func (c *Zone) GetStatus() ZoneStatus {
	return c.Status
}

func (c *Zone) SetStatus(status ZoneStatus) {
	c.Status = status
}

func (c *Zone) Copy() GenericZone {
	return c.DeepCopy()
}

func (c *Zone) SetDuplicated() {
	setZoneDuplicated(&c.Status, c.Generation)
}

func (c *Zone) SetValidated() {
	setZoneValidated(&c.Status, c.Generation)
}

func (c *Zone) SetUnprocessable(stage string, err error) {
	setZoneUnprocessable(stage, &c.Status, c.Generation, err)
}

func (c *Zone) SetBadRequest(stage string, err error) {
	setZoneBadRequest(stage, &c.Status, c.Generation, err)
}

func (c *Zone) SetSynchronizationFailed(stage string, err error) {
	setZoneSynchronizationFailed(stage, &c.Status, c.Generation, err)
}

func (c *Zone) SetProcessed() {
	setZoneProcessed(&c.Status, c.Generation)
}

func (c *Zone) SetAvailable(zoneRes *powerdns.Zone) {
	setZoneAvailable(&c.Status, c.Generation, zoneRes)
}

func (c *Zone) UnsetProcessed() {
	unsetZoneProcessed(&c.Status)
}

func (c *Zone) UnsetAvailable() {
	unsetZoneAvailable(&c.Status)
}

func (c *Zone) SetSyncStatus() {
	setZoneSyncStatusAndGeneration(&c.Status, c.Generation)
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
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

func (c *ClusterZone) SetAvailable(zoneRes *powerdns.Zone) {
	setZoneAvailable(&c.Status, c.Generation, zoneRes)
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

func setZoneDuplicated(status *ZoneStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Valid",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "Duplicated",
		Message:            "At least another ClusterZone/Zone exists with the same name",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneValidated(status *ZoneStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Valid",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "Valid",
		Message:            "Zone validated",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneUnprocessable(stage string, status *ZoneStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "Unprocessable",
		Message:            "Unprocessable:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneBadRequest(stage string, status *ZoneStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "BadRequest",
		Message:            "BadRequest:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneSynchronizationFailed(stage string, status *ZoneStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "SynchronizationFailed",
		Message:            "Synchronization failed:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneProcessed(status *ZoneStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Processed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "Processed",
		Message:            "Processed",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneAvailable(status *ZoneStatus, generation int64, zoneRes *powerdns.Zone) {
	status.ID = zoneRes.ID
	status.Name = zoneRes.Name
	status.Kind = ptr.To(string(ptr.Deref(zoneRes.Kind, "")))
	status.Serial = zoneRes.Serial
	status.NotifiedSerial = zoneRes.NotifiedSerial
	status.EditedSerial = zoneRes.EditedSerial
	status.Masters = zoneRes.Masters
	status.DNSsec = zoneRes.DNSsec
	status.Catalog = zoneRes.Catalog
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "Succeeded",
		Message:            "Succeeded",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func unsetZoneProcessed(status *ZoneStatus) {
	meta.RemoveStatusCondition(&status.Conditions, "Processed")
}

func unsetZoneAvailable(status *ZoneStatus) {
	meta.RemoveStatusCondition(&status.Conditions, "Available")
}

func calculateZoneSyncStatusAndGeneration(status *ZoneStatus, generation int64) (string, int64) {
	var validGeneration, processedGeneration, availableGeneration int64
	var validCondition, processedCondition, availableCondition, hasProcessedCondition bool

	if c := meta.FindStatusCondition(status.Conditions, "Valid"); c != nil {
		validCondition = c.Status == metav1.ConditionTrue
		validGeneration = c.ObservedGeneration
	}

	if c := meta.FindStatusCondition(status.Conditions, "Processed"); c != nil {
		hasProcessedCondition = true
		processedCondition = c.Status == metav1.ConditionTrue
		processedGeneration = c.ObservedGeneration
	}

	if c := meta.FindStatusCondition(status.Conditions, "Available"); c != nil {
		availableGeneration = c.ObservedGeneration
		availableCondition = c.Status == metav1.ConditionTrue
	}

	// The Zone/ClusterZone is available for a previous generation
	if availableCondition && availableGeneration < generation {
		return "Stale", availableGeneration
	}

	if !validCondition {
		return "Invalid", validGeneration
	}

	if !hasProcessedCondition {
		return "Valid", validGeneration
	}

	if !processedCondition {
		return "Unprocessed", processedGeneration
	}

	if !availableCondition {
		return "Processed", processedGeneration
	}

	return "Synced", availableGeneration
}

func setZoneSyncStatusAndGeneration(status *ZoneStatus, generation int64) {
	calculatedSyncStatus, calculatedGeneration := calculateZoneSyncStatusAndGeneration(status, generation)
	status.SyncStatus = ptr.To(calculatedSyncStatus)
	status.ObservedGeneration = &generation
	status.SyncGeneration = &calculatedGeneration
}
