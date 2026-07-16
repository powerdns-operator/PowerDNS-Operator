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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
)

// RRsetReconciler reconciles a RRset object
type RRsetReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(rrsetsStatusesMetric)
}

// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets/finalizers,verbs=update

func (r *RRsetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile RRset", "RRset.Name", req.Name)

	// RRset
	rrset := &dnsv1alpha2.RRset{}
	log.V(1).Info("Getting RRset", "RRset.Name", req.Name)
	err := r.Get(ctx, req.NamespacedName, rrset)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.V(1).Info("RRset found", "RRset", rrset)

	// Initialize variable to represent RRset situation
	isModified := rrset.Status.ObservedGeneration != nil && *rrset.Status.ObservedGeneration != rrset.GetGeneration()
	isDeleted := !rrset.DeletionTimestamp.IsZero()
	lastUpdateTime := &metav1.Time{Time: time.Now().UTC()}
	if rrset.Status.LastUpdateTime != nil {
		lastUpdateTime = rrset.Status.LastUpdateTime
	}
	//	log.V(1).Info("RRset situation", "isModified", isModified, "isDeleted", isDeleted, "lastUpdateTime", lastUpdateTime)
	grr := GenericRRsetReconciler{
		Client:     r.Client,
		PDNSClient: r.PDNSClient,
		scheme:     r.Scheme,
		log:        log,
	}

	if isDeleted {
		log.V(1).Info("RRset deleted", "RRset.Name", rrset.Name)
		// Delete external resources and remove finalizers
		if err := grr.deleteRRset(ctx, rrset); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	log.V(1).Info("RRset not deleted", "RRset.Name", rrset.Name)

	// Position metrics finalizer as soon as possible
	if !controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
		log.V(1).Info("Adding finalizer to RRset")
		controllerutil.AddFinalizer(rrset, METRICS_FINALIZER_NAME)
		lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
		if err := r.Update(ctx, rrset); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	original := rrset.DeepCopy()
	// Ensure we update the status in case of early return
	defer func() {
		rrset.SetSyncStatus(getRRsetName(rrset))
		updateRrsetsMetrics(getRRsetName(rrset), rrset)
		if err := r.Status().Patch(ctx, rrset, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch RRSet status")
		}
	}()

	exists, err := grr.alreadyExists(ctx, rrset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to determine if RRset is duplicated: %w", err)
	}
	if exists {
		rrset.SetDuplicated()
		// updateRrsetsMetrics(getRRsetName(rrset), rrset)
		return ctrl.Result{}, nil
	}

	// Zone
	var zone dnsv1alpha2.GenericZone
	switch rrset.Spec.ZoneRef.Kind {
	//nolint:goconst
	case "Zone":
		zone = &dnsv1alpha2.Zone{}
	//nolint:goconst
	case "ClusterZone":
		zone = &dnsv1alpha2.ClusterZone{}
	}
	log.V(1).Info("Getting associated Zone", "Zone.Name", rrset.Spec.ZoneRef.Name, "Zone.Kind", rrset.Spec.ZoneRef.Kind)
	err = r.Get(ctx, client.ObjectKey{Namespace: rrset.Namespace, Name: rrset.Spec.ZoneRef.Name}, zone)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Zone not found, remove finalizer and requeue
			log.V(1).Info("Zone not found", "Zone.Name", rrset.Spec.ZoneRef.Name)
			// actionOnFinalizer := false
			// if controllerutil.ContainsFinalizer(rrset, RESOURCES_FINALIZER_NAME) {
			// 	log.V(1).Info("Removing resources finalizer from RRset")
			// 	controllerutil.RemoveFinalizer(rrset, RESOURCES_FINALIZER_NAME)
			// 	actionOnFinalizer = true
			// }
			// if isDeleted && controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
			// 	log.V(1).Info("Removing metrics finalizer from RRset")
			// 	controllerutil.RemoveFinalizer(rrset, METRICS_FINALIZER_NAME)
			// 	// Remove resource metrics
			// 	removeRrsetMetrics(rrset)
			// 	actionOnFinalizer = true
			// }
			// if actionOnFinalizer {
			// 	if err := r.Update(ctx, rrset); err != nil {
			// 		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			// 	}
			// }

			// If RRset is under deletion, no need to update its status
			//			if !isDeleted {
			log.V(1).Info("Setting missing zone for RRset", "RRset.Name", rrset.Name)
			rrset.SetMissingZone()
			// updateRrsetsMetrics(getRRsetName(rrset), rrset)
			//			}

			// Race condition when creating Zone+RRset at the same time
			// RRset is not created because Zone is not created yet
			// Requeue after few seconds
			log.V(1).Info("Requeuing RRset", "RequeueAfter", 2*time.Second)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		} else {
			rrset.SetZoneNotAvailable(zone.GetName())
			return ctrl.Result{}, fmt.Errorf("failed to get zone: %w", err)
		}
	}

	// Set OwnerReference as soon as the Zone is known, so that RRsets in a
	// Failed status are also owned (and garbage-collected) by their Zone
	err = ctrl.SetControllerReference(zone, rrset, r.Scheme)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set owner reference: %w", err)
	}
	if err := r.Update(ctx, rrset); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// If a Zone/ClusterZone exists but is in Failed Status
	zoneIsInFailedStatus := (zone.GetStatus().SyncStatus != nil && *zone.GetStatus().SyncStatus != dnsv1alpha2.SYNCED_STATUS)
	if zoneIsInFailedStatus {
		log.V(1).Info("Zone is in failed status, setting zone not available")
		rrset.SetZoneNotAvailable(zone.GetName())
		// Update metrics
		// updateRrsetsMetrics(getRRsetName(rrset), rrset)

		// if isDeleted {
		// 	log.V(1).Info("RRset is deleted, removing metrics finalizer")
		// 	if controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
		// 		controllerutil.RemoveFinalizer(rrset, METRICS_FINALIZER_NAME)
		// 		// Remove resource metrics
		// 		removeRrsetMetrics(rrset)
		// 		if err := r.Update(ctx, rrset); err != nil {
		// 			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		// 		}
		// 	}
		// }

		return ctrl.Result{}, nil
	}

	log.V(1).Info("RRset is validated status")
	rrset.SetValidated()

	err = grr.reconcileRRset(ctx, rrset, zone, isModified, isDeleted, lastUpdateTime)
	if err != nil {
		if apierrors.IsConflict(err) {
			log.Info("Conflict on RRSet owner reference, retrying")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to reconcile RRSet: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RRsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use indexer to ensure that only one RRset exists for DNS entry
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &dnsv1alpha2.RRset{}, "RRset.Entry.Name", func(rawObj client.Object) []string {
		// grab the RRset object, extract its name...
		var RRsetName string
		if rawObj.(*dnsv1alpha2.RRset).Status.SyncStatus == nil || *rawObj.(*dnsv1alpha2.RRset).Status.SyncStatus == dnsv1alpha2.SYNCED_STATUS {
			RRsetName = getRRsetName(rawObj.(*dnsv1alpha2.RRset)) + "/" + rawObj.(*dnsv1alpha2.RRset).Spec.Type
		}
		return []string{RRsetName}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha2.RRset{}).
		Complete(r)
}
