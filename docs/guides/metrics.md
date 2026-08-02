# Metrics

The PowerDNS Operator exposes Prometheus metrics for monitoring and observability.

## Available Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `clusterzones_status` | gauge | ClusterZone status | `name`, `status` |
| `zones_status` | gauge | Zone status | `name`, `namespace`, `status` |
| `clusterrrsets_status` | gauge | ClusterRRset status | `fqdn`, `name`, `status`, `type` |
| `rrsets_status` | gauge | RRset status | `fqdn`, `name`, `namespace`, `status`, `type` |
| `powerdns_operator_managed_corrections_total` | counter | PowerDNS rewrites after managed identity mismatch | `kind` (`zone`, `ns`, `rrset`) |
| `powerdns_operator_orphan_rrsets` | gauge | Operator-marked PDNS RRsets with no matching Kubernetes CR | `zone` |
| `powerdns_operator_orphan_zones` | gauge | Operator-marked PDNS zones with no matching Zone/ClusterZone CR | _(none)_ |
| `powerdns_operator_orphan_deletions_total` | counter | Orphan PDNS resources deleted after grace | `kind` (`rrset`, `zone`) |

## Status Values

- **`Succeeded`**: Resource successfully reconciled
- **`Failed`**: Resource reconciliation failed
- **`Pending`**: Resource waiting for dependencies

## Drift and orphans

- **`powerdns_operator_managed_corrections_total`**: Incremented when an existing PowerDNS zone, apex NS, or RRset does not match the CR and is rewritten. Initial creates are not counted. Label `kind` is `zone`, `ns`, or `rrset`.
- **`powerdns_operator_orphan_rrsets`**: Set during zone reconcile when `--drift-check-interval` is enabled. Counts operator-marked RRsets in that zone with no matching RRset/ClusterRRset CR (SOA and zone-owned apex NS are excluded).
- **`powerdns_operator_orphan_zones`**: Set by a periodic scan when `--drift-check-interval` is enabled. Counts operator-marked PDNS zones with no matching Zone/ClusterZone CR.
- **`powerdns_operator_orphan_deletions_total`**: Incremented when opt-in orphan cleanup deletes an RRset or zone after its grace marker ages out (`--orphan-rrset-cleanup` / `--orphan-zone-cleanup`). Label `kind` is `rrset` or `zone`.

Detect runs whenever the drift interval is set. Cleanup is off by default; see the [FAQ](../introduction/faq.md#does-the-operator-check-for-configuration-drift) and [Getting Started](../introduction/getting-started.md#operator-flags).

## Example Metrics

Based on the [example configuration](../introduction/overview/#resource-model):

```prometheus
# Cluster zones
clusterzones_status{name="example.org",status="Succeeded"} 1

# Cluster records
clusterrrsets_status{fqdn="example.org.",name="mx.example.org",status="Succeeded",type="MX"} 1
clusterrrsets_status{fqdn="example.org.",name="soa.example.org",status="Succeeded",type="SOA"} 1
clusterrrsets_status{fqdn="ns1.example.org.",name="ns1.example.org",status="Succeeded",type="A"} 1
clusterrrsets_status{fqdn="ns2.example.org.",name="ns2.example.org",status="Succeeded",type="A"} 1

# Namespace zones
zones_status{name="myapp1.example.org",namespace="myapp1",status="Succeeded"} 1

# Namespace records
rrsets_status{fqdn="myapp1.example.org.",name="soa.myapp1.example.org",namespace="myapp1",status="Succeeded",type="SOA"} 1
rrsets_status{fqdn="front.myapp1.example.org.",name="front.myapp1.example.org",namespace="myapp1",status="Succeeded",type="A"} 1

# Drift / orphans (when --drift-check-interval is enabled)
powerdns_operator_managed_corrections_total{kind="rrset"} 2
powerdns_operator_orphan_rrsets{zone="myapp1.example.org."} 0
powerdns_operator_orphan_zones 0
powerdns_operator_orphan_deletions_total{kind="rrset"} 0
```

## Monitoring Setup

### ServiceMonitor

When using Prometheus Operator, the operator can be monitored using a ServiceMonitor resource. This is the recommended approach in Kubernetes environments:

!!! tip "Helm Chart Integration"
    If you're using the Helm chart, ServiceMonitor creation can be enabled with:
    ```yaml
    metrics:
      serviceMonitor:
        enabled: true
    ```

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: powerdns-operator-metrics
  namespace: powerdns-operator-system
spec:
  endpoints:
  - interval: 15s
    port: http-metrics
    scheme: http
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - powerdns-operator-system
  selector:
    matchLabels:
      control-plane: controller-manager
```

### Grafana Dashboard (WIP)

!!! note "Coming Soon"
    Grafana dashboards for PowerDNS Operator metrics will be available in a future release. These dashboards will provide pre-configured visualizations for monitoring DNS zone and record status, reconciliation metrics, and operator performance.
