# Getting Started

## Prerequisites

For detailed prerequisites and compatibility information, see the [Stability and Support](stability-support.md) documentation.

## Installation

### Option 1: Helm Installation

Check out the PowerDNS Operator Helm chart repository [here](https://github.com/powerdns-operator/PowerDNS-Operator-helm-chart).

```bash
# Add the Helm repository
helm repo add powerdns-operator https://powerdns-operator.github.io/PowerDNS-Operator-helm-chart
helm repo update

# Install the latest operator release
helm install powerdns-operator powerdns-operator/powerdns-operator \
  --namespace powerdns-operator-system \
  --create-namespace \
  --set api.url=https://your-powerdns-server:8081 \
  --set credentials.data.PDNS_API_KEY=you-api-key
```

### Option 2: Direct Installation

!!! note "Custom Configuration"
    The bundle installation method installs the operator with default configuration. If you need to customize the operator configuration (e.g., modify resource limits, add sidecars, or change deployment settings), you'll need to patch the bundle using tools like Kustomize.

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

# Or, install specific version of the operator - replace v0.0.0 with your desired version
kubectl apply -f https://github.com/powerdns-operator/PowerDNS-Operator/releases/download/v0.0.0/bundle.yaml
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `PDNS_API_URL` | PowerDNS API server URL | Yes | None |
| `PDNS_API_KEY` | PowerDNS API authentication key | Yes | None |
| `PDNS_API_VHOST` | PowerDNS virtual host | No | `localhost` |
| `PDNS_API_TIMEOUT` | PowerDNS API request timeout in seconds | No | `10` |
| `PDNS_API_INSECURE` | Insecure connections with PowerDNS API | No | "False" |
| `PDNS_API_CA_PATH` | Path to Certificate Authority | No | None |

### Operator Flags

Pass these as container args on the manager Deployment (or via Helm values that map to them). High-level behavior: [FAQ](faq.md#does-the-operator-check-for-configuration-drift); metrics: [Metrics](../guides/metrics.md); cleanup risks: [Warnings](../guides/warnings.md).

| Flag | Default | Description |
|------|---------|-------------|
| `--drift-check-interval` | `0` | Periodic re-GET of PowerDNS and re-apply of Kubernetes desired state. Zero disables periodic drift/orphan scans (event-driven reconcile only). Example: `5m` |
| `--orphan-rrset-cleanup` | `false` | With interval > 0, flag orphan RRsets then delete them after `--orphan-rrset-grace` |
| `--orphan-rrset-grace` | `1h` | Wall-clock wait after an RRset is flagged before deletion |
| `--orphan-zone-cleanup` | `false` | With interval > 0, flag orphan zones then delete them after `--orphan-zone-grace` |
| `--orphan-zone-grace` | `1h` | Wall-clock wait after a zone is flagged before deletion |

Startup fails if either cleanup flag is set with `--drift-check-interval=0`, or if a cleanup flag is set with a non-positive matching grace.

Grace markers are stored in PowerDNS (stateless operator): RRset comment content `powerdns-operator:orphan-since:<unix-epoch-seconds>`; zone metadata kind `X-POWERDNS-OPERATOR-ORPHAN-SINCE` = unix epoch seconds. User `spec.comment` values that use that RRset prefix are stripped on write. Inventory list errors are fail-closed (no flag/delete). Orphan zone detection uses the PowerDNS zone list `account` field — zones whose list payload omits `account` are not flagged.

Example Deployment args for detect-only drift (no cleanup):

```yaml
args:
  - --leader-elect
  - --health-probe-bind-address=:8081
  - --metrics-bind-address=:8080
  - --drift-check-interval=5m
```

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
