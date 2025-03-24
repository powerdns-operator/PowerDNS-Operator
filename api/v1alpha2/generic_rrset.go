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
