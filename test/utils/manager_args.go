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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const (
	// ManagerDeployment is the deployed controller-manager name (namePrefix + base).
	ManagerDeployment = "powerdns-operator-controller-manager"
	// MetricsService is the non-TLS metrics Service from config/default.
	MetricsService = "powerdns-operator-controller-manager-metrics-service"
	// MetricsLocalURL is reachable after StartMetricsPortForward.
	MetricsLocalURL  = "http://127.0.0.1:18080/metrics"
	metricsLocalPort = "18080"
)

// DefaultManagerArgs are the container args from config/manager/manager.yaml.
var DefaultManagerArgs = []string{
	"--leader-elect",
	"--health-probe-bind-address=:8081",
	"--metrics-bind-address=:8080",
}

// SetManagerArgs replaces the manager container args with DefaultManagerArgs plus extra,
// then waits for the deployment to become Available again.
func SetManagerArgs(extra ...string) error {
	args := append(append([]string{}, DefaultManagerArgs...), extra...)
	value, err := json.Marshal(args)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":%s}]`, value)
	cmd := exec.Command("kubectl", "patch", "deployment", ManagerDeployment,
		"-n", OperatorNamespace,
		"--type=json",
		"-p", patch,
	)
	if _, err := Run(cmd); err != nil {
		return err
	}
	cmd = exec.Command("kubectl", "rollout", "status", "deployment", ManagerDeployment,
		"-n", OperatorNamespace,
		"--timeout=3m",
	)
	_, err = Run(cmd)
	return err
}

// ResetManagerArgs restores the default manager args from the deploy manifest.
func ResetManagerArgs() error {
	return SetManagerArgs()
}

// StartMetricsPortForward tunnels the metrics Service to localhost:18080.
func StartMetricsPortForward() (*PortForward, error) {
	dir, _ := GetProjectDir()
	cmd := exec.Command("kubectl", "port-forward",
		"--namespace", OperatorNamespace,
		"service/"+MetricsService, metricsLocalPort+":8080",
	)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	pf := &PortForward{cmd: cmd}

	var lastErr error
	for i := 0; i < 30; i++ {
		if _, err := FetchMetrics(); err == nil {
			return pf, nil
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	pf.Stop()
	return nil, fmt.Errorf("metrics not reachable through port-forward: %w", lastErr)
}

// FetchMetrics returns the raw Prometheus text from the local metrics port-forward.
func FetchMetrics() (string, error) {
	var lastErr error
	for i := 0; i < 10; i++ {
		resp, err := http.Get(MetricsLocalURL)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("metrics status %d", resp.StatusCode)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return string(body), nil
	}
	return "", fmt.Errorf("fetch metrics: %w", lastErr)
}

// MetricValue returns the value of a Prometheus gauge/counter line matching name and labels.
// Missing series returns 0, false.
func MetricValue(body, name string, labels map[string]string) (float64, bool) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := strings.TrimPrefix(line, name)
		var valueStr string
		if strings.HasPrefix(rest, "{") {
			end := strings.Index(rest, "}")
			if end < 0 {
				continue
			}
			labelPart := rest[1:end]
			valueStr = strings.TrimSpace(rest[end+1:])
			if !prometheusLabelsMatch(labelPart, labels) {
				continue
			}
		} else {
			valueStr = strings.TrimSpace(rest)
			if len(labels) > 0 {
				continue
			}
		}
		var v float64
		if _, err := fmt.Sscanf(valueStr, "%f", &v); err != nil {
			continue
		}
		return v, true
	}
	return 0, false
}

func prometheusLabelsMatch(labelPart string, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	have := map[string]string{}
	for _, piece := range strings.Split(labelPart, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		kv := strings.SplitN(piece, "=", 2)
		if len(kv) != 2 {
			continue
		}
		have[kv[0]] = strings.Trim(kv[1], `"`)
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
