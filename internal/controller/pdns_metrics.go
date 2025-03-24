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
	clusterRrsetsStatusesMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clusterrrsets_status",
			Help: "Statuses of ClusterRRsets processed",
		},
		[]string{"fqdn", "type", "status", "name"},
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

func updateRrsetsMetrics(fqdn string, gr dnsv1alpha2.GenericRRset) {
	switch gr.(type) {
	case *dnsv1alpha2.RRset:
		rrsetsStatusesMetric.With(map[string]string{
			"fqdn":      fqdn,
			"type":      gr.GetSpec().Type,
			"status":    *gr.GetStatus().SyncStatus,
			"name":      gr.GetName(),
			"namespace": gr.GetNamespace(),
		}).Set(1)

	case *dnsv1alpha2.ClusterRRset:
		clusterRrsetsStatusesMetric.With(map[string]string{
			"fqdn":   fqdn,
			"type":   gr.GetSpec().Type,
			"status": *gr.GetStatus().SyncStatus,
			"name":   gr.GetName(),
		}).Set(1)
	}

}
func removeRrsetMetrics(gr dnsv1alpha2.GenericRRset) {
	switch gr.(type) {
	case *dnsv1alpha2.RRset:
		rrsetsStatusesMetric.DeletePartialMatch(
			map[string]string{
				"namespace": gr.GetNamespace(),
				"name":      gr.GetName(),
			},
		)
		clusterRrsetsStatusesMetric.DeletePartialMatch(
			map[string]string{
				"name": gr.GetName(),
			},
		)
	}
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
func getRrsetMetricWithLabels(rrsetFQDN, rrsetType, rrsetStatus, rrsetName, rrsetNamespace string) float64 {
	return testutil.ToFloat64(rrsetsStatusesMetric.With(prometheus.Labels{
		"fqdn":      rrsetFQDN,
		"type":      rrsetType,
		"status":    rrsetStatus,
		"name":      rrsetName,
		"namespace": rrsetNamespace,
	}))
}

//nolint:unparam
func getClusterRrsetMetricWithLabels(rrsetFQDN, rrsetType, rrsetStatus, rrsetName string) float64 {
	return testutil.ToFloat64(clusterRrsetsStatusesMetric.With(prometheus.Labels{
		"fqdn":   rrsetFQDN,
		"type":   rrsetType,
		"status": rrsetStatus,
		"name":   rrsetName,
	}))
}

//nolint:unparam
func getZoneMetricWithLabels(zoneStatus, zoneName, zoneNamespace string) float64 {
	return testutil.ToFloat64(zonesStatusesMetric.With(prometheus.Labels{
		"status":    zoneStatus,
		"name":      zoneName,
		"namespace": zoneNamespace,
	}))
}

//nolint:unparam
func getClusterZoneMetricWithLabels(zoneStatus, zoneName string) float64 {
	return testutil.ToFloat64(clusterZonesStatusesMetric.With(prometheus.Labels{
		"status": zoneStatus,
		"name":   zoneName,
	}))
}

func countRrsetsMetrics() int {
	return testutil.CollectAndCount(rrsetsStatusesMetric)
}

func countClusterRrsetsMetrics() int {
	return testutil.CollectAndCount(clusterRrsetsStatusesMetric)
}

func countZonesMetrics() int {
	return testutil.CollectAndCount(zonesStatusesMetric)
}

func countClusterZonesMetrics() int {
	return testutil.CollectAndCount(clusterZonesStatusesMetric)
}
