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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha1 "github.com/orange-opensource/powerdns-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func zoneReconcile(ctx context.Context, zone *dnsv1alpha1.Zone, isDeleted bool, cl client.Client, PDNSClient PdnsClienter, log logr.Logger) (ctrl.Result, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(zone, FINALIZER_NAME) {
			controllerutil.AddFinalizer(zone, FINALIZER_NAME)
			if err := cl.Update(ctx, zone); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(zone, FINALIZER_NAME) {
			// our finalizer is present, so lets handle any external dependency
			if err := deleteZoneExternalResources(ctx, zone, PDNSClient, log); err != nil {
				// if fail to delete the external resource, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(zone, FINALIZER_NAME)
			if err := cl.Update(ctx, zone); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Get zone
	zoneRes, err := getZoneExternalResources(ctx, zone.ObjectMeta.Name, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	syncStatus, conditionMessage, conditionReason, conditionStatus, err := zoneExternalResourcesReconcile(ctx, zoneRes, zone, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	if syncStatus == nil {
		syncStatus = ptr.To(SUCCEEDED_STATUS)
	}

	// Update ZoneStatus
	zoneRes, err = getZoneExternalResources(ctx, zone.ObjectMeta.Name, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = patchZoneStatus(ctx, zone, zoneRes, syncStatus, cl, metav1.Condition{
		Type:               "Available",
		LastTransitionTime: metav1.NewTime(time.Now().UTC()),
		Status:             conditionStatus,
		Reason:             conditionReason,
		Message:            conditionMessage,
	})
	if err != nil {
		if errors.IsConflict(err) {
			log.Info("Object has been modified, forcing a new reconciliation")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func getZoneExternalResources(ctx context.Context, domain string, PDNSClient PdnsClienter, log logr.Logger) (*powerdns.Zone, error) {
	zoneRes, err := PDNSClient.Zones.Get(ctx, domain)
	if err != nil {
		if err.Error() != ZONE_NOT_FOUND_MSG {
			log.Error(err, "Failed to get zone")
			return nil, err
		}
	}
	return zoneRes, nil
}

func createZoneExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, PDNSClient PdnsClienter, log logr.Logger) error {
	// Make Nameservers canonical
	for i, ns := range zone.Spec.Nameservers {
		zone.Spec.Nameservers[i] = makeCanonical(ns)
	}

	// Make Catalog canonical
	var catalog *string
	if zone.Spec.Catalog != nil {
		catalog = ptr.To(makeCanonical(ptr.Deref(zone.Spec.Catalog, "")))
	}

	z := powerdns.Zone{
		ID:          &zone.Name,
		Name:        &zone.Name,
		Kind:        powerdns.ZoneKindPtr(powerdns.ZoneKind(zone.Spec.Kind)),
		DNSsec:      ptr.To(false),
		SOAEditAPI:  zone.Spec.SOAEditAPI,
		Nameservers: zone.Spec.Nameservers,
		Catalog:     catalog,
	}

	_, err := PDNSClient.Zones.Add(ctx, &z)
	if err != nil {
		log.Error(err, "Failed to create zone")
		return err
	}

	return nil
}

func updateZoneExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, PDNSClient PdnsClienter, log logr.Logger) error {
	zoneKind := powerdns.ZoneKind(zone.Spec.Kind)

	// Make Catalog canonical
	var catalog *string
	if zone.Spec.Catalog != nil {
		catalog = ptr.To(makeCanonical(ptr.Deref(zone.Spec.Catalog, "")))
	}

	err := PDNSClient.Zones.Change(ctx, zone.ObjectMeta.Name, &powerdns.Zone{
		Name:        &zone.ObjectMeta.Name,
		Kind:        &zoneKind,
		Nameservers: zone.Spec.Nameservers,
		Catalog:     catalog,
		SOAEditAPI:  zone.Spec.SOAEditAPI,
	})
	if err != nil {
		log.Error(err, "Failed to update zone")
		return err
	}
	return nil
}

func updateNsOnZoneExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, ttl uint32, PDNSClient PdnsClienter, log logr.Logger) error {
	nameserversCanonical := []string{}
	for _, n := range zone.Spec.Nameservers {
		nameserversCanonical = append(nameserversCanonical, makeCanonical(n))
	}

	err := PDNSClient.Records.Change(ctx, makeCanonical(zone.ObjectMeta.Name), makeCanonical(zone.ObjectMeta.Name), powerdns.RRTypeNS, ttl, nameserversCanonical)
	if err != nil {
		log.Error(err, "Failed to update NS in zone")
		return err
	}
	return nil
}

func deleteZoneExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, PDNSClient PdnsClienter, log logr.Logger) error {
	err := PDNSClient.Zones.Delete(ctx, zone.ObjectMeta.Name)
	// Zone may have already been deleted and it is not an error
	if err != nil && err.Error() != ZONE_NOT_FOUND_MSG {
		log.Error(err, "Failed to delete zone")
		return err
	}
	return nil
}

func zoneExternalResourcesReconcile(ctx context.Context, zoneRes *powerdns.Zone, zone *dnsv1alpha1.Zone, PDNSClient PdnsClienter, log logr.Logger) (*string, string, string, metav1.ConditionStatus, error) {
	// Initialization
	var syncStatus *string
	conditionStatus := metav1.ConditionTrue
	conditionReason := ZoneReasonSynced
	conditionMessage := ZoneMessageSyncSucceeded

	if zoneRes.Name == nil {
		// If Zone does not exist, create it
		err := createZoneExternalResources(ctx, zone, PDNSClient, log)
		if err != nil {
			log.Error(err, "Failed to create external resources")
			syncStatus = ptr.To(FAILED_STATUS)
			conditionStatus = metav1.ConditionFalse
			conditionReason = ZoneReasonSynchronizationFailed
			conditionMessage = err.Error()
		}
	} else {
		// If Zone exists, compare content and update it if necessary
		ns, err := PDNSClient.Records.Get(ctx, zone.ObjectMeta.Name, zone.ObjectMeta.Name, ptr.To(powerdns.RRTypeNS))
		if err != nil {
			return nil, "", "", "", err
		}

		// An issue exist on GET API Calls, comments for another RRSet are included although we filter
		// See https://github.com/PowerDNS/pdns/issues/14539
		// See https://github.com/PowerDNS/pdns/pull/14045
		var filteredRRset powerdns.RRset
		for _, rr := range ns {
			if *rr.Name == makeCanonical(zone.ObjectMeta.Name) && *rr.Type == powerdns.RRTypeNS {
				filteredRRset = rr
			}
		}
		var nameservers []string
		for _, n := range filteredRRset.Records {
			nameservers = append(nameservers, strings.TrimSuffix(*n.Content, "."))
		}

		// Workflow is different on update types:
		// Nameservers changes  => patch RRSet
		// Other changes        => patch Zone
		zoneIdentical, nsIdentical := zoneIsIdenticalToExternalZone(zone, zoneRes, nameservers)

		// Nameservers changes
		if !nsIdentical {
			ttl := ptr.To(DEFAULT_TTL_FOR_NS_RECORDS)
			if filteredRRset.TTL != nil {
				ttl = filteredRRset.TTL
			}
			err := updateNsOnZoneExternalResources(ctx, zone, *ttl, PDNSClient, log)
			if err != nil {
				syncStatus = ptr.To(FAILED_STATUS)
				conditionStatus = metav1.ConditionFalse
				conditionReason = ZoneReasonNSSynchronizationFailed
				conditionMessage = err.Error()
			}
		}
		// Other changes
		if !zoneIdentical {
			err := updateZoneExternalResources(ctx, zone, PDNSClient, log)
			if err != nil {
				syncStatus = ptr.To(FAILED_STATUS)
				conditionStatus = metav1.ConditionFalse
				conditionReason = ZoneReasonSynchronizationFailed
				conditionMessage = err.Error()
			}
		}
	}
	return syncStatus, conditionMessage, conditionReason, conditionStatus, nil
}

func patchZoneStatus(ctx context.Context, zone *dnsv1alpha1.Zone, zoneRes *powerdns.Zone, status *string, cl client.Client, condition metav1.Condition) error {
	original := zone.DeepCopy()

	kind := string(ptr.Deref(zoneRes.Kind, ""))
	conditions := zone.Status.Conditions
	meta.SetStatusCondition(&conditions, condition)
	zone.Status = dnsv1alpha1.ZoneStatus{
		ID:                 zoneRes.ID,
		Name:               zoneRes.Name,
		Kind:               &kind,
		Serial:             zoneRes.Serial,
		NotifiedSerial:     zoneRes.NotifiedSerial,
		EditedSerial:       zoneRes.EditedSerial,
		Masters:            zoneRes.Masters,
		DNSsec:             zoneRes.DNSsec,
		SyncStatus:         status,
		Catalog:            zoneRes.Catalog,
		ObservedGeneration: ptr.To(zone.GetGeneration()),
		Conditions:         conditions,
	}
	return cl.Status().Patch(ctx, zone, client.MergeFrom(original))
}
