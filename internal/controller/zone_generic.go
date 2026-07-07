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
	"strings"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GenericZoneReconciler struct {
	client.Client
	log        logr.Logger
	PDNSClient PdnsClienter
}

//nolint:unparam // Always return ctrl.Result{} is ok
func (gzr *GenericZoneReconciler) reconcileZone(ctx context.Context, gz dnsv1alpha2.GenericZone, isModified bool, isDeleted bool) (ctrl.Result, error) {
	cl := gzr.Client
	log := gzr.log
	isInFailedStatus := (gz.GetStatus().SyncStatus != nil && *gz.GetStatus().SyncStatus == dnsv1alpha2.FAILED_STATUS)

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
			if err := gzr.deleteZoneExternalResources(ctx, gz); err != nil {
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
		gz.SetDuplicated()

		// Update resource metrics
		updateZonesMetrics(gz)

		return ctrl.Result{}, fmt.Errorf("zone already exists")
	}

	// Get zone
	zoneRes, err := gzr.getZoneExternalResources(ctx, gz.GetObjectMeta().Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = gzr.zoneExternalResourcesReconcile(ctx, zoneRes, gz)
	if err != nil {
		gz.SetSynchronizationFailed(err)
		return ctrl.Result{}, err
	}

	// Update ZoneStatus
	zoneRes, err = gzr.getZoneExternalResources(ctx, gz.GetObjectMeta().Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	gz.SetAvailable(zoneRes)

	// Update resource metrics
	updateZonesMetrics(gz)

	return ctrl.Result{}, nil
}

func (gzr *GenericZoneReconciler) getZoneExternalResources(ctx context.Context, domain string) (*powerdns.Zone, error) {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
	zoneRes, err := PDNSClient.Zones.Get(ctx, domain)
	if err != nil {
		if err.Error() != ZONE_NOT_FOUND_MSG {
			log.Error(err, "Failed to get zone")
			return nil, err
		}
	}
	return zoneRes, nil
}

func (gzr *GenericZoneReconciler) createZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
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

func (gzr *GenericZoneReconciler) updateZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
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

func (gzr *GenericZoneReconciler) updateNsOnZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, ttl uint32) error {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
	nameserversCanonical := make([]string, 0, len(zone.GetSpec().Nameservers))
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

func (gzr *GenericZoneReconciler) deleteZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
	err := PDNSClient.Zones.Delete(ctx, zone.GetObjectMeta().Name)
	// Zone may have already been deleted and it is not an error
	if err != nil && err.Error() != ZONE_NOT_FOUND_MSG {
		log.Error(err, "Failed to delete zone")
		return err
	}
	return nil
}

func (gzr *GenericZoneReconciler) zoneExternalResourcesReconcile(ctx context.Context, zoneRes *powerdns.Zone, gz dnsv1alpha2.GenericZone) error {
	PDNSClient := gzr.PDNSClient
	log := gzr.log
	if zoneRes.Name == nil {
		// If Zone does not exist, create it
		err := gzr.createZoneExternalResources(ctx, gz)
		if err != nil {
			log.Error(err, "Failed to create external resources")
			return err
		}
	} else {
		// If Zone exists, compare content and update it if necessary
		ns, err := PDNSClient.Records.Get(ctx, gz.GetObjectMeta().Name, gz.GetObjectMeta().Name, ptr.To(powerdns.RRTypeNS))
		if err != nil {
			return err
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
			err := gzr.updateNsOnZoneExternalResources(ctx, gz, *ttl)
			if err != nil {
				log.Error(err, "Failed to update NS in zone")
				return err
			}
		}
		// Other changes
		if !zoneIdentical {
			err := gzr.updateZoneExternalResources(ctx, gz)
			if err != nil {
				log.Error(err, "Failed to update zone")
				return err
			}
		}
	}
	return nil
}
