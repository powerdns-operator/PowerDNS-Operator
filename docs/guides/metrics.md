# Metrics

PowerDNS-Operator exposes metrics in Prometheus format.  

| Name | Type | Description | Available labels |
| ---- | ---- | ----------- | ---------------- |
| clusterzones_status | gauge | Statuses of ClusterZones processed | name, status |
| zones_status        | gauge | Statuses of Zones processed        | name, namespace ,status |
| rrsets_status       | gauge | Statuses of RRsets processed       | fqdn, name, namespace, status, type |

## Example

The following metrics are based on the example defined [here](/powerdns-operator/introduction/overview/#resource-model)

```
clusterzones_status{name="example.org",status="Succeeded"} 1
rrsets_status{fqdn="example.org.",name="mx.example.org",namespace="example",status="Succeeded",type="MX"} 1
rrsets_status{fqdn="example.org.",name="soa.example.org",namespace="example",status="Succeeded",type="SOA"} 1
rrsets_status{fqdn="ns1.example.org.",name="ns1.example.org",namespace="example",status="Succeeded",type="A"} 1
rrsets_status{fqdn="ns2.example.org.",name="ns2.example.org",namespace="example",status="Succeeded",type="A"} 1

zones_status{name="myapp1.example.org",namespace="myapp1",status="Succeeded"} 1
rrsets_status{fqdn="myapp1.example.org.",name="soa.myapp1.example.org",namespace="myapp1",status="Succeeded",type="SOA"} 1
rrsets_status{fqdn="front.myapp1.example.org.",name="front.myapp1.example.org",namespace="myapp1",status="Succeeded",type="A"} 1
```
