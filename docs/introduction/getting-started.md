# Getting Started

PowerDNS Operator runs within your Kubernetes cluster as a deployment resource. It utilizes CustomResourceDefinitions (CRDs) to manage PowerDNS resources. The Operator communicates with the PowerDNS API to manage zones and records.

## Pre-requisites

Before you can install PowerDNS Operator, you need to have the following:

* A Kubernetes cluster v1.29.0 or later
* A PowerDNS server v4.7 or later

> Note: The PowerDNS API must be enabled and accessible from the Kubernetes cluster where the operator is running.

## Installing with Kustomize

Create the namespace and create a Secret containing the needed PowerDNS variables but you can also create the Secret using External Secrets or any other secret management tool.

Theses secrets are used to configure the PowerDNS Operator to connect to the PowerDNS API.

```bash
kubectl create namespace powerdns-operator-system
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: powerdns-operator-manager
  namespace: powerdns-operator-system
type: Opaque
stringData:
  PDNS_API_URL: https://powerdns.example.local:8081
  PDNS_API_KEY: secret
  PDNS_API_VHOST: localhost
EOF
```

Install the latest version using the following command:

```bash
kubectl apply -k https://github.com/Orange-OpenSource/PowerDNS-Operator/releases/latest/download/bundle.yaml
```

Or you can specify a specific version (e.g. `v0.1.0`):

```bash
kubectl apply -k https://github.com/Orange-OpenSource/PowerDNS-Operator/releases/download/v0.1.0/bundle.yaml
```

## Installing with Helm

A Helm chart is available on a [specific project](https://github.com/orange-opensource/PowerDNS-Operator-helm-chart).
