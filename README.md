# PowerDNS Operator

<div align="center">

![PowerDNS Operator Logo](https://img.shields.io/badge/PowerDNS-Operator-blue?style=for-the-badge&logo=kubernetes)

[![GitHub Release](https://img.shields.io/github/v/release/powerdns-operator/powerdns-operator)](https://github.com/powerdns-operator/powerdns-operator/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/powerdns-operator/powerdns-operator)](https://goreportcard.com/report/github.com/powerdns-operator/powerdns-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Documentation](https://img.shields.io/badge/docs-powerdns--operator.github.io-blue)](https://powerdns-operator.github.io/powerdns-operator/)

**Declarative DNS Management for Kubernetes**

*A Kubernetes operator that manages PowerDNS zones and records through Custom Resource Definitions (CRDs)*

</div>

## 🚀 Features

- **Declarative DNS Management**: Manage PowerDNS zones and records using Kubernetes CRDs
- **Flexible**: PowerDNS can be deployed inside or outside the Kubernetes cluster - the operator only needs API access
- **Namespace Isolation**: Support for both cluster-wide and namespace-scoped resources
- **RBAC Integration**: Fine-grained access control with Kubernetes RBAC
- **Helm Support**: Easy deployment with Helm charts
- **Metrics & Monitoring**: Built-in Prometheus metrics and Grafana dashboards
- **GitOps Ready**: Perfect for GitOps workflows with ArgoCD, Flux, or similar tools

## 📋 Prerequisites

| Component | Supported Versions |
|-----------|-------------------|
| **PowerDNS Authoritative** | 4.7, 4.8, 4.9 |
| **Kubernetes** | 1.31, 1.32, 1.33 |
| **Go** (for development) | 1.24+ |

## 🛠️ Installation

### Option 1: Using Helm (Recommended)

```bash
# Add the Helm repository
helm repo add powerdns-operator https://powerdns-operator.github.io/PowerDNS-Operator-helm-chart
helm repo update

# Install the operator
helm install powerdns-operator powerdns-operator/powerdns-operator \
  --namespace powerdns-operator-system \
  --create-namespace \
  --set api.url=https://your-powerdns-server:8081 \
  --set credentials.data.PDNS_API_KEY=you-api-key
```

### Option 2: Using Kustomize

```bash
# Create namespace
kubectl create namespace powerdns-operator-system

# Create PowerDNS configuration secret
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: powerdns-operator-manager
  namespace: powerdns-operator-system
type: Opaque
stringData:
  PDNS_API_URL: https://your-powerdns-server:8081
  PDNS_API_KEY: your-api-key
  PDNS_API_VHOST: localhost
  # And optionally
  # PDNS_API_CA_PATH="/tmp/caroot.crt"
  # PDNS_API_INSECURE=true 
EOF

# Install the operator
kubectl apply -f https://github.com/powerdns-operator/PowerDNS-Operator/releases/latest/download/bundle.yaml
```

### Option 3: Direct Installation

```bash
# Install from the main branch
kubectl apply -f https://raw.githubusercontent.com/powerdns-operator/powerdns-operator/main/dist/install.yaml

# Or install a specific version
kubectl apply -f https://github.com/powerdns-operator/PowerDNS-Operator/releases/download/v0.1.0/bundle.yaml
```

## 🔧 Configuration

For detailed configuration options, environment variables, and advanced examples, please refer to our documentation:

- **[Getting Started](docs/introduction/getting-started.md)** - Installation, configuration, and environment variables
- **[Resource Guides](docs/guides/)** - Complete guides for zones, rrsets, and cluster resources
- **[Examples](docs/snippets/)** - Practical examples for all resource types
- **[FAQ](docs/introduction/faq.md)** - Common questions and troubleshooting

## 📖 Quickstart Usage

### Resource Types

The operator supports four main resource types:

1. **ClusterZone** - Cluster-wide DNS zones
2. **Zone** - Namespace-scoped DNS zones  
3. **ClusterRRset** - Cluster-wide DNS records
4. **RRset** - Namespace-scoped DNS records

### Examples

#### Creating a Cluster Zone

```yaml
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterZone
metadata:
  name: example.org
spec:
  kind: Native
  nameservers:
    - ns1.example.org
    - ns2.example.org
```

#### Creating a Namespace Zone

```yaml
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: Zone
metadata:
  name: myapp.example.com
  namespace: default
spec:
  kind: Native
  nameservers:
    - ns1.example.com
    - ns2.example.com
```

#### Creating DNS Records

```yaml
# A Record
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: RRset
metadata:
  name: web.myapp.example.com
  namespace: default
spec:
  type: A
  ttl: 300
  name: web
  records:
    - 192.168.1.10
    - 192.168.1.11
  zoneRef:
    name: myapp.example.com
    kind: Zone

# CNAME Record
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: RRset
metadata:
  name: www.myapp.example.com
  namespace: default
spec:
  type: CNAME
  name: www
  ttl: 300
  records:
    - web.myapp.example.com
  zoneRef:
    name: myapp.example.com
    kind: Zone
```

### Checking Resource Status

```bash
# List all DNS resources
kubectl get clusterzones,zones,rrsets,clusterrrsets

# Get detailed information
kubectl describe zone myapp.example.com -n default
```

## 🔐 RBAC and Security

The operator provides granular RBAC roles for different use cases:

- **Viewer roles**: Read-only access to DNS resources
- **Editor roles**: Full access to DNS resources within a namespace
- **Cluster roles**: Cluster-wide DNS management

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## ⭐️ GitHub Stars

<div align="center">

![GitHub Stars Over Time](https://starchart.cc/powerdns-operator/powerdns-operator.svg)

</div>

