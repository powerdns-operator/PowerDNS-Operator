package controller

import (
	"testing"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestUpdateRrsetsMetricsReplacesStatusLabel(t *testing.T) {
	rrset := &dnsv1alpha2.RRset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metric-replace-test",
			Namespace: "metric-replace-ns",
		},
		Spec: dnsv1alpha2.RRsetSpec{
			Type: "A",
		},
		Status: dnsv1alpha2.RRsetStatus{
			SyncStatus: ptr.To(dnsv1alpha2.INVALID_STATUS),
		},
	}
	fqdn := "metric-replace-test.example.org."
	removeRrsetMetrics(rrset)
	t.Cleanup(func() {
		removeRrsetMetrics(rrset)
	})

	start := countRrsetsMetrics()
	updateRrsetsMetrics(fqdn, rrset)
	if got := countRrsetsMetrics() - start; got != 1 {
		t.Fatalf("expected 1 metric after first status, got %d", got)
	}

	rrset.Status.SyncStatus = ptr.To(dnsv1alpha2.SYNCED_STATUS)
	updateRrsetsMetrics(fqdn, rrset)
	if got := countRrsetsMetrics() - start; got != 1 {
		t.Fatalf("expected still 1 metric after status change, got %d", got)
	}
	if getRrsetMetricWithLabels(fqdn, "A", dnsv1alpha2.SYNCED_STATUS, rrset.Name, rrset.Namespace) != 1.0 {
		t.Fatal("expected synced metric to be 1.0")
	}
}
