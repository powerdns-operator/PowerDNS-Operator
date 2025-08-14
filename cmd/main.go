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

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/powerdns-operator/powerdns-operator/internal/controller"

	powerdns "github.com/joeig/go-powerdns/v3"

	dnsv1alpha2 "github.com/powerdns-operator/powerdns-operator/api/v1alpha2"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dnsv1alpha2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool

	// Get environment variables for PowerDNS API configuration
	apiURL := os.Getenv("PDNS_API_URL")
	apiKey := os.Getenv("PDNS_API_KEY")
	apiVhost := os.Getenv("PDNS_API_VHOST")
	if apiVhost == "" {
		apiVhost = "localhost"
	}
	apiInsecureStr := os.Getenv("PDNS_API_INSECURE")
	var apiInsecure bool
	if apiInsecureStr != "" {
		if insecure, err := strconv.ParseBool(apiInsecureStr); err == nil {
			apiInsecure = insecure
		}
	}
	apiCAPath := os.Getenv("PDNS_API_CA_PATH")

	// Parse PowerDNS API timeout from environment variable (in seconds)
	apiTimeoutStr := os.Getenv("PDNS_API_TIMEOUT")
	apiTimeoutSeconds := 10 // default timeout in seconds
	if apiTimeoutStr != "" {
		if timeout, err := strconv.Atoi(apiTimeoutStr); err == nil && timeout > 0 {
			apiTimeoutSeconds = timeout
		}
	}

	// Parse command line flags
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&apiURL, "pdns-api-url", apiURL, "The URL of the PowerDNS API")
	flag.StringVar(&apiKey, "pdns-api-key", apiKey, "The API key to authenticate with the PowerDNS API")
	flag.StringVar(&apiVhost, "pdns-api-vhost", apiVhost, "The vhost of the PowerDNS API")
	flag.IntVar(&apiTimeoutSeconds, "pdns-api-timeout", apiTimeoutSeconds,
		"The timeout for PowerDNS API requests (in seconds)")
	flag.BoolVar(&apiInsecure, "pdns-api-insecure", apiInsecure,
		"Enable insecure connections to PowerDNS API")
	flag.StringVar(&apiCAPath, "pdns-api-ca-path", apiCAPath, "The path to certificate authority")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Validate mandatory configuration
	if apiURL == "" {
		setupLog.Error(nil, "PDNS_API_URL environment variable or --pdns-api-url flag is required")
		os.Exit(1)
	}
	setupLog.Info("PowerDNS API URL", "url", apiURL)

	if apiKey == "" {
		setupLog.Error(nil, "PDNS_API_KEY environment variable or --pdns-api-key flag is required")
		os.Exit(1)
	}
	setupLog.Info("PowerDNS API vhost", "vhost", apiVhost)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6bc048b3.cav.enablers.ob",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize a http.Client to communicate with PowerDNS API
	var httpClient *http.Client
	tlsConfig := &tls.Config{
		InsecureSkipVerify: apiInsecure,
	}
	if apiInsecure {
		setupLog.Info("the communication with PowerDNS API is set as insecure")
	}

	if apiCAPath != "" {
		caCert, err := os.ReadFile(apiCAPath)
		if err != nil {
			setupLog.Error(err, "unable to load CA certificate")
			os.Exit(1)
		}
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			setupLog.Error(err, "unable to parse CA certificate")
			os.Exit(1)
		}
		setupLog.Info("CA certificate parsed successfully", "apiCAPath", apiCAPath)
		tlsConfig.RootCAs = caCertPool
	}

	tr := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient = &http.Client{Transport: tr}

	pdnsClient, err := PDNSClientInitializer(apiURL, apiKey, apiVhost, apiTimeoutSeconds,
		httpClient)
	if err != nil {
		setupLog.Error(err, "unable to initialize connection with PowerDNS server")
		os.Exit(1)
	}
	if err = (&controller.ZoneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		PDNSClient: controller.PdnsClienter{
			Records: pdnsClient.Records,
			Zones:   pdnsClient.Zones,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Zone")
		os.Exit(1)
	}
	if err = (&controller.RRsetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		PDNSClient: controller.PdnsClienter{
			Records: pdnsClient.Records,
			Zones:   pdnsClient.Zones,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RRset")
		os.Exit(1)
	}
	if err = (&controller.ClusterZoneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		PDNSClient: controller.PdnsClienter{
			Records: pdnsClient.Records,
			Zones:   pdnsClient.Zones,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterZone")
		os.Exit(1)
	}
	if err = (&controller.ClusterRRsetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		PDNSClient: controller.PdnsClienter{
			Records: pdnsClient.Records,
			Zones:   pdnsClient.Zones,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterRRset")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func PDNSClientInitializer(baseURL string, key string, vhost string, timeoutSeconds int,
	httpClient *http.Client) (*powerdns.Client, error) {
	client := powerdns.New(baseURL, vhost, powerdns.WithAPIKey(key), powerdns.WithHTTPClient(httpClient))

	// Test connectivity by attempting to get server information
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	server, err := client.Servers.Get(ctx, vhost)
	if err != nil {
		return nil, err
	}

	// Log server information for operational visibility
	if server.Version != nil {
		setupLog.Info("Connected to PowerDNS server", "version", *server.Version)
	}
	if server.DaemonType != nil {
		setupLog.Info("PowerDNS daemon type", "type", *server.DaemonType)
		// Validate that we're connecting to an Authoritative server (not Recursor)
		if *server.DaemonType != "authoritative" {
			setupLog.Info("Warning: PowerDNS Operator is designed for Authoritative servers", "daemon_type", *server.DaemonType)
		}
	}
	if server.ID != nil {
		setupLog.Info("PowerDNS server ID", "id", *server.ID)
	}

	// Log successful connection with key details
	setupLog.Info("PowerDNS connectivity test successful", "url", baseURL, "vhost", vhost)

	return client, nil
}
