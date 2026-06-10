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
	GetTypeMeta() *metav1.TypeMeta

	GetSpec() *RRsetSpec
	GetStatus() RRsetStatus
	SetStatus(status RRsetStatus)
	Copy() GenericRRset

	// Set Status functions
	SetDuplicated(lastUpdateTime *metav1.Time, name string)
	SetMissingZone(err error)
	SetZoneNotAvailable(zoneName string)
	SetSynchronizationFailed(lastUpdateTime *metav1.Time, err error)
	SetAvailable(lastUpdateTime *metav1.Time, name string)
	SetUnprocessable(err error)
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
var _ GenericRRset = &RRset{}

func (c *RRset) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
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
func (c *RRset) SetMissingZone(err error) {
	setMissingZone(&c.Status, c.Generation, err)
}

func (c *RRset) SetZoneNotAvailable(zoneName string) {
	setZoneNotAvailable(&c.Status, c.Generation, zoneName)
}

func (c *RRset) SetDuplicated(lastUpdateTime *metav1.Time, name string) {
	setRRsetDuplicated(&c.Status, c.Generation, lastUpdateTime, name)
}

func (c *RRset) SetSynchronizationFailed(lastUpdateTime *metav1.Time, err error) {
	setRRsetSynchronizationFailed(&c.Status, c.Generation, lastUpdateTime, err)
}

func (c *RRset) SetAvailable(lastUpdateTime *metav1.Time, name string) {
	setRRsetAvailable(&c.Status, c.Generation, lastUpdateTime, name)
}

func (c *RRset) SetUnprocessable(err error) {
	setUnprocessable(&c.Status, c.Generation, err)
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
var _ GenericRRset = &ClusterRRset{}

func (c *ClusterRRset) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
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

func (c *ClusterRRset) SetMissingZone(err error) {
	setMissingZone(&c.Status, c.Generation, err)
}

func (c *ClusterRRset) SetZoneNotAvailable(zoneName string) {
	setZoneNotAvailable(&c.Status, c.Generation, zoneName)
}

func (c *ClusterRRset) SetDuplicated(lastUpdateTime *metav1.Time, name string) {
	setRRsetDuplicated(&c.Status, c.Generation, lastUpdateTime, name)
}

func (c *ClusterRRset) SetSynchronizationFailed(lastUpdateTime *metav1.Time, err error) {
	setRRsetSynchronizationFailed(&c.Status, c.Generation, lastUpdateTime, err)
}

func (c *ClusterRRset) SetAvailable(lastUpdateTime *metav1.Time, name string) {
	setRRsetAvailable(&c.Status, c.Generation, lastUpdateTime, name)
}

func (c *ClusterRRset) SetUnprocessable(err error) {
	setUnprocessable(&c.Status, c.Generation, err)
}

func setMissingZone(status *RRsetStatus, generation int64, err error) {
	status.SyncStatus = ptr.To(PENDING_STATUS)
	status.ObservedGeneration = &generation
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             MISSING_ZONE_REASON,
		Message:            MISSING_ZONE_MESSAGE + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneNotAvailable(status *RRsetStatus, generation int64, zoneName string) {
	status.SyncStatus = ptr.To(FAILED_STATUS)
	status.ObservedGeneration = &generation
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             ZONE_NOT_AVAILABLE_REASON,
		Message:            ZONE_NOT_AVAILABLE_MESSAGE + zoneName,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetDuplicated(status *RRsetStatus, generation int64, lastUpdateTime *metav1.Time, name string) {
	status.SyncStatus = ptr.To(FAILED_STATUS)
	status.ObservedGeneration = &generation
	status.LastUpdateTime = lastUpdateTime
	status.DnsEntryName = &name
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: *lastUpdateTime,
		Reason:             DUPLICATED_REASON,
		Message:            RRSET_DUPLICATED_MESSAGE,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetSynchronizationFailed(status *RRsetStatus, generation int64, lastUpdateTime *metav1.Time, err error) {
	status.SyncStatus = ptr.To(FAILED_STATUS)
	status.ObservedGeneration = &generation
	status.LastUpdateTime = lastUpdateTime
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: *lastUpdateTime,
		Reason:             SYNCHRONIZATION_FAILED_REASON,
		Message:            SYNCHRONIZATION_FAILED_MESSAGE + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setRRsetAvailable(status *RRsetStatus, generation int64, lastUpdateTime *metav1.Time, name string) {
	status.SyncStatus = ptr.To(SUCCEEDED_STATUS)
	status.ObservedGeneration = &generation
	status.LastUpdateTime = lastUpdateTime
	status.DnsEntryName = &name
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: *lastUpdateTime,
		Reason:             SUCCEEDED_REASON,
		Message:            SUCCEEDED_MESSAGE,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setUnprocessable(status *RRsetStatus, generation int64, err error) {
	status.SyncStatus = ptr.To(UNPROCESSABLE_STATUS)
	status.ObservedGeneration = &generation
	status.LastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Reason:             UNPROCESSABLE_REASON,
		Message:            err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}
