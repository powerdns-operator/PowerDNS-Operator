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

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GenericRRsetReconciler struct {
	client.Client
	log        logr.Logger
	PDNSClient PdnsClienter
	scheme     *runtime.Scheme
}

func (grr *GenericRRsetReconciler) rrsetReconcile(ctx context.Context, gr dnsv1alpha2.GenericRRset, zone dnsv1alpha2.GenericZone, isModified bool, isDeleted bool, lastUpdateTime *metav1.Time) (ctrl.Result, error) {
	cl := grr.Client
	log := grr.log
	scheme := grr.scheme
	isInFailedStatus := (gr.GetStatus().SyncStatus != nil && *gr.GetStatus().SyncStatus == dnsv1alpha2.FAILED_STATUS)
	log.V(1).Info("RRset situation", "isModified", isModified, "isDeleted", isDeleted, "lastUpdateTime", lastUpdateTime, "isInFailedStatus", isInFailedStatus)

	// examine DeletionTimestamp to determine if object is under deletion
	if !isDeleted {
		log.V(1).Info("RRset not deleted", "RRset.Name", gr.GetName())
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
			log.V(1).Info("Adding resources finalizer to RRset")
			controllerutil.AddFinalizer(gr, RESOURCES_FINALIZER_NAME)
			lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
			if err := cl.Update(ctx, gr); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		log.V(1).Info("RRset is deleted", "RRset.Name", gr.GetName())
		// The object is being deleted
		finalizerRemoved := false
		if controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
			log.V(1).Info("Removing resources finalizer from RRset")
			// our finalizer is present, so lets handle any external dependency
			if err := grr.deleteRrsetExternalResources(ctx, gr, zone); err != nil {
				// if fail to delete the external resource, return with error
				// so that it can be retried
				log.Error(err, "Failed to delete external resources")
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list.
			controllerutil.RemoveFinalizer(gr, RESOURCES_FINALIZER_NAME)
			finalizerRemoved = true
		}
		if controllerutil.ContainsFinalizer(gr, METRICS_FINALIZER_NAME) {
			// Remove resource metrics and finalizer
			removeRrsetMetrics(gr)
			controllerutil.RemoveFinalizer(gr, METRICS_FINALIZER_NAME)
			finalizerRemoved = true
		}
		if finalizerRemoved {
			if err := cl.Update(ctx, gr); err != nil {
				log.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Set OwnerReference as soon as the Zone is known, so that RRsets in a
	// Failed status are also owned (and garbage-collected) by their Zone
	if err := ownObject(ctx, zone, gr, scheme, cl, log); err != nil {
		if apierrors.IsConflict(err) {
			log.Info("Conflict on RRSet owner reference, retrying")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// We cannot exit previously (at the early moments of reconcile), because we have to allow deletion process
	if isInFailedStatus && !isModified {
		// Update resource metrics
		updateRrsetsMetrics(getRRsetName(gr), gr)
		return ctrl.Result{}, nil
	}

	// If a RRset already exists with the same DNS name:
	// * Stop reconciliation
	// * Append a Failed Status on RRset
	var existingRRsets dnsv1alpha2.RRsetList
	if err := cl.List(ctx, &existingRRsets, client.MatchingFields{"RRset.Entry.Name": getRRsetName(gr) + "/" + gr.GetSpec().Type}); err != nil {
		log.Error(err, "unable to find RRsets related to the DNS Name")
		return ctrl.Result{}, err
	}
	var existingClusterRRsets dnsv1alpha2.ClusterRRsetList
	if err := cl.List(ctx, &existingClusterRRsets, client.MatchingFields{"ClusterRRset.Entry.Name": getRRsetName(gr) + "/" + gr.GetSpec().Type}); err != nil {
		log.Error(err, "unable to find RRsets related to the DNS Name")
		return ctrl.Result{}, err
	}

	// Multiple use-cases:
	// 1 RRset (test.example.com in NS example1) + 1 RRset (test.example.com in NS example3)
	// In that case: len(existingRRsets.Items) > 1
	// 1 RRset (test.example.com in NS example1) + 1 ClusterRRset (test.example.com)
	// In that case: len(existingRRsets.Items) >= 1 AND len(existingClusterRRsets.Items) >= 1
	// 1 ClusterRRset (test.example.com) + 1 ClusterRRset (test.example.com)
	// In that case: len(existingClusterRRsets.Items) > 1
	if len(existingRRsets.Items) > 1 || (len(existingRRsets.Items) >= 1 && len(existingClusterRRsets.Items) >= 1) || len(existingClusterRRsets.Items) > 1 {
		name := getRRsetName(gr)
		gr.SetDuplicated(lastUpdateTime, name)

		// Update resource metrics
		updateRrsetsMetrics(getRRsetName(gr), gr)

		return ctrl.Result{}, fmt.Errorf("RRset already exists")
	}

	// Create or Update
	var changed bool
	var err error
	changed, err = grr.createOrUpdateRrsetExternalResources(ctx, gr, zone)
	if changed {
		lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
	}
	if err != nil {
		log.Error(err, "Failed to create or update external resources")
		gr.SetSynchronizationFailed(lastUpdateTime, err)
		updateRrsetsMetrics(getRRsetName(gr), gr)
		return ctrl.Result{}, err
	}

	// This Patch is very important:
	// When an update on RRSet is applied, a reconcile event is triggered on Zone
	// But, sometimes, Zone reonciliation finish before RRSet update is applied
	// In that case, the Serial in Zone Status is false
	// This update permits triggering a new event after RRSet update applied
	name := getRRsetName(gr)
	gr.SetAvailable(lastUpdateTime, name)

	// Metrics calculation
	updateRrsetsMetrics(getRRsetName(gr), gr)

	return ctrl.Result{}, nil
}

func (grr *GenericRRsetReconciler) deleteRrsetExternalResources(ctx context.Context, rrset dnsv1alpha2.GenericRRset, zone dnsv1alpha2.GenericZone) error {
	PDNSClient := grr.PDNSClient
	log := grr.log
	err := PDNSClient.Records.Delete(ctx, zone.GetObjectMeta().Name, getRRsetName(rrset), powerdns.RRType(rrset.GetSpec().Type))
	if err != nil {
		log.Error(err, "Failed to delete record")
		return err
	}

	return nil
}

func (grr *GenericRRsetReconciler) createOrUpdateRrsetExternalResources(ctx context.Context, rrset dnsv1alpha2.GenericRRset, zone dnsv1alpha2.GenericZone) (bool, error) {
	PDNSClient := grr.PDNSClient
	name := getRRsetName(rrset)
	rrType := powerdns.RRType(rrset.GetSpec().Type)
	// Looking for a record with same Name and Type
	records, err := PDNSClient.Records.Get(ctx, zone.GetObjectMeta().Name, name, &rrType)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	// An issue exist on GET API Calls, comments for another RRSet are included although we filter
	// See https://github.com/PowerDNS/pdns/issues/14539
	// See https://github.com/PowerDNS/pdns/pull/14045
	var filteredRecord powerdns.RRset
	for _, fr := range records {
		if *fr.Name == makeCanonical(name) {
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
	if rrset.GetSpec().Comment != nil {
		comments = powerdns.WithComments(powerdns.Comment{Content: rrset.GetSpec().Comment, Account: &operatorAccount})
	}
	err = PDNSClient.Records.Change(ctx, zone.GetObjectMeta().Name, name, rrType, rrset.GetSpec().TTL, rrset.GetSpec().Records, comments)
	if err != nil {
		return false, err
	}

	return true, nil
}
