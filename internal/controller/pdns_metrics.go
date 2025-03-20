package controller

import (
	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	rrsetsStatusesMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rrsets_status",
			Help: "Statuses of RRsets processed",
		},
		[]string{"fqdn", "type", "status", "name", "namespace"},
	)
	zonesStatusesMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zones_status",
			Help: "Statuses of Zones processed",
		},
		[]string{"status", "name", "namespace"},
	)
	clusterZonesStatusesMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clusterzones_status",
			Help: "Statuses of ClusterZones processed",
		},
		[]string{"status", "name"},
	)
)

func updateRrsetsMetrics(fqdn, rrsetType, rrsetStatus, name, namespace string) {
	rrsetsStatusesMetric.With(map[string]string{
		"fqdn":      fqdn,
		"type":      rrsetType,
		"status":    rrsetStatus,
		"name":      name,
		"namespace": namespace,
	}).Set(1)
}
func removeRrsetMetrics(name, namespace string) {
	rrsetsStatusesMetric.DeletePartialMatch(
		map[string]string{
			"namespace": namespace,
			"name":      name,
		},
	)
}

func updateZonesMetrics(gz dnsv1alpha2.GenericZone) {
	switch gz.(type) {
	case *dnsv1alpha2.Zone:
		zonesStatusesMetric.With(map[string]string{
			"status":    *gz.GetStatus().SyncStatus,
			"name":      gz.GetName(),
			"namespace": gz.GetNamespace(),
		}).Set(1)
	case *dnsv1alpha2.ClusterZone:
		clusterZonesStatusesMetric.With(map[string]string{
			"status": *gz.GetStatus().SyncStatus,
			"name":   gz.GetName(),
		}).Set(1)
	}
}
func removeZonesMetrics(gz dnsv1alpha2.GenericZone) {
	switch gz.(type) {
	case *dnsv1alpha2.Zone:
		zonesStatusesMetric.DeletePartialMatch(
			map[string]string{
				"namespace": gz.GetNamespace(),
				"name":      gz.GetName(),
			},
		)
	case *dnsv1alpha2.ClusterZone:
		clusterZonesStatusesMetric.DeletePartialMatch(
			map[string]string{
				"name": gz.GetName(),
			},
		)
	}
}

//nolint:unparam
func getMetricWithLabels(rrsetFQDN, rrsetType, rrsetStatus, rrsetName, rrsetNamespace string) float64 {
	return testutil.ToFloat64(rrsetsStatusesMetric.With(prometheus.Labels{
		"fqdn":      rrsetFQDN,
		"type":      rrsetType,
		"status":    rrsetStatus,
		"name":      rrsetName,
		"namespace": rrsetNamespace,
	}))
}

func countMetrics() int {
	return testutil.CollectAndCount(rrsetsStatusesMetric)
}
