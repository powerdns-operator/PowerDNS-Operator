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
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// +kubebuilder:object:root=false
// +kubebuilder:object:generate:false
// +k8s:deepcopy-gen:interfaces=nil
// +k8s:deepcopy-gen=nil

// GenericRRset is a common interface for interacting with ClusterRRset or a namespaced RRset.
type GenericRRset interface {
	runtime.Object
	metav1.Object

	GetObjectMeta() *metav1.ObjectMeta
	GetKind() string
	GetTypeMeta() *metav1.TypeMeta

	GetSpec() *RRsetSpec
	GetStatus() RRsetStatus
	SetStatus(status RRsetStatus)
	Copy() GenericRRset
	GetDomain() string

	// Set Status functions
	SetDuplicated()
	SetMissingZone()
	SetValidated()
	SetZoneNotAvailable(zoneName string)
	SetUnprocessable(stage string, err error)
	SetBadRequest(stage string, err error)
	SetSynchronizationFailed(stage string, err error)
	SetProcessed()
	SetAvailable(name string)
	SetSyncStatus(name string)
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
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

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
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

func setMissingZone(status *RRsetStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Valid",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "ZoneMissing",
		Message:            "No Zone found for RRset",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneNotAvailable(status *RRsetStatus, generation int64, zoneName string) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "ZoneNotAvailable",
		Message:            "Zone not available:" + zoneName,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetDuplicated(status *RRsetStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Valid",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "Duplicated",
		Message:            "At least another ClusterRRset/RRset exists with the same name",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetValidated(status *RRsetStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Valid",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "Valid",
		Message:            "Valid",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetUnprocessable(stage string, status *RRsetStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "Unprocessable",
		Message:            "Unprocessable:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetBadRequest(stage string, status *RRsetStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "BadRequest",
		Message:            "BadRequest:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetSynchronizationFailed(stage string, status *RRsetStatus, generation int64, err error) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               stage,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "SynchronizationFailed",
		Message:            "Synchronization failed:" + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetProcessed(status *RRsetStatus, generation int64) {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Processed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "Processed",
		Message:            "Processed",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetAvailable(status *RRsetStatus, generation int64, name string) {
	status.DnsEntryName = &name
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             "Succeeded",
		Message:            "Succeeded",
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}
func calculateRRsetSyncStatusAndGeneration(status *RRsetStatus, generation int64) (string, int64) {
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

func setRRsetSyncStatus(status *RRsetStatus, generation int64, name string) {
	calculatedSyncStatus, calculatedGeneration := calculateRRsetSyncStatusAndGeneration(status, generation)
	status.SyncStatus = ptr.To(calculatedSyncStatus)
	status.ObservedGeneration = &generation
	status.SyncGeneration = &calculatedGeneration
	status.DnsEntryName = &name
}
