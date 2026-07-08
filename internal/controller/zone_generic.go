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
	"strings"

	"github.com/go-logr/logr"
	"github.com/joeig/go-powerdns/v3"
	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GenericZoneReconciler struct {
	client.Client
	log        logr.Logger
	PDNSClient PdnsClienter
}

//nolint:unparam // Always return ctrl.Result{} is ok
func (gzr *GenericZoneReconciler) deleteZone(ctx context.Context, gz dnsv1alpha2.GenericZone) error {
	finalizerRemoved := false
	if controllerutil.ContainsFinalizer(gz, RESOURCES_FINALIZER_NAME) {
		// our finalizer is present, so lets handle any external dependency
		if err := gzr.deleteZoneExternalResources(ctx, gz); err != nil {
			// if fail to delete the external resource, return with error
			// so that it can be retried
			return err
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
		if err := gzr.Update(ctx, gz); err != nil {
			return err
		}
	}

	// Stop reconciliation as the item is being deleted
	return nil
}

func (gzr *GenericZoneReconciler) reconcileZone(ctx context.Context, gz dnsv1alpha2.GenericZone) error {
	log := gzr.log.WithValues("kind", gz.GetKind(), "name", gz.GetName(), "namespace", gz.GetNamespace())

	// Get zone
	log.V(1).Info("Getting zone external resources")
	zoneRes, err := gzr.getZoneExternalResources(ctx, gz.GetName())
	if err != nil {
		log.V(1).Error(err, "Failed to get zone external resources")
		switch err.Error() {
		case UNPROCESSABLE_ERROR_MSG:
			gz.SetUnprocessable("Processed", err)
		case BAD_REQUEST_ERROR_MSG:
			gz.SetBadRequest("Processed", err)
		default:
			gz.SetSynchronizationFailed("Processed", err)
		}
		return fmt.Errorf("failed to get zone external resources: %w", err)
	}

	// Let's add the finalizer and update the object.
	if !controllerutil.ContainsFinalizer(gz, RESOURCES_FINALIZER_NAME) {
		log.V(1).Info("Adding resources finalizer to Zone")
		controllerutil.AddFinalizer(gz, RESOURCES_FINALIZER_NAME)
		if err := gzr.Update(ctx, gz); err != nil {
			return fmt.Errorf("failed to add finalizer on Zone: %w", err)
		}
	}

	// After the finalizer update at first time, the status is reseted
	// so we need to set the validated status again
	gz.SetValidated()

	if zoneRes.Name == nil {
		// CREATE ZONE
		log.V(1).Info("External resource does not exist, creating it")
		err := gzr.createZoneExternalResources(ctx, gz)
		if err != nil {
			switch err.Error() {
			case UNPROCESSABLE_ERROR_MSG:
				gz.SetUnprocessable("Processed", err)
				return nil
			case BAD_REQUEST_ERROR_MSG:
				gz.SetBadRequest("Processed", err)
				return nil
			default:
				gz.SetSynchronizationFailed("Processed", err)
				return fmt.Errorf("failed to create external resources: %w", err)
			}
		}
		log.V(1).Info("External resource created")
	} else {
		// UPDATE ZONE
		log.V(1).Info("External resource exists, comparing content and updating it if necessary")
		nameservers, ttl, err := gzr.getNSFromZoneExternalResources(ctx, gz)
		if err != nil {
			return fmt.Errorf("failed to get NS from external resource: %w", err)
		}

		// Workflow is different on update types:
		// Nameservers changes  => patch RRSet
		// Other changes        => patch Zone
		zoneIdentical, nsIdentical := zoneIsIdenticalToExternalZone(gz, zoneRes, nameservers)

		// Nameservers changes
		if !nsIdentical {
			log.V(1).Info("NS in external resource are not identical, updating them")
			err := gzr.updateNsOnZoneExternalResources(ctx, gz, *ttl)
			if err != nil {
				switch err.Error() {
				case UNPROCESSABLE_ERROR_MSG:
					gz.SetUnprocessable("Processed", err)
					return nil
				case BAD_REQUEST_ERROR_MSG:
					gz.SetBadRequest("Processed", err)
					return nil
				case NOT_FOUND_ERROR_MSG:
					return fmt.Errorf("failed to update NS in external resource: %w", err)
				default:
					gz.SetSynchronizationFailed("Processed", err)
					return fmt.Errorf("failed to update NS in external resource: %w", err)
				}
			}
			log.V(1).Info("NS in external resource updated")
		}
		// Other changes
		if !zoneIdentical {
			log.V(1).Info("External resource is not identical, updating it")
			err := gzr.updateZoneExternalResources(ctx, gz)
			if err != nil {
				switch err.Error() {
				case NOT_FOUND_ERROR_MSG:
					return fmt.Errorf("failed to update external resource: %w", err)
				case UNPROCESSABLE_ERROR_MSG:
					gz.SetUnprocessable("Processed", err)
				case BAD_REQUEST_ERROR_MSG:
					gz.SetBadRequest("Processed", err)
				default:
					gz.SetSynchronizationFailed("Processed", err)
				}
				// Revert NS in external resource
				err := gzr.revertNsOnZoneExternalResources(ctx, zoneRes, nameservers, *ttl)
				if err != nil {
					switch err.Error() {
					case UNPROCESSABLE_ERROR_MSG:
						gz.SetUnprocessable("Processed", err)
						return nil
					case BAD_REQUEST_ERROR_MSG:
						gz.SetBadRequest("Processed", err)
						return nil
					case NOT_FOUND_ERROR_MSG:
						return fmt.Errorf("failed to revert NS in external resource: %w", err)
					default:
						gz.SetSynchronizationFailed("Processed", err)
						return fmt.Errorf("failed to revert NS in external resource: %w", err)
					}
				}
				return fmt.Errorf("failed to revert NS in external resource: %w", err)
			}
			log.V(1).Info("External resource updated")
		}
	}
	gz.SetProcessed()

	// Update ZoneStatus
	zoneRes, err = gzr.getZoneExternalResources(ctx, gz.GetName())
	if err != nil {
		switch err.Error() {
		case UNPROCESSABLE_ERROR_MSG:
			gz.SetUnprocessable("Available", err)
		case BAD_REQUEST_ERROR_MSG:
			gz.SetBadRequest("Available", err)
		case NOT_FOUND_ERROR_MSG:
			gz.UnsetProcessed()
			gz.UnsetAvailable()
		default:
			gz.SetSynchronizationFailed("Available", err)
		}
		return fmt.Errorf("failed to get zone external resources after update: %w", err)
	}

	gz.SetAvailable(zoneRes)

	// Update resource metrics
	// updateZonesMetrics(gz)

	return nil
}

// getZoneExternalResources gets the zone from the PowerDNS API
// and returns it as a powerdns.Zone object, nil if the zone is not found
// or an error if the PowerDNS API fails
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

	err := gzr.PDNSClient.Records.Change(ctx, makeCanonical(zone.GetName()), makeCanonical(zone.GetName()), powerdns.RRTypeNS, ttl, nameserversCanonical)
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

func (gzr *GenericZoneReconciler) alreadyExists(ctx context.Context, gz dnsv1alpha2.GenericZone) (bool, error) {
	log := gzr.log.WithValues("kind", gz.GetKind(), "name", gz.GetName(), "namespace", gz.GetNamespace())

	// If a Zone already exists with the same DNS name:
	// * Stop reconciliation
	// * Append a Failed Status on Zone
	var existingZones dnsv1alpha2.ZoneList
	if err := gzr.List(ctx, &existingZones, client.MatchingFields{"Zone.Entry.Name": gz.GetName()}); err != nil {
		return false, fmt.Errorf("error while listing Zone related to the DNS Name: %w", err)
	}
	var existingClusterZones dnsv1alpha2.ClusterZoneList
	if err := gzr.List(ctx, &existingClusterZones, client.MatchingFields{"ClusterZone.Entry.Name": gz.GetName()}); err != nil {
		return false, fmt.Errorf("error while listing ClusterZone related to the DNS Name: %w", err)
	}

	// Remove current Zone or ClusterZone from the lists
	switch gz.GetKind() {
	case "Zone":
		existingZones.Items = slices.DeleteFunc(existingZones.Items, func(item dnsv1alpha2.Zone) bool {
			return item.GetName() == gz.GetName() && item.GetNamespace() == gz.GetNamespace()
		})
	case "ClusterZone":
		existingClusterZones.Items = slices.DeleteFunc(existingClusterZones.Items, func(item dnsv1alpha2.ClusterZone) bool {
			return item.GetName() == gz.GetName()
		})
	}

	// Multiple use-cases:
	// 1 Zone (example.com in NS example1) + 1 Zone (example.com in NS example3)
	// In that case: len(existingZones.Items) > 1
	// 1 Zone (example.com in NS example1) + 1 ClusterZone (example.com)
	// In that case: len(existingZones.Items) >= 1 AND len(existingClusterZones.Items) >= 1
	if len(existingZones.Items) >= 1 || len(existingClusterZones.Items) >= 1 {
		log.V(1).WithValues("existingZones", existingZones.Items, "existingClusterZones", existingClusterZones.Items).Info("Zone is duplicated")

		return true, nil
	}
	return false, nil
}

func (gzr *GenericZoneReconciler) revertNsOnZoneExternalResources(ctx context.Context, zoneRes *powerdns.Zone, nameservers []string, ttl uint32) error {
	log := gzr.log.WithValues("name", *zoneRes.Name)
	err := gzr.PDNSClient.Records.Change(ctx, *zoneRes.Name, *zoneRes.Name, powerdns.RRTypeNS, ttl, nameservers)
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while reverting NS in external resource: %w", err)
	}
	log.V(1).Info("NS in external resource reverted")
	return nil
}

/*
	revertNsOnZoneExternalResources

func (gzr *GenericZoneReconciler) updateNsOnZoneExternalResources(ctx context.Context, zone dnsv1alpha2.GenericZone, ttl uint32) error {
	log := gzr.log.WithValues("kind", zone.GetKind(), "name", zone.GetName(), "namespace", zone.GetNamespace())
	nameserversCanonical := make([]string, 0, len(zone.GetSpec().Nameservers))
	for _, n := range zone.GetSpec().Nameservers {
		nameserversCanonical = append(nameserversCanonical, makeCanonical(n))
	}

	err := gzr.PDNSClient.Records.Change(ctx, makeCanonical(zone.GetName()), makeCanonical(zone.GetName()), powerdns.RRTypeNS, ttl, nameserversCanonical)
	if err != nil {
		return fmt.Errorf("PowerDNS API returned an error while updating NS in external resource: %w", err)
	}
	log.V(1).Info("NS in external resource updated")
	return nil
}

*/

func (gzr *GenericZoneReconciler) getNSFromZoneExternalResources(ctx context.Context, gz dnsv1alpha2.GenericZone) ([]string, *uint32, error) {
	// Get NS Records
	ns, err := gzr.PDNSClient.Records.Get(ctx, gz.GetName(), gz.GetName(), ptr.To(powerdns.RRTypeNS))
	if err != nil {
		return nil, nil, fmt.Errorf("PowerDNS API returned an error while getting NS in external resource: %w", err)
	}

	// Extract Nameservers
	var filteredRRset powerdns.RRset
	if len(ns) > 0 {
		filteredRRset = ns[0]
	}
	var nameservers []string
	for _, n := range filteredRRset.Records {
		nameservers = append(nameservers, strings.TrimSuffix(*n.Content, "."))
	}

	// Extract TTL
	ttl := ptr.To(DEFAULT_TTL_FOR_NS_RECORDS)
	if filteredRRset.TTL != nil {
		ttl = filteredRRset.TTL
	}

	return nameservers, ttl, nil
}
