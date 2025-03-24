/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

//nolint:dupl
package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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
	GetTypeMeta() *metav1.TypeMeta

	GetSpec() *ZoneSpec
	GetStatus() ZoneStatus
	SetStatus(status ZoneStatus)
	Copy() GenericZone
}

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
var _ GenericZone = &Zone{}

func (c *Zone) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
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

// +kubebuilder:object:root:false
// +kubebuilder:object:generate:false
var _ GenericZone = &ClusterZone{}

func (c *ClusterZone) GetObjectMeta() *metav1.ObjectMeta {
	return &c.ObjectMeta
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
