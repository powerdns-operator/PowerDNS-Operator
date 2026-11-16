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

func setRRsetProcessed(status *RRsetStatus, generation int64, spec *RRsetSpec) {
	status.SyncSpec = spec.DeepCopy()
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
