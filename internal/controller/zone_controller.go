/*
 * Software Name : PowerDNS-Operator
 *
 * SPDX-FileCopyrightText: Copyright (c) Orange Business Services SA
 * SPDX-License-Identifier: Apache-2.0
 *
 * This software is distributed under the Apache 2.0 License,
 * see the "LICENSE" file for more details
 */

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
)

const (
	FINALIZER_NAME             = "dns.cav.enablers.ob/finalizer"
	DEFAULT_TTL_FOR_NS_RECORDS = uint32(1500)

	ZONE_NOT_FOUND_MSG  = "Not Found"
	ZONE_NOT_FOUND_CODE = 404
	ZONE_CONFLICT_MSG   = "Conflict"
	ZONE_CONFLICT_CODE  = 409
)

const (
	ZoneReasonSynced                  = "ZoneSynced"
	ZoneMessageSyncSucceeded          = "Zone synced with PowerDNS instance"
	ZoneReasonSynchronizationFailed   = "SynchronizationFailed"
	ZoneReasonNSSynchronizationFailed = "NSSynchronizationFailed"
)

// ZoneReconciler reconciles a Zone object
type ZoneReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/finalizers,verbs=update

func (r *ZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile Zone", "Zone.Name", req.Name)

	// Get Zone
	zone := &dnsv1alpha1.Zone{}
	err := r.Get(ctx, req.NamespacedName, zone)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize variable to represent Zone situation
	isDeleted := !zone.ObjectMeta.DeletionTimestamp.IsZero()

	return zoneReconcile(ctx, zone, isDeleted, r.Client, r.PDNSClient, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha1.Zone{}).
		Owns(&dnsv1alpha1.RRset{}).
		Complete(r)
}
