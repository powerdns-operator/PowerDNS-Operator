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
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dnsv1alpha2 "github.com/orange-opensource/powerdns-operator/api/v1alpha2"
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
	zones   sync.Map
	records sync.Map
)

const (
	FIRST_GENERATION    = 1
	MODIFIED_GENERATION = 2
	FAKE_SITE           = "fake.com"
)

const (
	NATIVE_KIND_ZONE   = "Native"
	MASTER_KIND_ZONE   = "Master"
	SLAVE_KIND_ZONE    = "Slave"
	PRODUCER_KIND_ZONE = "Producer"
	CONSUMER_KIND_ZONE = "Consumer"
)

// writeToZonesMap stores a value in the Zones sync.Map
func writeToZonesMap(key string, value *powerdns.Zone) {
	result, err := json.Marshal(value)
	if err != nil {
		GinkgoLogr.Error(err, "error while marshalling zone")
	}
	zones.Store(key, result)
}

// readFromZonesMap retrieves a value from the Zones sync.Map
func readFromZonesMap(key string) (*powerdns.Zone, bool) {
	result := &powerdns.Zone{}
	value, ok := zones.Load(key)
	if !ok {
		return result, false
	}
	valueByte, _ := value.([]byte)
	err := json.Unmarshal(valueByte, result)
	if err != nil {
		GinkgoLogr.Error(err, "error while unmarshalling zone")
	}
	return result, true
}

// deleteFromZonesMap removes a key from the Zones sync.Map
func deleteFromZonesMap(key string) {
	zones.Delete(key)
}

// writeToRecordsMap stores a value in the Records sync.Map
func writeToRecordsMap(key string, value *powerdns.RRset) {
	result, err := json.Marshal(value)
	if err != nil {
		GinkgoLogr.Error(err, "error while marshalling rrset")
	}
	records.Store(key, result)
}

// readFromRecordsMap retrieves a value from the Records sync.Map
func readFromRecordsMap(key string) (*powerdns.RRset, bool) {
	result := &powerdns.RRset{}
	value, ok := records.Load(key)
	if !ok {
		return result, false
	}
	valueByte, _ := value.([]byte)
	err := json.Unmarshal(valueByte, result)
	if err != nil {
		GinkgoLogr.Error(err, "error while unmarshalling rrset")
	}
	return result, true
}

// deleteFromRecordsMap removes a key from the Records sync.Map
func deleteFromRecordsMap(key string) {
	records.Delete(key)
}

// resetZonesMap removes all entries from the Zones sync.Map
func resetZonesMap() {
	zones.Clear()
}

// resetZonesMap removes all entries from the Zones sync.Map
func resetRecordsMap() {
	records.Clear()
}

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
			fmt.Sprintf("1.31.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = dnsv1alpha2.AddToScheme(scheme.Scheme)
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
	err = (&RRsetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records: m.Records,
			Zones:   m.Zones,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterRRsetReconciler{
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

	err = (&ClusterZoneReconciler{
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

	/*
		#####################################################################################
		#  Application Namespaces creation
		#####################################################################################
	*/
	By("creating application namespaces")
	namespaces := []string{
		"example1",
		"example2",
		"example3",
		"example4",
		"example5",
		"example6",
		"example7",
	}

	for _, n := range namespaces {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: n,
			},
		}
		_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, ns, func() error {
			return nil
		})
		Expect(err).Should(Succeed())
	}

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
	// Specific behaviour to
	// for "fake" domain, return an error
	if *zone.Name == FAKE_SITE {
		return &powerdns.Zone{}, &powerdns.Error{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
			Message:    "Internal Server Error",
		}
	}

	if _, ok := readFromZonesMap(makeCanonical(*zone.Name)); ok {
		return &powerdns.Zone{}, powerdns.Error{StatusCode: ZONE_CONFLICT_CODE, Status: fmt.Sprintf("%d %s", ZONE_CONFLICT_CODE, ZONE_CONFLICT_MSG), Message: ZONE_CONFLICT_MSG}
	}

	// Serial initialization
	var serial uint32
	switch *zone.SOAEditAPI {
	case "EPOCH":
		serial = uint32(time.Now().UTC().Unix())
	case "INCREASE":
		serial = uint32(1)
	default:
		now := time.Now().UTC()
		serial = uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1
	}
	zone.Serial = &serial

	// RRset type NS creation
	zoneCanonicalName := makeCanonical(*zone.Name)
	rrset := powerdns.RRset{
		Name:    &zoneCanonicalName,
		TTL:     ptr.To(DEFAULT_TTL_FOR_NS_RECORDS),
		Type:    ptr.To(powerdns.RRTypeNS),
		Records: []powerdns.Record{},
	}
	for _, ns := range zone.Nameservers {
		nsName := ns
		rrset.Records = append(rrset.Records, powerdns.Record{Content: &nsName, Disabled: ptr.To(false), SetPTR: ptr.To(false)})
	}
	writeToRecordsMap(zoneCanonicalName, &rrset)
	writeToZonesMap(zoneCanonicalName, zone)
	return zone, nil
}

func (m mockZonesClient) Get(ctx context.Context, domain string) (*powerdns.Zone, error) {
	// Specific behaviour to
	// for "fake" domain, return an error
	if domain == FAKE_SITE {
		return &powerdns.Zone{}, &powerdns.Error{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
			Message:    "Internal Server Error",
		}
	}

	if z, ok := readFromZonesMap(makeCanonical(domain)); ok {
		return z, nil
	}
	return &powerdns.Zone{}, powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
}

func (m mockZonesClient) Delete(ctx context.Context, domain string) error {
	// Specific behaviour to
	// for "fake" domain, return an error
	if domain == FAKE_SITE {
		return &powerdns.Error{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
			Message:    "Internal Server Error",
		}
	}

	deleteFromRecordsMap(makeCanonical(domain))
	if _, ok := readFromZonesMap(makeCanonical(domain)); !ok {
		return powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
	}
	deleteFromZonesMap(makeCanonical(domain))
	return nil
}

func (m mockZonesClient) Change(ctx context.Context, domain string, zone *powerdns.Zone) error {
	// Specific behaviour to
	// for "fake" domain, return an error
	if *zone.Name == FAKE_SITE {
		return &powerdns.Error{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
			Message:    "Internal Server Error",
		}
	}

	localZone, ok := readFromZonesMap(makeCanonical(domain))
	if !ok {
		return powerdns.Error{StatusCode: ZONE_NOT_FOUND_CODE, Status: fmt.Sprintf("%d %s", ZONE_NOT_FOUND_CODE, ZONE_NOT_FOUND_MSG), Message: ZONE_NOT_FOUND_MSG}
	}
	serial := localZone.Serial
	if *zone.Kind != *localZone.Kind || *zone.Catalog != *localZone.Catalog || *zone.SOAEditAPI != *localZone.SOAEditAPI {
		switch *zone.SOAEditAPI {
		case "EPOCH":
			serial = ptr.To(uint32(time.Now().UTC().Unix()))
		case "INCREASE":
			serial = ptr.To(*localZone.Serial + uint32(1))
		default:
			match, _ := regexp.MatchString("[0-9]{10}", fmt.Sprintf("%d", *localZone.Serial))
			if match {
				serial = ptr.To(*localZone.Serial + uint32(1))
				break
			}
			now := time.Now().UTC()
			serial = ptr.To(uint32(now.Year())*1000000 + uint32((now.Month()))*10000 + uint32(now.Day())*100 + 1)
		}
	}
	zone.Serial = serial

	writeToZonesMap(makeCanonical(domain), zone)
	return nil
}

func (m mockRecordsClient) Get(ctx context.Context, domain string, name string, recordType *powerdns.RRType) ([]powerdns.RRset, error) {
	results := []powerdns.RRset{}
	if record, ok := readFromRecordsMap(makeCanonical(name)); ok {
		results = append(results, *record)
		return results, nil
	}
	return results, nil
}

func (m mockRecordsClient) Change(ctx context.Context, domain string, name string, recordType powerdns.RRType, ttl uint32, content []string, options ...func(*powerdns.RRset)) error {
	// Specific behaviour to
	// for "fake" domain, return an error
	if domain == FAKE_SITE+"." {
		return &powerdns.Error{
			StatusCode: 500,
			Status:     "500 Internal Server Error",
			Message:    "Internal Server Error",
		}
	}

	// Preliminary test - Linked to 'wrong-rrset && wrong-type' test
	if string(recordType) == "AA" {
		return &powerdns.Error{
			StatusCode: 422,
			Status:     "422 Unprocessable Entity",
			Message:    "RRset " + name + " IN AA: unknown type given",
		}
	}

	// Preliminary test - Linked to 'wrong-rrset && wrong-format' test
	if string(recordType) == "SRV" && content[0] == strings.TrimSuffix(content[0], ".") {
		return &powerdns.Error{
			StatusCode: 422,
			Status:     "422 Unprocessable Entity",
			Message:    "Record " + name + "/SRV '" + strings.Join(content, ",") + "': Not in expected format (parsed as '" + strings.Join(content, ",") + ".')",
		}
	}

	// Preliminary test - Linked to 'wrong-rrset && unquoted-txt' test
	if string(recordType) == "TXT" && content[0] == strings.TrimSuffix(content[0], "\"") && content[0] == strings.TrimPrefix(content[0], "\"") {
		return &powerdns.Error{
			StatusCode: 422,
			Status:     "422 Unprocessable Entity",
			Message:    "Record " + name + "/TXT '" + strings.Join(content, ",") + "': Parsing record content (try 'pdnsutil check-zone'): Data field in DNS should start with quote(\") at position 0 of '" + strings.Join(content, ",") + "'",
		}
	}

	var isRRsetIdentical, isNewRRset, ok bool
	var rrset *powerdns.RRset
	var comment, specifiedComment string

	// The specified comment is included inside the opt function (through .WithComments)
	// So to extract it, we need to apply opt() function on an empty RRSet
	fakeRrset := &powerdns.RRset{
		Comments: []powerdns.Comment{},
	}
	for _, opt := range options {
		opt(fakeRrset)
	}
	if len(fakeRrset.Comments) > 0 {
		specifiedComment = *fakeRrset.Comments[0].Content
	}

	if rrset, ok = readFromRecordsMap(makeCanonical(name)); !ok {
		rrset = &powerdns.RRset{}
		isNewRRset = true
	}

	// TTL, Records & Comment comparison
	if !isNewRRset {
		localRecords := []string{}
		for _, r := range rrset.Records {
			localRecords = append(localRecords, *r.Content)
		}

		for _, c := range rrset.Comments {
			comment = *c.Content
		}
		isRRsetIdentical = reflect.DeepEqual(localRecords, content) && reflect.DeepEqual(*rrset.TTL, ttl) && reflect.DeepEqual(comment, specifiedComment)
	}

	rrset.Name = &name
	rrset.Type = &recordType
	rrset.TTL = &ttl
	rrset.ChangeType = powerdns.ChangeTypePtr(powerdns.ChangeTypeReplace)
	rrset.Records = make([]powerdns.Record, 0)
	rrset.Comments = []powerdns.Comment{}
	if specifiedComment != "" {
		rrset.Comments = append(rrset.Comments, powerdns.Comment{Content: &specifiedComment})
	}

	for _, c := range content {
		localContent := c
		r := powerdns.Record{Content: &localContent, Disabled: ptr.To(false), SetPTR: ptr.To(false)}
		rrset.Records = append(rrset.Records, r)
	}
	writeToRecordsMap(makeCanonical(name), rrset)

	if !isRRsetIdentical || isNewRRset {
		if zone, ok := readFromZonesMap(makeCanonical(domain)); ok {
			zone.Serial = ptr.To(*zone.Serial + uint32(1))
			writeToZonesMap(makeCanonical(domain), zone)
		}
	}

	return nil
}

func (m mockRecordsClient) Delete(ctx context.Context, domain string, name string, recordType powerdns.RRType) error {
	deleteFromRecordsMap(makeCanonical(name))
	return nil
}

func getMockedNameservers(zoneName string) (result []string) {
	rrset, _ := readFromRecordsMap(makeCanonical(zoneName))
	for _, r := range rrset.Records {
		result = append(result, strings.TrimSuffix(*r.Content, "."))
	}
	slices.Sort(result)
	return
}

func getMockedKind(zoneName string) (result string) {
	zone, _ := readFromZonesMap(makeCanonical(zoneName))
	result = string(ptr.Deref(zone.Kind, ""))
	return
}

func getMockedRecordsForType(rrsetName, rrsetType string) []string {
	result := []string{}
	rrset, ok := readFromRecordsMap(makeCanonical(rrsetName))
	if !ok {
		return result
	}
	if string(*rrset.Type) == rrsetType {
		for _, r := range rrset.Records {
			result = append(result, *r.Content)
		}
	}
	slices.Sort(result)
	return result
}

func getMockedTTL(rrsetName, rrsetType string) (result uint32) {
	rrset, _ := readFromRecordsMap(makeCanonical(rrsetName))
	if string(*rrset.Type) == rrsetType {
		result = *rrset.TTL
	}
	return
}

func getMockedComment(rrsetName, rrsetType string) (result string) {
	rrset, _ := readFromRecordsMap(makeCanonical(rrsetName))
	if string(*rrset.Type) == rrsetType {
		result = *rrset.Comments[0].Content
	}
	return
}

func getMockedCatalog(zoneName string) (result string) {
	zone, _ := readFromZonesMap(makeCanonical(zoneName))
	result = ptr.Deref(zone.Catalog, "")
	return
}

func getMockedSOAEditAPI(zoneName string) (result string) {
	zone, _ := readFromZonesMap(makeCanonical(zoneName))
	result = ptr.Deref(zone.SOAEditAPI, "")
	return
}
