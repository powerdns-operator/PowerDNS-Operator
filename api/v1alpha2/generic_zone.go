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
	SetSynchronizationFailed(err error)
	SetAvailable(zoneRes *powerdns.Zone)
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

func (c *Zone) SetSynchronizationFailed(err error) {
	setZoneSynchronizationFailed(&c.Status, c.Generation, err)
}

func (c *Zone) SetAvailable(zoneRes *powerdns.Zone) {
	setZoneAvailable(&c.Status, c.Generation, zoneRes)
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

func (c *ClusterZone) SetSynchronizationFailed(err error) {
	setZoneSynchronizationFailed(&c.Status, c.Generation, err)
}

func (c *ClusterZone) SetAvailable(zoneRes *powerdns.Zone) {
	setZoneAvailable(&c.Status, c.Generation, zoneRes)
}

func setZoneDuplicated(status *ZoneStatus, generation int64) {
	status.SyncStatus = ptr.To(FAILED_STATUS)
	status.ObservedGeneration = &generation
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             DUPLICATED_REASON,
		Message:            ZONE_DUPLICATED_MESSAGE,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneSynchronizationFailed(status *ZoneStatus, generation int64, err error) {
	status.SyncStatus = ptr.To(FAILED_STATUS)
	status.ObservedGeneration = &generation
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             SYNCHRONIZATION_FAILED_REASON,
		Message:            SYNCHRONIZATION_FAILED_MESSAGE + err.Error(),
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}

func setZoneAvailable(status *ZoneStatus, generation int64, zoneRes *powerdns.Zone) {
	status.SyncStatus = ptr.To(SUCCEEDED_STATUS)
	status.ObservedGeneration = &generation
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
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             SUCCEEDED_REASON,
		Message:            SUCCEEDED_MESSAGE,
	}
	meta.SetStatusCondition(&status.Conditions, condition)
}
