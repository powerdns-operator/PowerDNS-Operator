# Getting Started

## Prerequisites

For detailed prerequisites and compatibility information, see the [Stability and Support](stability-support.md) documentation.

## Installation

### Option 1: Direct Installation (Recommended)

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
EOF

# Install the operator
kubectl apply -f https://github.com/powerdns-operator/PowerDNS-Operator/releases/latest/download/bundle.yaml
```

### Option 2: Helm Installation

```bash
# Add the Helm repository
helm repo add powerdns-operator https://powerdns-operator.github.io/PowerDNS-Operator-helm-chart
helm repo update

# Install the operator
helm install powerdns-operator powerdns-operator/powerdns-operator \
  --namespace powerdns-operator-system \
  --create-namespace
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `PDNS_API_URL` | PowerDNS API server URL | Yes | None |
| `PDNS_API_KEY` | PowerDNS API authentication key | Yes | None |
| `PDNS_API_VHOST` | PowerDNS virtual host | No | `localhost` |
| `PDNS_API_TIMEOUT` | PowerDNS API request timeout in seconds | No | `10` |

### Verification

```bash
# Check operator status
kubectl get pods -n powerdns-operator-system

# Verify CRDs are installed
kubectl get crd | grep dns.cav.enablers.ob

# Test with a simple zone
kubectl apply -f - <<EOF
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterZone
metadata:
  name: test.example.com
spec:
  kind: Native
  nameservers:
    - ns1.test.example.com
    - ns2.test.example.com
EOF
```
