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

const (
	RESOURCES_FINALIZER_NAME   = "dns.cav.enablers.ob/external-resources"
	METRICS_FINALIZER_NAME     = "dns.cav.enablers.ob/metrics"
	DEFAULT_TTL_FOR_NS_RECORDS = uint32(1500)

	NOT_FOUND_ERROR_MSG  = "Not Found"
	NOT_FOUND_ERROR_CODE = 404
	CONFLICT_ERROR_MSG   = "Conflict"
	CONFLICT_ERROR_CODE  = 409
)

func ownObject(ctx context.Context, zone dnsv1alpha2.GenericZone, rrset dnsv1alpha2.GenericRRset, scheme *runtime.Scheme, cl client.Client, log logr.Logger) error {
	err := ctrl.SetControllerReference(zone, rrset, scheme)
	if err != nil {
		log.Error(err, "Failed to set owner reference. Is there already a controller managing this object?")
		return err
	}
	return cl.Update(ctx, rrset)
}
