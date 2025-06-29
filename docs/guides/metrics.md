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

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'powerdns-operator'
    static_configs:
      - targets: ['powerdns-operator-controller-manager:8080']
    metrics_path: /metrics
```

### Grafana Dashboard

Create alerts for:
- Resources in `Failed` status
- Resources stuck in `Pending` status
- High failure rates

### Example Queries

```prometheus
# Failed resources count
sum(clusterzones_status{status="Failed"}) + sum(zones_status{status="Failed"})

# Success rate
sum(clusterzones_status{status="Succeeded"}) / sum(clusterzones_status)
```
