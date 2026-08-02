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
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// errNSStampAfterCreate: Zones.Add succeeded but NS stamp failed; do not sticky-Failed.
var errNSStampAfterCreate = errors.New("failed to stamp NS after zone create")

type GenericZoneReconciler struct {
	client.Client
	log        logr.Logger
	PDNSClient PdnsClienter
	Drift      DriftConfig
}

func (gzr *GenericZoneReconciler) reconcileZone(ctx context.Context, gz dnsv1alpha2.GenericZone, isModified bool, isDeleted bool) error {
	log := gzr.log.WithValues("kind", gz.GetKind(), "name", gz.GetName(), "namespace", gz.GetNamespace())
	isInFailedStatus := (gz.GetStatus().SyncStatus != nil && *gz.GetStatus().SyncStatus == dnsv1alpha2.FAILED_STATUS)

	// examine DeletionTimestamp to determine if object is under deletion
	if !isDeleted {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(gz, RESOURCES_FINALIZER_NAME) {
			controllerutil.AddFinalizer(gz, RESOURCES_FINALIZER_NAME)
			if err := gzr.Update(ctx, gz); err != nil {
				return fmt.Errorf("failed to add finalizer on Zone: %w", err)
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
				return fmt.Errorf("failed to delete zone external resources: %w", err)
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(gz, RESOURCES_FINALIZER_NAME)
			log.V(1).Info("Removing finalizer from zone")
			finalizerRemoved = true
		}
		if controllerutil.ContainsFinalizer(gz, METRICS_FINALIZER_NAME) {
			// Remove resource metrics and finalizer
			removeZonesMetrics(gz)
			controllerutil.RemoveFinalizer(gz, METRICS_FINALIZER_NAME)
			log.V(1).Info("Removing metrics finalizer from zone")
			finalizerRemoved = true
		}
		if finalizerRemoved {
			if err := gzr.Update(ctx, gz); err != nil {
				return fmt.Errorf("failed to remove finalizers on Zone: %w", err)
			}
		}

		// Stop reconciliation as the item is being deleted
		return nil
	}

	// Sticky Failed + no Spec change: skip PDNS unless drift interval is set.
	if isInFailedStatus && !isModified && gzr.Drift.Interval == 0 {
		// Update resource metrics
		updateZonesMetrics(gz)
		return nil
	}

	// If a Zone already exists with the same DNS name:
	// * Stop reconciliation
	// * Append a Failed Status on Zone
	var existingZones dnsv1alpha2.ZoneList
	if err := gzr.List(ctx, &existingZones, client.MatchingFields{"Zone.Entry.Name": gz.GetName()}); err != nil {
		return fmt.Errorf("error while listing Zone related to the DNS Name: %w", err)
	}
	var existingClusterZones dnsv1alpha2.ClusterZoneList
	if err := gzr.List(ctx, &existingClusterZones, client.MatchingFields{"ClusterZone.Entry.Name": gz.GetName()}); err != nil {
		return fmt.Errorf("error while listing ClusterZone related to the DNS Name: %w", err)
	}

	// Multiple use-cases:
	// 1 Zone (example.com in NS example1) + 1 Zone (example.com in NS example3)
	// In that case: len(existingZones.Items) > 1
	// 1 Zone (example.com in NS example1) + 1 ClusterZone (example.com)
	// In that case: len(existingZones.Items) >= 1 AND len(existingClusterZones.Items) >= 1
	if len(existingZones.Items) > 1 || (len(existingZones.Items) >= 1 && len(existingClusterZones.Items) >= 1) {
		log.V(1).WithValues("existingZones", existingZones.Items, "existingClusterZones", existingClusterZones.Items).Info("Zone is duplicated")
		gz.SetDuplicated()

		// Update resource metrics
		updateZonesMetrics(gz)

		return fmt.Errorf("zone already exists")
	}

	// Get zone
	zoneRes, err := gzr.getZoneExternalResources(ctx, gz.GetName())
	if err != nil {
		return err
	}

	err = gzr.zoneExternalResourcesReconcile(ctx, zoneRes, gz)
	if err != nil {
		// Zone already in PDNS; requeue without sticky Failed so NS stamp can finish.
		if !errors.Is(err, errNSStampAfterCreate) {
			gz.SetSynchronizationFailed(err)
		}
		return err
	}

	// Update ZoneStatus
	zoneRes, err = gzr.getZoneExternalResources(ctx, gz.GetName())
	if err != nil {
		return err
	}

	// Clear before Succeeded so a failed clear is retried (avoids skipping grace).
	if err := clearOrphanZoneMetadata(ctx, gz.GetName(), gzr.PDNSClient, log); err != nil {
		return err
	}

	gz.SetAvailable(zoneRes)

	// Update resource metrics
	updateZonesMetrics(gz)

	if gzr.Drift.Interval > 0 {
		detectOrphanRrsets(ctx, gz, zoneRes, gzr.Client, gzr.PDNSClient, log, gzr.Drift.OrphanRRsetCleanup, gzr.Drift.OrphanRRsetGrace)
	}

	return nil
}

func (gzr *GenericZoneReconciler) getZoneExternalResources(ctx context.Context, domain string) (*powerdns.Zone, error) {
	log := gzr.log.WithValues("domain", domain)
	zoneRes, err := gzr.PDNSClient.Zones.Get(ctx, domain)
	if err != nil {
		if err.Error() != NOT_FOUND_ERROR_MSG {
			return nil, fmt.Errorf("PowerDNS API returned an error while getting external resource: %w", err)
		}
	}
	log.V(1).WithValues("zone", zoneRes).Info("External resource found")
	return zoneRes, nil
}

func (gzr *GenericZoneReconciler) createZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	log := gzr.log.WithValues("kind", zone.GetKind(), "name", zone.GetName(), "namespace", zone.GetNamespace())
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
		ID:          ptr.To(zone.GetName()),
		Name:        ptr.To(zone.GetName()),
		Kind:        powerdns.ZoneKindPtr(powerdns.ZoneKind(zone.GetSpec().Kind)),
		DNSsec:      ptr.To(false),
		SOAEditAPI:  zone.GetSpec().SOAEditAPI,
		Nameservers: zone.GetSpec().Nameservers,
		Catalog:     catalog,
		Account:     ptr.To(OperatorAccount),
	}

	_, err := gzr.PDNSClient.Zones.Add(ctx, &z)
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while creating external resource: %w", err)
	}
	log.V(1).Info("External resource created")
	return nil
}

func (gzr *GenericZoneReconciler) updateZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	log := gzr.log.WithValues("kind", zone.GetKind(), "name", zone.GetName(), "namespace", zone.GetNamespace())
	zoneKind := powerdns.ZoneKind(zone.GetSpec().Kind)

	// Make Catalog canonical
	var catalog *string
	if zone.GetSpec().Catalog != nil {
		catalog = ptr.To(makeCanonical(ptr.Deref(zone.GetSpec().Catalog, "")))
	}

	err := gzr.PDNSClient.Zones.Change(ctx, zone.GetName(), &powerdns.Zone{
		Name:        ptr.To(zone.GetName()),
		Kind:        &zoneKind,
		Nameservers: zone.GetSpec().Nameservers,
		Catalog:     catalog,
		SOAEditAPI:  zone.GetSpec().SOAEditAPI,
		Account:     ptr.To(OperatorAccount),
	})
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while updating external resource: %w", err)
	}
	log.V(1).Info("External resource updated")
	return nil
}

func (gzr *GenericZoneReconciler) updateNsOnZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, ttl uint32) error {
	log := gzr.log.WithValues("kind", zone.GetKind(), "name", zone.GetName(), "namespace", zone.GetNamespace())
	nameserversCanonical := make([]string, 0, len(zone.GetSpec().Nameservers))
	for _, n := range zone.GetSpec().Nameservers {
		nameserversCanonical = append(nameserversCanonical, makeCanonical(n))
	}

	err := gzr.PDNSClient.Records.Change(ctx, makeCanonical(zone.GetName()), makeCanonical(zone.GetName()), powerdns.RRTypeNS, ttl, nameserversCanonical, powerdns.WithComments(operatorComment(nil)))
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while updating NS in external resource: %w", err)
	}
	log.V(1).Info("NS in external resource updated")
	return nil
}

func (gzr *GenericZoneReconciler) deleteZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone) error {
	log := gzr.log.WithValues("kind", zone.GetKind(), "name", zone.GetName(), "namespace", zone.GetNamespace())
	err := gzr.PDNSClient.Zones.Delete(ctx, zone.GetName())
	// Zone may have already been deleted and it is not an error
	if err != nil && err.Error() != NOT_FOUND_ERROR_MSG {
		return fmt.Errorf("PowerDNS API returned an error while deleting external resource: %w", err)
	}
	log.V(1).Info("External resource deleted")
	return nil
}

func (gzr *GenericZoneReconciler) zoneExternalResourcesReconcile(ctx context.Context, zoneRes *powerdns.Zone, gz dnsv1alpha2.GenericZone) error {
	log := gzr.log.WithValues("kind", gz.GetKind(), "name", gz.GetName(), "namespace", gz.GetNamespace())
	if zoneRes.Name == nil {
		log.V(1).Info("External resource does not exist, creating it")
		// If Zone does not exist, create it
		err := gzr.createZoneExternalResources(ctx, gz)
		if err != nil {
			return fmt.Errorf("failed to create external resources: %w", err)
		}
		// Zones.Add omits NS comments; stamp operator account separately.
		err = gzr.updateNsOnZoneExternalResources(ctx, gz, DEFAULT_TTL_FOR_NS_RECORDS)
		if err != nil {
			return fmt.Errorf("%w: %w", errNSStampAfterCreate, err)
		}
		log.V(1).Info("External resource created")
	} else {
		log.V(1).Info("External resource exists, comparing content and updating it if necessary")
		// If Zone exists, compare content and update it if necessary
		ns, err := gzr.PDNSClient.Records.Get(ctx, gz.GetName(), gz.GetName(), ptr.To(powerdns.RRTypeNS))
		if err != nil {
			return fmt.Errorf("PowerDNS API returned an error while getting NS in external resource: %w", err)
		}

		var filteredRRset powerdns.RRset
		if len(ns) > 0 {
			filteredRRset = ns[0]
		}
		var nameservers []string
		for _, n := range filteredRRset.Records {
			nameservers = append(nameservers, strings.TrimSuffix(*n.Content, "."))
		}

		// Workflow is different on update types:
		// Nameservers changes  => patch RRSet
		// Other changes        => patch Zone
		zoneIdentical, nsIdentical := zoneIsIdenticalToExternalZone(gz, zoneRes, nameservers)
		if !hasOperatorAccount(filteredRRset.Comments) {
			nsIdentical = false
		}

		// Nameservers changes
		if !nsIdentical {
			log.Info("correcting managed PDNS drift", "kind", "ns", "zone", gz.GetName())
			incManagedCorrection("ns")
			log.V(1).Info("NS in external resource are not identical, updating them")
			ttl := ptr.To(DEFAULT_TTL_FOR_NS_RECORDS)
			if filteredRRset.TTL != nil {
				ttl = filteredRRset.TTL
			}
			err := gzr.updateNsOnZoneExternalResources(ctx, gz, *ttl)
			if err != nil {
				return fmt.Errorf("failed to update NS in external resource: %w", err)
			}
			log.V(1).Info("NS in external resource updated")
		}
		// Other changes
		if !zoneIdentical {
			log.Info("correcting managed PDNS drift", "kind", "zone", "zone", gz.GetName())
			incManagedCorrection("zone")
			log.V(1).Info("External resource is not identical, updating it")
			err := gzr.updateZoneExternalResources(ctx, gz)
			if err != nil {
				return fmt.Errorf("failed to update external resource: %w", err)
			}
			log.V(1).Info("External resource updated")
		}
	}
	log.V(1).Info("External resources reconciled")
	return nil
}
