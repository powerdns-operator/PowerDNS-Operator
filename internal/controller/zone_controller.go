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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
)

const (
	RESOURCES_FINALIZER_NAME   = "dns.cav.enablers.ob/external-resources"
	METRICS_FINALIZER_NAME     = "dns.cav.enablers.ob/metrics"
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
	ZoneReasonDuplicated              = "ZoneDuplicated"
	ZoneMessageDuplicated             = "Already existing Zone with the same FQDN"
)

// ZoneReconciler reconciles a Zone object
type ZoneReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(zonesStatusesMetric)
}

//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/finalizers,verbs=update

func (r *ZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile Zone", "Zone.Name", req.Name)

	// Get Zone
	zone := &dnsv1alpha2.Zone{}
	err := r.Get(ctx, req.NamespacedName, zone)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize variable to represent Zone situation
	isModified := zone.Status.ObservedGeneration != nil && *zone.Status.ObservedGeneration != zone.GetGeneration()
	isDeleted := !zone.DeletionTimestamp.IsZero()

	// Position metrics finalizer as soon as possible
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(zone, METRICS_FINALIZER_NAME) {
			controllerutil.AddFinalizer(zone, METRICS_FINALIZER_NAME)
			if err := r.Update(ctx, zone); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	// When updating a Zone, if 'Status' is not changed, 'LastTransitionTime' will not be updated
	// So we delete condition to force new 'LastTransitionTime'
	original := zone.DeepCopy()
	if !isDeleted && isModified {
		isModified = true
		meta.RemoveStatusCondition(&zone.Status.Conditions, "Available")
		if err := r.Status().Patch(ctx, zone, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch Zone status")
			return ctrl.Result{}, err
		}
	}

	return zoneReconcile(ctx, zone, isModified, isDeleted, r.Client, r.PDNSClient, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use indexer to ensure that only one Zone/ClusterZone exists for one DNS entry
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &dnsv1alpha2.Zone{}, "Zone.Entry.Name", func(rawObj client.Object) []string {
		// grab the Zone object, extract its name...
		var ZoneName string
		if rawObj.(*dnsv1alpha2.Zone).Status.SyncStatus == nil || *rawObj.(*dnsv1alpha2.Zone).Status.SyncStatus == SUCCEEDED_STATUS {
			ZoneName = (rawObj.(*dnsv1alpha2.Zone)).Name
		}
		return []string{ZoneName}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha2.Zone{}).
		Owns(&dnsv1alpha2.ClusterRRset{}).
		Owns(&dnsv1alpha2.RRset{}).
		Complete(r)
}
