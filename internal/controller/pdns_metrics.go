package controller

import (
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
