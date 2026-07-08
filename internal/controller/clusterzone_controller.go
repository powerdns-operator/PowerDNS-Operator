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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
)

// ClusterZoneReconciler reconciles a ClusterZone object
type ClusterZoneReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(clusterZonesStatusesMetric)
}

//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusterzones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusterzones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=clusterzones/finalizers,verbs=update

func (r *ClusterZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile ClusterZone", "ClusterZone.Name", req.Name)

	// Get ClusterZone
	zone := &dnsv1alpha2.ClusterZone{}
	log.V(1).Info("Getting ClusterZone", "ClusterZone.Name", req.Name)
	err := r.Get(ctx, req.NamespacedName, zone)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.V(1).Info("ClusterZone found", "ClusterZone", zone)

	// Initialize variable to represent ClusterZone situation
	isDeleted := !zone.DeletionTimestamp.IsZero()
	gzr := GenericZoneReconciler{
		Client:     r.Client,
		PDNSClient: r.PDNSClient,
		log:        log,
	}

	if isDeleted {
		log.V(1).Info("ClusterZone deleted", "ClusterZone.Name", zone.Name)
		// Delete external resources and remove finalizers
		if err := gzr.deleteZone(ctx, zone); err != nil {
			return ctrl.Result{}, err
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	log.V(1).Info("ClusterZone not deleted", "ClusterZone.Name", zone.Name)

	// Position metrics finalizer as soon as possible
	if !controllerutil.ContainsFinalizer(zone, METRICS_FINALIZER_NAME) {
		log.V(1).Info("Adding finalizer to ClusterZone")
		controllerutil.AddFinalizer(zone, METRICS_FINALIZER_NAME)
		if err := r.Update(ctx, zone); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	original := zone.DeepCopy()
	// Ensure we update the status in case of early return
	defer func() {
		zone.SetSyncStatus()
		updateZonesMetrics(zone)
		if err := r.Status().Patch(ctx, zone, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch ClusterZone status")
		}
	}()

	exists, err := gzr.alreadyExists(ctx, zone)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to determine if zone is duplicated: %w", err)
	}
	if exists {
		zone.SetDuplicated()
		// updateZonesMetrics(zone)
		return ctrl.Result{}, nil
	}

	log.V(1).Info("ClusterZone is validated status")
	zone.SetValidated()

	err = gzr.reconcileZone(ctx, zone)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ClusterZone: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use indexer to ensure that only one Zone/ClusterZone exists for one DNS entry
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &dnsv1alpha2.ClusterZone{}, "ClusterZone.Entry.Name", func(rawObj client.Object) []string {
		// grab the ClusterZone object, extract its name...
		var ZoneName string
		if rawObj.(*dnsv1alpha2.ClusterZone).Status.SyncStatus == nil ||
			*rawObj.(*dnsv1alpha2.ClusterZone).Status.SyncStatus == dnsv1alpha2.SYNCED_STATUS ||
			*rawObj.(*dnsv1alpha2.ClusterZone).Status.SyncStatus == dnsv1alpha2.STALE_STATUS {
			ZoneName = (rawObj.(*dnsv1alpha2.ClusterZone)).Name
		}
		return []string{ZoneName}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha2.ClusterZone{}).
		Owns(&dnsv1alpha2.ClusterRRset{}).
		Owns(&dnsv1alpha2.RRset{}).
		Complete(r)
}
