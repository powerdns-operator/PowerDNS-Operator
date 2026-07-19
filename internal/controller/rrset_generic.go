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
	"slices"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GenericRRsetReconciler struct {
	client.Client
	log        logr.Logger
	PDNSClient PdnsClienter
	scheme     *runtime.Scheme
}

//nolint:unparam // Always return ctrl.Result{} is ok
func (grr *GenericRRsetReconciler) deleteRRset(ctx context.Context, gr dnsv1alpha2.GenericRRset) error {
	finalizerRemoved := false
	if controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
		// our finalizer is present, so lets handle any external dependency
		if err := grr.deleteRrsetExternalResources(ctx, gr, gr.GetDomain()); err != nil {
			// if fail to delete the external resource, return with error
			// so that it can be retried
			return fmt.Errorf("failed to delete RRset external resources: %w", err)
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
		if err := grr.Update(ctx, gr); err != nil {
			return fmt.Errorf("failed to remove finalizer on RRset: %w", err)
		}
	}

	// Stop reconciliation as the item is being deleted
	return nil

}

func (grr *GenericRRsetReconciler) reconcileRRset(ctx context.Context, gr dnsv1alpha2.GenericRRset, zone dnsv1alpha2.GenericZone, isModified bool, isDeleted bool, lastUpdateTime *metav1.Time) error {
	log := grr.log.WithValues("rrset.name", gr.GetName(), "rrset.namespace", gr.GetNamespace(), "zone.name", zone.GetName(), "zone.namespace", zone.GetNamespace())
	isInFailedStatus := (gr.GetStatus().SyncStatus != nil && *gr.GetStatus().SyncStatus != dnsv1alpha2.SYNCED_STATUS)
	log.V(1).Info("RRset situation", "isModified", isModified, "isDeleted", isDeleted, "lastUpdateTime", lastUpdateTime, "isInFailedStatus", isInFailedStatus)

	// Get zone
	log.V(1).Info("Getting RRset external resources")
	rrsetRes, err := grr.getRRsetExternalResources(ctx, gr.GetDomain(), gr)
	if err != nil {
		log.V(1).Error(err, "Failed to get zone external resources")
		switch pdnsErrorStatusCode(err) {
		case UNPROCESSABLE_ERROR_CODE:
			gr.SetUnprocessable("Processed", err)
			return nil
		case BAD_REQUEST_ERROR_CODE:
			gr.SetBadRequest("Processed", err)
			return nil
		default:
			gr.SetSynchronizationFailed("Processed", err)
			return fmt.Errorf("failed to get zone external resources: %w", err)
		}
	}

	// Let's add the finalizer and update the object.
	if !controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
		log.V(1).Info("Adding resources finalizer to RRset")
		controllerutil.AddFinalizer(gr, RESOURCES_FINALIZER_NAME)
		if err := grr.Update(ctx, gr); err != nil {
			return fmt.Errorf("failed to add finalizer on RRset: %w", err)
		}
	}

	// After the finalizer update at first time, the status is reseted
	// so we need to set the validated status again
	gr.SetValidated()

	if rrsetRes.Name == nil {
		// CREATE RRSET
		log.V(1).Info("External resource does not exist, creating it")
		err := grr.createRrsetExternalResources(ctx, gr, gr.GetDomain())
		if err != nil {
			switch pdnsErrorStatusCode(err) {
			case UNPROCESSABLE_ERROR_CODE:
				gr.SetUnprocessable("Processed", err)
				return nil
			case BAD_REQUEST_ERROR_CODE:
				gr.SetBadRequest("Processed", err)
				return nil
			default:
				gr.SetSynchronizationFailed("Processed", err)
				return fmt.Errorf("failed to create external resources: %w", err)
			}
		}
		log.V(1).Info("External resource created")
	} else {
		// UPDATE RRSET
		log.V(1).Info("External resource exists, updating it")
		identical := rrsetIsIdenticalToExternalRRset(gr, *rrsetRes)
		if !identical {
			err := grr.updateRrsetExternalResources(ctx, gr, gr.GetDomain())
			if err != nil {
				switch pdnsErrorStatusCode(err) {
				case UNPROCESSABLE_ERROR_CODE:
					gr.SetUnprocessable("Processed", err)
					return nil
				case BAD_REQUEST_ERROR_CODE:
					gr.SetBadRequest("Processed", err)
					return nil
				default:
					gr.SetSynchronizationFailed("Processed", err)
					return fmt.Errorf("failed to update external resources: %w", err)
				}
			}
			log.V(1).Info("External resource updated")
		}
	}
	gr.SetProcessed()

	// This Patch is very important:
	// When an update on RRSet is applied, a reconcile event is triggered on Zone
	// But, sometimes, Zone reonciliation finish before RRSet update is applied
	// In that case, the Serial in Zone Status is false
	// This update permits triggering a new event after RRSet update applied
	name := getRRsetName(gr)
	gr.SetAvailable(name)

	// Metrics calculation
	// updateRrsetsMetrics(getRRsetName(gr), gr)

	return nil
}

func (grr *GenericRRsetReconciler) getRRsetExternalResources(ctx context.Context, domain string, rrset dnsv1alpha2.GenericRRset) (*powerdns.RRset, error) {
	log := grr.log.WithValues("kind", rrset.GetKind(), "name", rrset.GetName(), "namespace", rrset.GetNamespace())
	name := getRRsetName(rrset)
	rrType := powerdns.RRType(rrset.GetSpec().Type)
	rrsets, err := grr.PDNSClient.Records.Get(ctx, domain, name, &rrType)
	if err != nil && pdnsErrorStatusCode(err) != NOT_FOUND_ERROR_CODE {
		return nil, fmt.Errorf("PowerDNS API returned an error while getting external resource: %w", err)
	}
	log.V(1).WithValues("rrsets", rrsets).Info("External resource found")

	var result powerdns.RRset
	if len(rrsets) > 0 {
		result = rrsets[0]
	}

	return &result, nil
}

func (grr *GenericRRsetReconciler) deleteRrsetExternalResources(ctx context.Context, rrset dnsv1alpha2.GenericRRset, domain string) error {
	log := grr.log.WithValues("kind", rrset.GetKind(), "name", rrset.GetName(), "namespace", rrset.GetNamespace())
	err := grr.PDNSClient.Records.Delete(ctx, domain, getRRsetName(rrset), powerdns.RRType(rrset.GetSpec().Type))
	if err != nil {
		switch pdnsErrorStatusCode(err) {
		case NOT_FOUND_ERROR_CODE:
			// RRset or its zone may have already been deleted and it is not an error
		case UNPROCESSABLE_ERROR_CODE:
			// PowerDNS cannot process the RRset identity (e.g. invalid type),
			// so it could never have stored it: there is nothing to delete and
			// the finalizer must not stay stuck on it
		default:
			return fmt.Errorf("PowerDNS API returned an error while deleting external resource: %w", err)
		}
	}
	log.V(1).Info("External resource deleted")
	return nil
}

func (grr *GenericRRsetReconciler) createRrsetExternalResources(ctx context.Context, rrset dnsv1alpha2.GenericRRset, domain string) error {
	log := grr.log.WithValues("kind", rrset.GetKind(), "name", rrset.GetName(), "namespace", rrset.GetNamespace())
	name := getRRsetName(rrset)
	rrType := powerdns.RRType(rrset.GetSpec().Type)

	// Create
	operatorAccount := "powerdns-operator"
	comments := func(*powerdns.RRset) {}
	if rrset.GetSpec().Comment != nil {
		comments = powerdns.WithComments(powerdns.Comment{Content: rrset.GetSpec().Comment, Account: &operatorAccount})
	}
	err := grr.PDNSClient.Records.Change(ctx, domain, name, rrType, rrset.GetSpec().TTL, rrset.GetSpec().Records, comments)
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while creating external resource: %w", err)
	}

	log.V(1).Info("External resource created")
	return nil
}

func (grr *GenericRRsetReconciler) updateRrsetExternalResources(ctx context.Context, rrset dnsv1alpha2.GenericRRset, domain string) error {
	log := grr.log.WithValues("kind", rrset.GetKind(), "name", rrset.GetName(), "namespace", rrset.GetNamespace())
	name := getRRsetName(rrset)
	rrType := powerdns.RRType(rrset.GetSpec().Type)
	// rrType := powerdns.RRType(rrset.GetSpec().Type)
	// // Looking for a record with same Name and Type
	// records, err := grr.PDNSClient.Records.Get(ctx, domain, name, &rrType)
	// if err != nil && !apierrors.IsNotFound(err) {
	// 	return false, fmt.Errorf("PowerDNS API returned an error while getting external resource: %w", err)
	// }
	// var filteredRecord powerdns.RRset
	// if len(records) > 0 {
	// 	filteredRecord = records[0]
	// }

	// Create or Update
	operatorAccount := "powerdns-operator"
	comments := func(*powerdns.RRset) {}
	if rrset.GetSpec().Comment != nil {
		comments = powerdns.WithComments(powerdns.Comment{Content: rrset.GetSpec().Comment, Account: &operatorAccount})
	}
	err := grr.PDNSClient.Records.Change(ctx, domain, name, rrType, rrset.GetSpec().TTL, rrset.GetSpec().Records, comments)
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while updating external resource: %w", err)
	}

	log.V(1).Info("External resource updated")
	return nil
}

func (grr *GenericRRsetReconciler) alreadyExists(ctx context.Context, gr dnsv1alpha2.GenericRRset) (bool, error) {
	log := grr.log.WithValues("kind", gr.GetKind(), "name", gr.GetName(), "namespace", gr.GetNamespace())

	// If a RRset already exists with the same DNS name:
	// * Stop reconciliation
	// * Append a Failed Status on RRset
	var existingRRsets dnsv1alpha2.RRsetList
	if err := grr.List(ctx, &existingRRsets, client.MatchingFields{"RRset.Entry.Name": getRRsetName(gr) + "/" + gr.GetSpec().Type}); err != nil {
		return false, fmt.Errorf("error while listing RRsets related to the DNS Name: %w", err)

	}
	var existingClusterRRsets dnsv1alpha2.ClusterRRsetList
	if err := grr.List(ctx, &existingClusterRRsets, client.MatchingFields{"ClusterRRset.Entry.Name": getRRsetName(gr) + "/" + gr.GetSpec().Type}); err != nil {
		return false, fmt.Errorf("error while listing ClusterRRsets related to the DNS Name: %w", err)
	}

	// Remove current Zone or ClusterZone from the lists
	switch gr.GetKind() {
	case "RRset":
		existingRRsets.Items = slices.DeleteFunc(existingRRsets.Items, func(item dnsv1alpha2.RRset) bool {
			return item.GetName() == gr.GetName() && item.GetNamespace() == gr.GetNamespace()
		})
	case "ClusterRRset":
		existingClusterRRsets.Items = slices.DeleteFunc(existingClusterRRsets.Items, func(item dnsv1alpha2.ClusterRRset) bool {
			return item.GetName() == gr.GetName()
		})
	}

	if len(existingRRsets.Items) >= 1 || len(existingClusterRRsets.Items) >= 1 {
		log.V(1).WithValues("existingRRsets", existingRRsets.Items, "existingClusterRRsets", existingClusterRRsets.Items).Info("RRset is duplicated")

		return true, nil
	}
	return false, nil
}
