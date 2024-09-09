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

	"github.com/joeig/go-powerdns/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1alpha1 "gitlab.tech.orange/parent-factory/hzf-tools/powerdns-operator/api/v1alpha1"
)

// RRsetReconciler reconciles a RRset object
type RRsetReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=rrsets/finalizers,verbs=update

func (r *RRsetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile RRset", "Zone.RRset.Name", req.Name)
	// RRset
	rrset := &dnsv1alpha1.RRset{}
	err := r.Get(ctx, req.NamespacedName, rrset)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Retrieve lastUpdateTime if defined, otherwise Now()
	lastUpdateTime := &metav1.Time{Time: time.Now().UTC()}
	if rrset.Status.LastUpdateTime != nil {
		lastUpdateTime = rrset.Status.LastUpdateTime
	}

	// Zone
	zone := &dnsv1alpha1.Zone{}
	err = r.Get(ctx, client.ObjectKey{Namespace: rrset.Namespace, Name: rrset.Spec.ZoneRef.Name}, zone)
	if err != nil {
		if errors.IsNotFound(err) {
			// Zone not found, remove finalizer and requeue
			if controllerutil.ContainsFinalizer(rrset, FINALIZER_NAME) {
				controllerutil.RemoveFinalizer(rrset, FINALIZER_NAME)
				if err := r.Update(ctx, rrset); err != nil {
					log.Error(err, "Failed to remove finalizer")
					return ctrl.Result{}, err
				}
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

	// examine DeletionTimestamp to determine if object is under deletion
	if rrset.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(rrset, FINALIZER_NAME) {
			controllerutil.AddFinalizer(rrset, FINALIZER_NAME)
			lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
			if err := r.Update(ctx, rrset); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(rrset, FINALIZER_NAME) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(ctx, zone, rrset); err != nil {
				// if fail to delete the external resource, return with error
				// so that it can be retried
				log.Error(err, "Failed to delete external resources")
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(rrset, FINALIZER_NAME)
			if err := r.Update(ctx, rrset); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
			//nolint:ineffassign
			lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Create or Update
	changed, err := r.createOrUpdateExternalResources(ctx, zone, rrset)
	if err != nil {
		log.Error(err, "Failed to create or update external resources")
		return ctrl.Result{}, err
	}
	if changed {
		lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
	}

	// Set OwnerReference
	if err := r.ownObject(ctx, zone, rrset); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict on RRSet owner reference, retrying")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// This Patch is very important:
	// When an update on RRSet is applied, a reconcile event is triggered on Zone
	// But, sometimes, Zone reonciliation finish before RRSet update is applied
	// In that case, the Serial in Zone Status is false
	// This update permits triggering a new event after RRSet update applied
	original := rrset.DeepCopy()
	rrset.Status.LastUpdateTime = lastUpdateTime
	if err := r.Status().Patch(ctx, rrset, client.MergeFrom(original)); err != nil {
		log.Error(err, "unable to patch RRSet status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RRsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha1.RRset{}).
		Complete(r)
}

func (r *RRsetReconciler) deleteExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, rrset *dnsv1alpha1.RRset) error {
	log := log.FromContext(ctx)

	// Delete
	err := r.PDNSClient.Records.Delete(ctx, zone.ObjectMeta.Name, rrset.ObjectMeta.Name, powerdns.RRType(rrset.Spec.Type))
	if err != nil {
		log.Error(err, "Failed to delete record")
		return err
	}

	return nil
}

// createOrUpdateExternalResources create or update the input resource if necessary, and return True if changed
func (r *RRsetReconciler) createOrUpdateExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, rrset *dnsv1alpha1.RRset) (bool, error) {
	log := log.FromContext(ctx)

	rrType := powerdns.RRType(rrset.Spec.Type)
	// Looking for a record with same Name and Type
	records, err := r.PDNSClient.Records.Get(ctx, zone.ObjectMeta.Name, rrset.ObjectMeta.Name, &rrType)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Failed to get external rrsets for the type")
		return false, err
	}
	// An issue exist on GET API Calls, comments for another RRSet are included although we filter
	// See https://github.com/PowerDNS/pdns/issues/14539
	// See https://github.com/PowerDNS/pdns/pull/14045
	var filteredRecord powerdns.RRset
	for _, fr := range records {
		if *fr.Name == makeCanonical(rrset.ObjectMeta.Name) {
			filteredRecord = fr
			break
		}
	}
	if filteredRecord.Name != nil && rrsetIsIdenticalToExternalRRset(rrset, filteredRecord) {
		return false, nil
	}

	// Create or Update
	operatorAccount := "powerdns-operator"
	comments := func(*powerdns.RRset) {}
	if rrset.Spec.Comment != nil {
		comments = powerdns.WithComments(powerdns.Comment{Content: rrset.Spec.Comment, Account: &operatorAccount})
	}
	err = r.PDNSClient.Records.Change(ctx, zone.ObjectMeta.Name, rrset.ObjectMeta.Name, rrType, rrset.Spec.TTL, rrset.Spec.Records, comments)
	if err != nil {
		log.Error(err, "Failed to create record")
		return false, err
	}

	return true, nil
}

// ownObject set the owner reference on RRset
func (r *RRsetReconciler) ownObject(ctx context.Context, zone *dnsv1alpha1.Zone, rrset *dnsv1alpha1.RRset) error {
	log := log.FromContext(ctx)

	err := ctrl.SetControllerReference(zone, rrset, r.Scheme)
	if err != nil {
		log.Error(err, "Failed to set owner reference. Is there already a controller managing this object?")
		return err
	}
	return r.Update(ctx, rrset)
}
