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
	"fmt"
	"strings"

	"github.com/joeig/go-powerdns/v3"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dnsv1alpha1 "gitlab.tech.orange/parent-factory/hzf-tools/powerdns-operator/api/v1alpha1"
)

const (
	FINALIZER_NAME             = "dns.cav.enablers.ob/finalizer"
	DEFAULT_TTL_FOR_NS_RECORDS = 1500

	ZONE_NOT_FOUND_MSG  = "Not Found"
	ZONE_NOT_FOUND_CODE = 404
	ZONE_CONFLICT_MSG   = "Conflict"
	ZONE_CONFLICT_CODE  = 409
)

// ZoneReconciler reconciles a Zone object
type ZoneReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	PDNSClient PdnsClienter
}

//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dns.cav.enablers.ob,resources=zones/finalizers,verbs=update

func (r *ZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile Zone", "Zone.Name", req.Name)

	// Get Zone
	zone := &dnsv1alpha1.Zone{}
	err := r.Get(ctx, req.NamespacedName, zone)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if zone.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(zone, FINALIZER_NAME) {
			controllerutil.AddFinalizer(zone, FINALIZER_NAME)
			if err := r.Update(ctx, zone); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(zone, FINALIZER_NAME) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(ctx, zone); err != nil {
				// if fail to delete the external resource, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(zone, FINALIZER_NAME)
			if err := r.Update(ctx, zone); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Get zone
	zoneRes, err := r.getExternalResources(ctx, zone.ObjectMeta.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if zoneRes.Name == nil {
		// If Zone does not exist, create it
		err := r.createExternalResources(ctx, zone)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// If Zone exists, compare Type and Nameservers and update it if necessary
		ns, err := r.PDNSClient.Records.Get(ctx, zone.ObjectMeta.Name, zone.ObjectMeta.Name, RRType(powerdns.RRTypeNS))
		if err != nil {
			return ctrl.Result{}, err
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
			ttl := Uint32(DEFAULT_TTL_FOR_NS_RECORDS)
			if filteredRRset.TTL != nil {
				ttl = filteredRRset.TTL
			}
			err := r.updateNsOnExternalResources(ctx, zone, *ttl)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// Other changes
		if !zoneIdentical {
			err := r.updateExternalResources(ctx, zone)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Update ZoneStatus
	zoneRes, err = r.getExternalResources(ctx, zone.ObjectMeta.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.patchStatus(ctx, zone, zoneRes)
	if err != nil {
		if errors.IsConflict(err) {
			log.Info("Object has been modified, forcing a new reconciliation")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dnsv1alpha1.Zone{}).
		Owns(&dnsv1alpha1.RRset{}).
		Complete(r)
}

func (r *ZoneReconciler) getExternalResources(ctx context.Context, domain string) (*powerdns.Zone, error) {
	log := log.FromContext(ctx)
	zoneRes, err := r.PDNSClient.Zones.Get(ctx, domain)
	if err != nil {
		if err.Error() != ZONE_NOT_FOUND_MSG {
			log.Error(err, "Failed to get zone")
			return nil, err
		}
	}

	return zoneRes, nil
}

func (r *ZoneReconciler) deleteExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone) error {
	log := log.FromContext(ctx)

	err := r.PDNSClient.Zones.Delete(ctx, zone.ObjectMeta.Name)
	// Zone may have already been deleted and it is not an error
	if err != nil && err.Error() != ZONE_NOT_FOUND_MSG {
		log.Error(err, "Failed to delete zone")
		return err
	}

	return nil
}

func (r *ZoneReconciler) updateExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone) error {
	log := log.FromContext(ctx)

	zoneKind := powerdns.ZoneKind(zone.Spec.Kind)
	err := r.PDNSClient.Zones.Change(ctx, zone.ObjectMeta.Name, &powerdns.Zone{
		Name:        &zone.ObjectMeta.Name,
		Kind:        &zoneKind,
		Nameservers: zone.Spec.Nameservers,
	})
	if err != nil {
		log.Error(err, "Failed to update zone")
		return err
	}

	return nil
}

func (r *ZoneReconciler) updateNsOnExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone, ttl uint32) error {
	log := log.FromContext(ctx)

	nameserversCanonical := []string{}
	for _, n := range zone.Spec.Nameservers {
		nameserversCanonical = append(nameserversCanonical, fmt.Sprintf("%s.", n))
	}

	err := r.PDNSClient.Records.Change(ctx, makeCanonical(zone.ObjectMeta.Name), makeCanonical(zone.ObjectMeta.Name), powerdns.RRTypeNS, ttl, nameserversCanonical)
	if err != nil {
		log.Error(err, "Failed to update NS in zone")
		return err
	}
	return nil
}

func (r *ZoneReconciler) createExternalResources(ctx context.Context, zone *dnsv1alpha1.Zone) error {
	log := log.FromContext(ctx)

	// Make Nameservers canonical
	for i, ns := range zone.Spec.Nameservers {
		zone.Spec.Nameservers[i] = makeCanonical(ns)
	}

	z := powerdns.Zone{
		ID:     &zone.Name,
		Name:   &zone.Name,
		Kind:   powerdns.ZoneKindPtr(powerdns.ZoneKind(zone.Spec.Kind)),
		DNSsec: Bool(false),
		//		SOAEdit:     &soaEdit,
		//		SOAEditAPI:  &soaEditApi,
		//		APIRectify:  &apiRectify,
		Nameservers: zone.Spec.Nameservers,
	}

	_, err := r.PDNSClient.Zones.Add(ctx, &z)
	if err != nil {
		log.Error(err, "Failed to create zone")
		return err
	}

	return nil
}

func (r *ZoneReconciler) patchStatus(ctx context.Context, zone *dnsv1alpha1.Zone, zoneRes *powerdns.Zone) error {
	original := zone.DeepCopy()

	var kind string
	if zoneRes.Kind != nil {
		kind = string(*zoneRes.Kind)
	}
	zone.Status = dnsv1alpha1.ZoneStatus{
		ID:             zoneRes.ID,
		Name:           zoneRes.Name,
		Kind:           &kind,
		Serial:         zoneRes.Serial,
		NotifiedSerial: zoneRes.NotifiedSerial,
		EditedSerial:   zoneRes.EditedSerial,
		Masters:        zoneRes.Masters,
		DNSsec:         zoneRes.DNSsec,
		Catalog:        zoneRes.Catalog,
	}

	return r.Status().Patch(ctx, zone, client.MergeFrom(original))
}
