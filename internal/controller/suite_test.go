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
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dnsv1alpha1 "gitlab.tech.orange/parent-factory/hzf-tools/powerdns-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

var (
	zones   map[string]*powerdns.Zone
	records map[string]*powerdns.RRset
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.29.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = dnsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// Initialize mockClient
	m := NewMockClient()
	zones = map[string]*powerdns.Zone{}
	records = map[string]*powerdns.RRset{}
	err = (&RRsetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records: m.Records,
			Zones:   m.Zones,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ZoneReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records: m.Records,
			Zones:   m.Zones,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

type mockClient struct {
	Zones   mockZonesClient
	Records mockRecordsClient
}

type mockZonesClient struct{}
type mockRecordsClient struct{}

func NewMockClient() mockClient {
	return mockClient{
		Zones:   mockZonesClient{},
		Records: mockRecordsClient{},
	}
}

func (m mockZonesClient) Add(ctx context.Context, zone *powerdns.Zone) (*powerdns.Zone, error) {
	if _, ok := zones[makeCanonical(*zone.Name)]; ok {
		return &powerdns.Zone{}, powerdns.Error{StatusCode: ZONE_CONFLICT_CODE, Status: fmt.Sprintf("%d %s", ZONE_CONFLICT_CODE, ZONE_CONFLICT_MSG), Message: ZONE_CONFLICT_MSG}
	}

	// Serial initialization
	now := time.Now().UTC()
	serial := uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1
	zone.Serial = &serial

	// RRset type NS creation
	zoneCanonicalName := makeCanonical(*zone.Name)
	rrset := powerdns.RRset{
		Name:    &zoneCanonicalName,
		TTL:     Uint32(DEFAULT_TTL_FOR_NS_RECORDS),
		Type:    RRType(powerdns.RRTypeNS),
		Records: []powerdns.Record{},
	}
	for _, ns := range zone.Nameservers {
		nsName := ns
		rrset.Records = append(rrset.Records, powerdns.Record{Content: &nsName, Disabled: Bool(false), SetPTR: Bool(false)})
	}
	records[zoneCanonicalName] = &rrset
	zones[zoneCanonicalName] = zone
	return zone, nil
}

func (m mockZonesClient) Get(ctx context.Context, domain string) (*powerdns.Zone, error) {
	if z, ok := zones[makeCanonical(domain)]; ok {
		return z, nil
	}
	return &powerdns.Zone{}, powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
}

func (m mockZonesClient) Delete(ctx context.Context, domain string) error {
	if _, ok := zones[makeCanonical(domain)]; !ok {
		return powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
	}
	delete(zones, makeCanonical(domain))
	return nil
}

func (m mockZonesClient) Change(ctx context.Context, domain string, zone *powerdns.Zone) error {
	localZone, ok := zones[makeCanonical(domain)]
	if !ok {
		return powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
	}
	serial := localZone.Serial
	if *zone.Kind != *localZone.Kind {
		serial = Uint32(*localZone.Serial + uint32(1))
	}
	zone.Serial = serial

	zones[makeCanonical(domain)] = zone
	return nil
}

func (m mockRecordsClient) Get(ctx context.Context, domain string, name string, recordType *powerdns.RRType) ([]powerdns.RRset, error) {
	results := []powerdns.RRset{}
	if record, ok := records[makeCanonical(name)]; ok {
		results = append(results, *record)
		return results, nil
	}
	return results, nil
}

func (m mockRecordsClient) Change(ctx context.Context, domain string, name string, recordType powerdns.RRType, ttl uint32, content []string, options ...func(*powerdns.RRset)) error {
	var recordIdentical, recordAdded, ok bool
	var rrset *powerdns.RRset
	zone := &powerdns.Zone{}

	if rrset, ok = records[makeCanonical(name)]; !ok {
		rrset = &powerdns.RRset{}
		recordAdded = true
	}

	// TTL & Records comparison
	if !recordAdded {
		localRecords := []string{}
		for _, r := range rrset.Records {
			localRecords = append(localRecords, *r.Content)
		}
		recordIdentical = reflect.DeepEqual(localRecords, content) && reflect.DeepEqual(*rrset.TTL, ttl)
	}

	rrset.Name = &name
	rrset.Type = &recordType
	rrset.TTL = &ttl
	rrset.ChangeType = powerdns.ChangeTypePtr(powerdns.ChangeTypeReplace)
	rrset.Records = make([]powerdns.Record, 0)
	for _, opt := range options {
		opt(rrset)
	}

	for _, c := range content {
		localContent := c
		r := powerdns.Record{Content: &localContent, Disabled: Bool(false), SetPTR: Bool(false)}
		rrset.Records = append(rrset.Records, r)
	}

	if !recordIdentical || recordAdded {
		if zone, ok = zones[makeCanonical(domain)]; ok {
			zone.Serial = Uint32(*zone.Serial + uint32(1))
			zones[makeCanonical(domain)] = zone
		}
	}

	records[makeCanonical(name)] = rrset
	return nil
}

func (m mockRecordsClient) Delete(ctx context.Context, domain string, name string, recordType powerdns.RRType) error {
	delete(records, makeCanonical(name))
	return nil
}

func getMockedNameservers(zoneName string) (result []string) {
	rrset := records[makeCanonical(zoneName)]
	for _, r := range rrset.Records {
		result = append(result, strings.TrimSuffix(*r.Content, "."))
	}
	slices.Sort(result)
	return
}

func getMockedKind(zoneName string) (result string) {
	zone := zones[makeCanonical(zoneName)]
	if zone.Kind != nil {
		result = string(*zone.Kind)
	}
	return
}
