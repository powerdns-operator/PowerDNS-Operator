# FAQ

## General Questions

### Can I use PowerDNS-Admin and PowerDNS Operator together?

**Yes, but with caution.** The operator only supports the official PowerDNS API, while PowerDNS-Admin implements its own custom API. Both can coexist, but avoid managing the same resources with both tools to prevent conflicts.

### Can I manage multiple PowerDNS servers with a single operator?

**No.** The operator is designed to manage a single PowerDNS server. For multiple servers, deploy separate operator instances in different clusters.

### Does the operator check for configuration drift?

**No.** The operator only reconciles on Kubernetes events (create, update, delete). It does not periodically check for drift between Kubernetes resources and PowerDNS. This feature may be added in future versions.

## Technical Questions

### What happens if I delete a zone that has records?

The operator will delete the zone from PowerDNS, which removes all records in that zone. Additionally, due to Kubernetes owner references, all RRSets and ClusterRRSets that reference the deleted zone will be automatically deleted from Kubernetes as well. This cascading deletion ensures that orphaned records don't remain in the cluster.

### Can I use the operator with PowerDNS Recursor?

**No.** The operator only works with PowerDNS Authoritative Server. The Recursor does not have the same API for zone and record management.

### How do I handle DNS propagation delays?

The operator manages the PowerDNS server configuration but does not control DNS propagation. Consider TTL values and upstream DNS server configurations for propagation timing.

## Troubleshooting

### My zone shows "Failed" status

Check for:
- Duplicate zones with the same FQDN
- PowerDNS API connectivity issues
- Invalid zone configuration (nameservers, etc.)
- PowerDNS Operator logs

### My records are not being created

Check for:
- Referenced zone exists and is healthy
- No duplicate records with the same name and type
- PowerDNS API permissions
- Record format (especially for CNAME, MX, SRV records)
- PowerDNS Operator logs
