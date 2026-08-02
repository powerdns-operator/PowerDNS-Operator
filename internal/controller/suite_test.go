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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeig/go-powerdns/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

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

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
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
	zones    sync.Map
	records  sync.Map
	metadata sync.Map
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

func resetRecordsMap() {
	records.Clear()
}

func resetMetadataMap() {
	metadata.Clear()
}

func metadataKey(domain string, kind powerdns.MetadataKind) string {
	return makeCanonical(domain) + "|" + string(kind)
}

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(
		zap.WriteTo(GinkgoWriter),
		zap.UseDevMode(true),
		zap.StacktraceLevel(zapcore.PanicLevel), // Only stacktrace at panic level
	))

	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Loopback fallback when there is no default route (CI/sandbox).
	testEnv.ControlPlane.GetAPIServer().Configure().Set("advertise-address", "127.0.0.1")

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
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
			Records:  m.Records,
			Zones:    m.Zones,
			Metadata: m.Metadata,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterRRsetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records:  m.Records,
			Zones:    m.Zones,
			Metadata: m.Metadata,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ZoneReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records:  m.Records,
			Zones:    m.Zones,
			Metadata: m.Metadata,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterZoneReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		PDNSClient: PdnsClienter{
			Records:  m.Records,
			Zones:    m.Zones,
			Metadata: m.Metadata,
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

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

type mockClient struct {
	Zones    mockZonesClient
	Records  mockRecordsClient
	Metadata mockMetadataClient
}

type mockZonesClient struct{}
type mockRecordsClient struct{}
type mockMetadataClient struct{}

func NewMockClient() mockClient {
	return mockClient{
		Zones:    mockZonesClient{},
		Records:  mockRecordsClient{},
		Metadata: mockMetadataClient{},
	}
}

func (m mockZonesClient) List(ctx context.Context) ([]powerdns.Zone, error) {
	results := make([]powerdns.Zone, 0)
	zones.Range(func(key, value any) bool {
		z, ok := readFromZonesMap(key.(string))
		if ok && z.Name != nil {
			results = append(results, *z)
		}
		return true
	})
	return results, nil
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
		return &powerdns.Zone{}, powerdns.Error{StatusCode: CONFLICT_ERROR_CODE, Status: fmt.Sprintf("%d %s", CONFLICT_ERROR_CODE, CONFLICT_ERROR_MSG), Message: CONFLICT_ERROR_MSG}
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
	return &powerdns.Zone{}, powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Status: fmt.Sprintf("%d %s", NOT_FOUND_ERROR_CODE, NOT_FOUND_ERROR_MSG), Message: NOT_FOUND_ERROR_MSG}
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
		return powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Status: fmt.Sprintf("%d %s", NOT_FOUND_ERROR_CODE, NOT_FOUND_ERROR_MSG), Message: NOT_FOUND_ERROR_MSG}
	}
	deleteFromZonesMap(makeCanonical(domain))
	metadata.Delete(metadataKey(domain, OrphanSinceMetadataKind))
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
		return powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Status: fmt.Sprintf("%d %s", NOT_FOUND_ERROR_CODE, NOT_FOUND_ERROR_MSG), Message: NOT_FOUND_ERROR_MSG}
	}
	serial := localZone.Serial
	changed := ptr.Deref(zone.Kind, "") != ptr.Deref(localZone.Kind, "") ||
		ptr.Deref(zone.Catalog, "") != ptr.Deref(localZone.Catalog, "") ||
		ptr.Deref(zone.SOAEditAPI, "") != ptr.Deref(localZone.SOAEditAPI, "") ||
		ptr.Deref(zone.Account, "") != ptr.Deref(localZone.Account, "")
	if changed {
		switch ptr.Deref(zone.SOAEditAPI, "") {
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
	var comment, specifiedComment, specifiedAccount, account string

	// The specified comment is included inside the opt function (through .WithComments)
	// So to extract it, we need to apply opt() function on an empty RRSet
	fakeRrset := &powerdns.RRset{
		Comments: []powerdns.Comment{},
	}
	for _, opt := range options {
		opt(fakeRrset)
	}
	hasSpecifiedComments := len(fakeRrset.Comments) > 0
	if hasSpecifiedComments {
		specifiedComment = ptr.Deref(fakeRrset.Comments[0].Content, "")
		specifiedAccount = ptr.Deref(fakeRrset.Comments[0].Account, "")
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
			comment = ptr.Deref(c.Content, "")
			account = ptr.Deref(c.Account, "")
		}
		isRRsetIdentical = stringSlicesEqualUnordered(localRecords, content) && reflect.DeepEqual(*rrset.TTL, ttl) && reflect.DeepEqual(comment, specifiedComment) && reflect.DeepEqual(account, specifiedAccount)
	}

	rrset.Name = &name
	rrset.Type = &recordType
	rrset.TTL = &ttl
	rrset.ChangeType = powerdns.ChangeTypePtr(powerdns.ChangeTypeReplace)
	rrset.Records = make([]powerdns.Record, 0)
	rrset.Comments = []powerdns.Comment{}
	if hasSpecifiedComments {
		rrset.Comments = append(rrset.Comments, powerdns.Comment{
			Content: ptr.To(specifiedComment),
			Account: ptr.To(specifiedAccount),
		})
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

func (m mockMetadataClient) Get(ctx context.Context, domain string, kind powerdns.MetadataKind) (*powerdns.Metadata, error) {
	value, ok := metadata.Load(metadataKey(domain, kind))
	if !ok {
		return nil, powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Status: fmt.Sprintf("%d %s", NOT_FOUND_ERROR_CODE, NOT_FOUND_ERROR_MSG), Message: NOT_FOUND_ERROR_MSG}
	}
	values, _ := value.([]string)
	return &powerdns.Metadata{
		Kind:     powerdns.MetadataKindPtr(kind),
		Metadata: values,
	}, nil
}

func (m mockMetadataClient) Set(ctx context.Context, domain string, kind powerdns.MetadataKind, values []string) (*powerdns.Metadata, error) {
	copied := append([]string(nil), values...)
	metadata.Store(metadataKey(domain, kind), copied)
	return &powerdns.Metadata{
		Kind:     powerdns.MetadataKindPtr(kind),
		Metadata: copied,
	}, nil
}

func (m mockMetadataClient) Delete(ctx context.Context, domain string, kind powerdns.MetadataKind) error {
	key := metadataKey(domain, kind)
	if _, ok := metadata.Load(key); !ok {
		return powerdns.Error{StatusCode: NOT_FOUND_ERROR_CODE, Status: fmt.Sprintf("%d %s", NOT_FOUND_ERROR_CODE, NOT_FOUND_ERROR_MSG), Message: NOT_FOUND_ERROR_MSG}
	}
	metadata.Delete(key)
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
	rrset, ok := readFromRecordsMap(makeCanonical(rrsetName))
	if !ok || rrset.Type == nil {
		return
	}
	if string(*rrset.Type) == rrsetType && len(rrset.Comments) > 0 {
		result = ptr.Deref(rrset.Comments[0].Content, "")
	}
	return
}

func getMockedCommentAccount(rrsetName, rrsetType string) (result string) {
	rrset, ok := readFromRecordsMap(makeCanonical(rrsetName))
	if !ok || rrset.Type == nil {
		return
	}
	if string(*rrset.Type) == rrsetType {
		for _, c := range rrset.Comments {
			if ptr.Deref(c.Account, "") != "" {
				return ptr.Deref(c.Account, "")
			}
		}
	}
	return
}

func getMockedZoneAccount(zoneName string) (result string) {
	zone, _ := readFromZonesMap(makeCanonical(zoneName))
	result = ptr.Deref(zone.Account, "")
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
