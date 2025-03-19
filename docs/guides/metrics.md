# Metrics

PowerDNS-Operator exposes metrics in Prometheus format.  

| Name | Type | Description | Available labels |
| ---- | ---- | ----------- | ---------------- |
| rrsets_status | gauge | Statuses of RRsets processed | fqdn, name, namespace, status, type |

## Example

```
rrsets_status{fqdn="myapp1.example.org.",name="soa.myapp1.example.org",namespace="myapp1",status="Succeeded",type="SOA"} 1
rrsets_status{fqdn="front.myapp1.example.org.",name="front.myapp1.example.org",namespace="myapp1",status="Succeeded",type="A"} 1
```
