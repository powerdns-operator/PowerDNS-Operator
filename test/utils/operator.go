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
	"fmt"
	"os/exec"
	"strings"
)

const (
	// OperatorNamespace is the namespace the operator is deployed into by the
	// config/default overlay.
	OperatorNamespace = "powerdns-operator-system"
	// OperatorImage is the image tag built and loaded into Kind for e2e tests.
	OperatorImage = "example.com/powerdns-operator:e2e"
	// operatorSecretManifest holds the PDNS credentials consumed by the manager.
	operatorSecretManifest = "test/e2e/testdata/operator-secret.yaml"
)

// CreateNamespace creates a namespace, ignoring an already-exists error.
func CreateNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "create", "ns", namespace)
	if _, err := Run(cmd); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		return err
	}
	return nil
}

// DeleteNamespace deletes a namespace, ignoring not-found errors.
func DeleteNamespace(namespace string) {
	cmd := exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found=true")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// CreateOperatorSecret creates the powerdns-operator-manager secret holding the
// PDNS API credentials. The operator namespace must exist beforehand.
func CreateOperatorSecret() error {
	cmd := exec.Command("kubectl", "apply", "-f", operatorSecretManifest)
	_, err := Run(cmd)
	return err
}

// BuildAndLoadOperatorImage builds the manager image and loads it into the Kind
// cluster.
func BuildAndLoadOperatorImage(image string) error {
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", image))
	if _, err := Run(cmd); err != nil {
		return err
	}
	return LoadImageToKindClusterWithName(image)
}

// InstallCRDs installs the operator CRDs via `make install`.
func InstallCRDs() error {
	cmd := exec.Command("make", "install")
	_, err := Run(cmd)
	return err
}

// UninstallCRDs removes the operator CRDs via `make uninstall`.
func UninstallCRDs() {
	cmd := exec.Command("make", "uninstall", "ignore-not-found=true")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// DeployOperator deploys the controller-manager via `make deploy`.
func DeployOperator(image string) error {
	cmd := exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", image))
	_, err := Run(cmd)
	return err
}

// UndeployOperator removes the controller-manager via `make undeploy`.
func UndeployOperator() {
	cmd := exec.Command("make", "undeploy", "ignore-not-found=true")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// WaitForControllerManager waits until the controller-manager deployment is
// Available.
func WaitForControllerManager() error {
	cmd := exec.Command("kubectl", "wait", "deployment",
		"-l", "control-plane=controller-manager",
		"--for", "condition=Available",
		"--namespace", OperatorNamespace,
		"--timeout", "3m",
	)
	_, err := Run(cmd)
	return err
}

// ApplyManifest applies the given YAML content via `kubectl apply -f -`.
func ApplyManifest(manifest string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := Run(cmd)
	return err
}

// DeleteManifest deletes the resources described by the given YAML content.
func DeleteManifest(manifest string) error {
	cmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := Run(cmd)
	return err
}

// GetResourceField returns a single field of a kubernetes resource using a
// jsonpath expression. namespace may be empty for cluster-scoped resources.
func GetResourceField(kind, name, namespace, jsonpath string) (string, error) {
	args := []string{"get", kind, name, "-o", fmt.Sprintf("jsonpath=%s", jsonpath)}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	cmd := exec.Command("kubectl", args...)
	out, err := Run(cmd)
	return string(out), err
}
