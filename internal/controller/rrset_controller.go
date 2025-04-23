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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/api/errors"
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
	RrsetReasonZoneNotAvailable      = "ZoneNotAvailable"
	RrsetReasonSynchronizationFailed = "SynchronizationFailed"
	RrsetReasonDuplicated            = "RrsetDuplicated"
	RrsetReasonSynced                = "RrsetSynced"
	RrsetMessageDuplicated           = "Already existing RRset with the same FQDN"
	RrsetMessageSyncSucceeded        = "RRset synced with PowerDNS instance"
	RrsetMessageNonExistentZone      = "non-existent zone:"
	RrsetMessageUnavailableZone      = "unavailable zone:"
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
	log.Info("Reconcile RRset", "Zone.RRset.Name", req.Name)

	// RRset
	rrset := &dnsv1alpha2.RRset{}
	err := r.Get(ctx, req.NamespacedName, rrset)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize variable to represent RRset situation
	isModified := rrset.Status.ObservedGeneration != nil && *rrset.Status.ObservedGeneration != rrset.GetGeneration()
	isDeleted := !rrset.DeletionTimestamp.IsZero()
	lastUpdateTime := &metav1.Time{Time: time.Now().UTC()}
	if rrset.Status.LastUpdateTime != nil {
		lastUpdateTime = rrset.Status.LastUpdateTime
	}

	// Position metrics finalizer as soon as possible
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
			controllerutil.AddFinalizer(rrset, METRICS_FINALIZER_NAME)
			lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
			if err := r.Update(ctx, rrset); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	// When updating a RRset, if 'Status' is not changed, 'LastTransitionTime' will not be updated
	// So we delete condition to force new 'LastTransitionTime'
	original := rrset.DeepCopy()
	if !isDeleted && isModified {
		meta.RemoveStatusCondition(&rrset.Status.Conditions, "Available")
		if err := r.Status().Patch(ctx, rrset, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch RRSet status")
			return ctrl.Result{}, err
		}
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
	err = r.Get(ctx, client.ObjectKey{Namespace: rrset.Namespace, Name: rrset.Spec.ZoneRef.Name}, zone)
	if err != nil {
		if errors.IsNotFound(err) {
			// Zone not found, remove finalizer and requeue
			actionOnFinalizer := false
			if controllerutil.ContainsFinalizer(rrset, RESOURCES_FINALIZER_NAME) {
				controllerutil.RemoveFinalizer(rrset, RESOURCES_FINALIZER_NAME)
				actionOnFinalizer = true
			}
			if isDeleted && controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
				controllerutil.RemoveFinalizer(rrset, METRICS_FINALIZER_NAME)
				// Remove resource metrics
				removeRrsetMetrics(rrset)
				actionOnFinalizer = true
			}
			if actionOnFinalizer {
				if err := r.Update(ctx, rrset); err != nil {
					log.Error(err, "Failed to remove finalizer")
					return ctrl.Result{}, err
				}
			}

			// If RRset is under deletion, no need to update its status
			if !isDeleted {
				original = rrset.DeepCopy()
				rrset.Status.SyncStatus = ptr.To(PENDING_STATUS)
				rrset.Status.ObservedGeneration = &rrset.Generation
				meta.SetStatusCondition(&rrset.Status.Conditions, metav1.Condition{
					Type:               "Available",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(time.Now().UTC()),
					Reason:             RrsetReasonZoneNotAvailable,
					Message:            RrsetMessageNonExistentZone + err.Error(),
				})
				if err := r.Status().Patch(ctx, rrset, client.MergeFrom(original)); err != nil {
					log.Error(err, "unable to patch RRSet status")
					return ctrl.Result{}, err
				}
				updateRrsetsMetrics(getRRsetName(rrset), rrset)
			}

			// Race condition when creating Zone+RRset at the same time
			// RRset is not created because Zone is not created yet
			// Requeue after few seconds
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		} else {
			log.Error(err, "Failed to get zone")
			return ctrl.Result{}, err
		}
	}
	// If a Zone/ClusterZone exists but is in Failed Status
	zoneIsInFailedStatus := (zone.GetStatus().SyncStatus != nil && *zone.GetStatus().SyncStatus == FAILED_STATUS)
	if zoneIsInFailedStatus {
		original = rrset.DeepCopy()
		rrset.Status.SyncStatus = ptr.To(FAILED_STATUS)
		rrset.Status.ObservedGeneration = &rrset.Generation
		meta.SetStatusCondition(&rrset.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(time.Now().UTC()),
			Reason:             RrsetReasonZoneNotAvailable,
			Message:            RrsetMessageUnavailableZone + zone.GetName(),
		})
		if err := r.Status().Patch(ctx, rrset, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch RRSet status")
			return ctrl.Result{}, err
		}

		// Update metrics
		updateRrsetsMetrics(getRRsetName(rrset), rrset)

		if isDeleted {
			if controllerutil.ContainsFinalizer(rrset, METRICS_FINALIZER_NAME) {
				controllerutil.RemoveFinalizer(rrset, METRICS_FINALIZER_NAME)
				// Remove resource metrics
				removeRrsetMetrics(rrset)
				if err := r.Update(ctx, rrset); err != nil {
					log.Error(err, "Failed to remove finalizer")
					return ctrl.Result{}, err
				}
			}
		}

		return ctrl.Result{}, nil
	}

	return rrsetReconcile(ctx, rrset, zone, isModified, isDeleted, lastUpdateTime, r.Scheme, r.Client, r.PDNSClient, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RRsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use indexer to ensure that only one RRset exists for DNS entry
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &dnsv1alpha2.RRset{}, "RRset.Entry.Name", func(rawObj client.Object) []string {
		// grab the RRset object, extract its name...
		var RRsetName string
		if rawObj.(*dnsv1alpha2.RRset).Status.SyncStatus == nil || *rawObj.(*dnsv1alpha2.RRset).Status.SyncStatus == SUCCEEDED_STATUS {
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
