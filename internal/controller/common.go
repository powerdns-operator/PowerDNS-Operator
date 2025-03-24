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
	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func zoneReconcile(ctx context.Context, gz dnsv1alpha2.GenericZone, isModified bool, isDeleted bool, cl client.Client, PDNSClient PdnsClienter, log logr.Logger) (ctrl.Result, error) {
	isInFailedStatus := (gz.GetStatus().SyncStatus != nil && *gz.GetStatus().SyncStatus == FAILED_STATUS)

	// examine DeletionTimestamp to determine if object is under deletion
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(gz, RESOURCES_FINALIZER_NAME) {
			controllerutil.AddFinalizer(gz, RESOURCES_FINALIZER_NAME)
			if err := cl.Update(ctx, gz); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		finalizerRemoved := false
		if controllerutil.ContainsFinalizer(gz, RESOURCES_FINALIZER_NAME) {
			// our finalizer is present, so lets handle any external dependency
			if err := deleteZoneExternalResources(ctx, gz, PDNSClient, log); err != nil {
				// if fail to delete the external resource, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(gz, RESOURCES_FINALIZER_NAME)
			finalizerRemoved = true
		}
		if controllerutil.ContainsFinalizer(gz, METRICS_FINALIZER_NAME) {
			// Remove resource metrics and finalizer
			removeZonesMetrics(gz)
			controllerutil.RemoveFinalizer(gz, METRICS_FINALIZER_NAME)
			finalizerRemoved = true
		}
		if finalizerRemoved {
			if err := cl.Update(ctx, gz); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// We cannot exit previously (at the early moments of reconcile), because we have to allow deletion process
	if isInFailedStatus && !isModified {
		// Update resource metrics
		updateZonesMetrics(gz)
		return ctrl.Result{}, nil
	}

	// If a Zone already exists with the same DNS name:
	// * Stop reconciliation
	// * Append a Failed Status on Zone
	var existingZones dnsv1alpha2.ZoneList
	if err := cl.List(ctx, &existingZones, client.MatchingFields{"Zone.Entry.Name": gz.GetName()}); err != nil {
		log.Error(err, "unable to find Zone related to the DNS Name")
		return ctrl.Result{}, err
	}
	var existingClusterZones dnsv1alpha2.ClusterZoneList
	if err := cl.List(ctx, &existingClusterZones, client.MatchingFields{"ClusterZone.Entry.Name": gz.GetName()}); err != nil {
		log.Error(err, "unable to find ClusterZone related to the DNS Name")
		return ctrl.Result{}, err
	}

	// Multiple use-cases:
	// 1 Zone (example.com in NS example1) + 1 Zone (example.com in NS example3)
	// In that case: len(existingZones.Items) > 1
	// 1 Zone (example.com in NS example1) + 1 ClusterZone (example.com)
	// In that case: len(existingZones.Items) >= 1 AND len(existingClusterZones.Items) >= 1
	if len(existingZones.Items) > 1 || (len(existingZones.Items) >= 1 && len(existingClusterZones.Items) >= 1) {
		original := gz.Copy()
		conditions := gz.GetStatus().Conditions
		meta.SetStatusCondition(&conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
			Reason:             ZoneReasonDuplicated,
			Message:            ZoneMessageDuplicated,
		})
		gz.SetStatus(dnsv1alpha2.ZoneStatus{
			SyncStatus:         ptr.To(FAILED_STATUS),
			ObservedGeneration: &gz.GetObjectMeta().Generation,
			Conditions:         conditions,
		})
		if err := cl.Status().Patch(ctx, gz, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch RRSet status")
			return ctrl.Result{}, err
		}

		// Update resource metrics
		updateZonesMetrics(gz)

		return ctrl.Result{}, nil
	}

	// Get zone
	zoneRes, err := getZoneExternalResources(ctx, gz.GetObjectMeta().Name, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	syncStatus, conditionMessage, conditionReason, conditionStatus, err := zoneExternalResourcesReconcile(ctx, zoneRes, gz, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	if syncStatus == nil {
		syncStatus = ptr.To(SUCCEEDED_STATUS)
	}

	// Update ZoneStatus
	zoneRes, err = getZoneExternalResources(ctx, gz.GetObjectMeta().Name, PDNSClient, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = patchZoneStatus(ctx, gz, zoneRes, syncStatus, cl, metav1.Condition{
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

	// Update resource metrics
	updateZonesMetrics(gz)

	return ctrl.Result{}, nil
}

func rrsetReconcile(ctx context.Context, gr dnsv1alpha2.GenericRRset, zone dnsv1alpha2.GenericZone, isModified bool, isDeleted bool, lastUpdateTime *metav1.Time, scheme *runtime.Scheme, cl client.Client, PDNSClient PdnsClienter, log logr.Logger) (ctrl.Result, error) {
	isInFailedStatus := (gr.GetStatus().SyncStatus != nil && *gr.GetStatus().SyncStatus == FAILED_STATUS)

	// initialize syncStatus
	var syncStatus *string
	conditionStatus := metav1.ConditionTrue
	conditionReason := RrsetReasonSynced
	conditionMessage := RrsetMessageSyncSucceeded

	// examine DeletionTimestamp to determine if object is under deletion
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
			controllerutil.AddFinalizer(gr, RESOURCES_FINALIZER_NAME)
			lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
			if err := cl.Update(ctx, gr); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		finalizerRemoved := false
		if controllerutil.ContainsFinalizer(gr, RESOURCES_FINALIZER_NAME) {
			// our finalizer is present, so lets handle any external dependency
			if err := deleteRrsetExternalResources(ctx, zone, gr, PDNSClient, log); err != nil {
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
		//nolint:ineffassign
		lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
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
	if len(existingRRsets.Items) > 1 || (len(existingRRsets.Items) >= 1 && len(existingClusterRRsets.Items) >= 1) {
		original := gr.Copy()
		conditions := gr.GetStatus().Conditions
		meta.SetStatusCondition(&conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: *lastUpdateTime,
			Reason:             RrsetReasonDuplicated,
			Message:            RrsetMessageDuplicated,
		})
		name := getRRsetName(gr)
		gr.SetStatus(dnsv1alpha2.RRsetStatus{
			LastUpdateTime:     lastUpdateTime,
			DnsEntryName:       &name,
			SyncStatus:         ptr.To(FAILED_STATUS),
			ObservedGeneration: &gr.GetObjectMeta().Generation,
			Conditions:         conditions,
		})
		if err := cl.Status().Patch(ctx, gr, client.MergeFrom(original)); err != nil {
			log.Error(err, "unable to patch RRSet status")
			return ctrl.Result{}, err
		}

		// Update resource metrics
		updateRrsetsMetrics(getRRsetName(gr), gr)

		return ctrl.Result{}, nil
	}

	// Create or Update
	changed, err := createOrUpdateRrsetExternalResources(ctx, zone, gr, PDNSClient)
	if err != nil {
		log.Error(err, "Failed to create or update external resources")
		syncStatus = ptr.To(FAILED_STATUS)
		conditionStatus = metav1.ConditionFalse
		conditionReason = RrsetReasonSynchronizationFailed
		conditionMessage = err.Error()
	}
	if changed {
		lastUpdateTime = &metav1.Time{Time: time.Now().UTC()}
	}

	// Set OwnerReference
	if err := ownObject(ctx, zone, gr, scheme, cl, log); err != nil {
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
	original := gr.Copy()
	if syncStatus == nil {
		syncStatus = ptr.To(SUCCEEDED_STATUS)
	}
	conditions := gr.GetStatus().Conditions
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               "Available",
		LastTransitionTime: *lastUpdateTime,
		Status:             conditionStatus,
		Reason:             conditionReason,
		Message:            conditionMessage,
	})
	name := getRRsetName(gr)
	gr.SetStatus(dnsv1alpha2.RRsetStatus{
		LastUpdateTime:     lastUpdateTime,
		DnsEntryName:       &name,
		SyncStatus:         syncStatus,
		ObservedGeneration: &gr.GetObjectMeta().Generation,
	})
	if err := cl.Status().Patch(ctx, gr, client.MergeFrom(original)); err != nil {
		log.Error(err, "unable to patch RRSet status")
		return ctrl.Result{}, err
	}

	// Metrics calculation
	updateRrsetsMetrics(getRRsetName(gr), gr)

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

func createZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, PDNSClient PdnsClienter, log logr.Logger) error {
	// Make Nameservers canonical
	for i, ns := range zone.GetSpec().Nameservers {
		zone.GetSpec().Nameservers[i] = makeCanonical(ns)
	}

	// Make Catalog canonical
	var catalog *string
	if zone.GetSpec().Catalog != nil {
		catalog = ptr.To(makeCanonical(ptr.Deref(zone.GetSpec().Catalog, "")))
	}

	z := powerdns.Zone{
		ID:          &zone.GetObjectMeta().Name,
		Name:        &zone.GetObjectMeta().Name,
		Kind:        powerdns.ZoneKindPtr(powerdns.ZoneKind(zone.GetSpec().Kind)),
		DNSsec:      ptr.To(false),
		SOAEditAPI:  zone.GetSpec().SOAEditAPI,
		Nameservers: zone.GetSpec().Nameservers,
		Catalog:     catalog,
	}

	_, err := PDNSClient.Zones.Add(ctx, &z)
	if err != nil {
		log.Error(err, "Failed to create zone")
		return err
	}

	return nil
}

func updateZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, PDNSClient PdnsClienter, log logr.Logger) error {
	zoneKind := powerdns.ZoneKind(zone.GetSpec().Kind)

	// Make Catalog canonical
	var catalog *string
	if zone.GetSpec().Catalog != nil {
		catalog = ptr.To(makeCanonical(ptr.Deref(zone.GetSpec().Catalog, "")))
	}

	err := PDNSClient.Zones.Change(ctx, zone.GetObjectMeta().Name, &powerdns.Zone{
		Name:        &zone.GetObjectMeta().Name,
		Kind:        &zoneKind,
		Nameservers: zone.GetSpec().Nameservers,
		Catalog:     catalog,
		SOAEditAPI:  zone.GetSpec().SOAEditAPI,
	})
	if err != nil {
		log.Error(err, "Failed to update zone")
		return err
	}
	return nil
}

func updateNsOnZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, ttl uint32, PDNSClient PdnsClienter, log logr.Logger) error {
	nameserversCanonical := []string{}
	for _, n := range zone.GetSpec().Nameservers {
		nameserversCanonical = append(nameserversCanonical, makeCanonical(n))
	}

	err := PDNSClient.Records.Change(ctx, makeCanonical(zone.GetObjectMeta().Name), makeCanonical(zone.GetObjectMeta().Name), powerdns.RRTypeNS, ttl, nameserversCanonical)
	if err != nil {
		log.Error(err, "Failed to update NS in zone")
		return err
	}
	return nil
}

func deleteZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, PDNSClient PdnsClienter, log logr.Logger) error {
	err := PDNSClient.Zones.Delete(ctx, zone.GetObjectMeta().Name)
	// Zone may have already been deleted and it is not an error
	if err != nil && err.Error() != ZONE_NOT_FOUND_MSG {
		log.Error(err, "Failed to delete zone")
		return err
	}
	return nil
}

func zoneExternalResourcesReconcile(ctx context.Context, zoneRes *powerdns.Zone, gz dnsv1alpha2.GenericZone, PDNSClient PdnsClienter, log logr.Logger) (*string, string, string, metav1.ConditionStatus, error) {
	// Initialization
	var syncStatus *string
	conditionStatus := metav1.ConditionTrue
	conditionReason := ZoneReasonSynced
	conditionMessage := ZoneMessageSyncSucceeded

	if zoneRes.Name == nil {
		// If Zone does not exist, create it
		err := createZoneExternalResources(ctx, gz, PDNSClient, log)
		if err != nil {
			log.Error(err, "Failed to create external resources")
			syncStatus = ptr.To(FAILED_STATUS)
			conditionStatus = metav1.ConditionFalse
			conditionReason = ZoneReasonSynchronizationFailed
			conditionMessage = err.Error()
		}
	} else {
		// If Zone exists, compare content and update it if necessary
		ns, err := PDNSClient.Records.Get(ctx, gz.GetObjectMeta().Name, gz.GetObjectMeta().Name, ptr.To(powerdns.RRTypeNS))
		if err != nil {
			return nil, "", "", "", err
		}

		// An issue exist on GET API Calls, comments for another RRSet are included although we filter
		// See https://github.com/PowerDNS/pdns/issues/14539
		// See https://github.com/PowerDNS/pdns/pull/14045
		var filteredRRset powerdns.RRset
		for _, rr := range ns {
			if *rr.Name == makeCanonical(gz.GetObjectMeta().Name) && *rr.Type == powerdns.RRTypeNS {
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
		zoneIdentical, nsIdentical := zoneIsIdenticalToExternalZone(gz, zoneRes, nameservers)

		// Nameservers changes
		if !nsIdentical {
			ttl := ptr.To(DEFAULT_TTL_FOR_NS_RECORDS)
			if filteredRRset.TTL != nil {
				ttl = filteredRRset.TTL
			}
			err := updateNsOnZoneExternalResources(ctx, gz, *ttl, PDNSClient, log)
			if err != nil {
				syncStatus = ptr.To(FAILED_STATUS)
				conditionStatus = metav1.ConditionFalse
				conditionReason = ZoneReasonNSSynchronizationFailed
				conditionMessage = err.Error()
			}
		}
		// Other changes
		if !zoneIdentical {
			err := updateZoneExternalResources(ctx, gz, PDNSClient, log)
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

func patchZoneStatus(ctx context.Context, zone dnsv1alpha2.GenericZone, zoneRes *powerdns.Zone, status *string, cl client.Client, condition metav1.Condition) error {
	original := zone.Copy()

	kind := string(ptr.Deref(zoneRes.Kind, ""))
	conditions := zone.GetStatus().Conditions
	meta.SetStatusCondition(&conditions, condition)
	zone.SetStatus(dnsv1alpha2.ZoneStatus{
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
	})
	return cl.Status().Patch(ctx, zone, client.MergeFrom(original))
}

func deleteRrsetExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, rrset dnsv1alpha2.GenericRRset, PDNSClient PdnsClienter, log logr.Logger) error {
	err := PDNSClient.Records.Delete(ctx, zone.GetObjectMeta().Name, getRRsetName(rrset), powerdns.RRType(rrset.GetSpec().Type))
	if err != nil {
		log.Error(err, "Failed to delete record")
		return err
	}

	return nil
}

func createOrUpdateRrsetExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, rrset dnsv1alpha2.GenericRRset, PDNSClient PdnsClienter) (bool, error) {
	name := getRRsetName(rrset)
	rrType := powerdns.RRType(rrset.GetSpec().Type)
	// Looking for a record with same Name and Type
	records, err := PDNSClient.Records.Get(ctx, zone.GetObjectMeta().Name, name, &rrType)
	if err != nil && !errors.IsNotFound(err) {
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

func ownObject(ctx context.Context, zone dnsv1alpha2.GenericZone, rrset dnsv1alpha2.GenericRRset, scheme *runtime.Scheme, cl client.Client, log logr.Logger) error {
	err := ctrl.SetControllerReference(zone, rrset, scheme)
	if err != nil {
		log.Error(err, "Failed to set owner reference. Is there already a controller managing this object?")
		return err
	}
	return cl.Update(ctx, rrset)
}
