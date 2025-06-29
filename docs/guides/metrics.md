# Metrics

The PowerDNS Operator exposes Prometheus metrics for monitoring and observability.

## Available Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `clusterzones_status` | gauge | ClusterZone status | `name`, `status` |
| `zones_status` | gauge | Zone status | `name`, `namespace`, `status` |
| `clusterrrsets_status` | gauge | ClusterRRset status | `fqdn`, `name`, `status`, `type` |
| `rrsets_status` | gauge | RRset status | `fqdn`, `name`, `namespace`, `status`, `type` |

## Status Values

- **`Succeeded`**: Resource successfully reconciled
- **`Failed`**: Resource reconciliation failed
- **`Pending`**: Resource waiting for dependencies

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
