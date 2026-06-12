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

package utils

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive

	"github.com/joeig/go-powerdns/v3"
)

const (
	// PowerDNSNamespace is the namespace where the in-cluster PowerDNS test
	// backend is deployed.
	PowerDNSNamespace = "powerdns"
	// PowerDNSAPIKey is the API key configured on the test PowerDNS instance.
	// It must match testdata/powerdns.yaml and testdata/operator-secret.yaml.
	PowerDNSAPIKey = "e2e-test-api-key"
	// PowerDNSVHost is the PowerDNS server id (a.k.a. virtual host).
	PowerDNSVHost = "localhost"
	// PowerDNSLocalAPIURL is the API endpoint reachable from the test process
	// once the port-forward is established.
	PowerDNSLocalAPIURL = "http://localhost:8081"
	// powerDNSManifest is the path (relative to the project root) of the
	// PowerDNS deployment manifest.
	powerDNSManifest = "test/e2e/testdata/powerdns.yaml"
)

// DeployPowerDNS deploys the in-cluster PowerDNS authoritative server used as
// the real backend for e2e tests and waits for it to become available.
func DeployPowerDNS() error {
	cmd := exec.Command("kubectl", "apply", "-f", powerDNSManifest)
	if _, err := Run(cmd); err != nil {
		return err
	}

	cmd = exec.Command("kubectl", "wait", "deployment.apps/powerdns",
		"--for", "condition=Available",
		"--namespace", PowerDNSNamespace,
		"--timeout", "5m",
	)
	_, err := Run(cmd)
	return err
}

// UndeployPowerDNS removes the in-cluster PowerDNS test backend.
func UndeployPowerDNS() {
	cmd := exec.Command("kubectl", "delete", "-f", powerDNSManifest, "--ignore-not-found=true")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// PortForward represents a running `kubectl port-forward` process.
type PortForward struct {
	cmd *exec.Cmd
}

// Stop terminates the port-forward process.
func (p *PortForward) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	if err := p.cmd.Process.Kill(); err != nil {
		warnError(err)
	}
	_ = p.cmd.Wait()
}

// StartPowerDNSPortForward starts a `kubectl port-forward` to the PowerDNS API
// service on localhost:8081 and waits until the tunnel is usable. The caller is
// responsible for calling Stop() on the returned PortForward.
func StartPowerDNSPortForward() (*PortForward, error) {
	dir, _ := GetProjectDir()
	cmd := exec.Command("kubectl", "port-forward",
		"--namespace", PowerDNSNamespace,
		"service/powerdns", "8081:8081",
	)
	cmd.Dir = dir
	fmt.Fprintf(GinkgoWriter, "running: %s\n", "kubectl port-forward service/powerdns 8081:8081")
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	pf := &PortForward{cmd: cmd}

	// Wait until the API answers through the tunnel.
	client := NewPDNSClient()
	var lastErr error
	for i := 0; i < 30; i++ {
		if _, err := client.Servers.Get(context.Background(), PowerDNSVHost); err == nil {
			return pf, nil
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	pf.Stop()
	return nil, fmt.Errorf("powerdns API not reachable through port-forward: %w", lastErr)
}

// NewPDNSClient returns a PowerDNS API client pointed at the local port-forward.
// It uses the same library as the operator, so assertions exercise the exact
// representation the operator writes.
func NewPDNSClient() *powerdns.Client {
	return powerdns.New(PowerDNSLocalAPIURL, PowerDNSVHost, powerdns.WithAPIKey(PowerDNSAPIKey))
}
