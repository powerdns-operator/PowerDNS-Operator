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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ownObject(ctx context.Context, zone dnsv1alpha2.GenericZone, rrset dnsv1alpha2.GenericRRset, scheme *runtime.Scheme, cl client.Client, log logr.Logger) error {
	err := ctrl.SetControllerReference(zone, rrset, scheme)
	if err != nil {
		log.Error(err, "Failed to set owner reference. Is there already a controller managing this object?")
		return err
	}
	return cl.Update(ctx, rrset)
}
