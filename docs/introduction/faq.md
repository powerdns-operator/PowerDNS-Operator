# FAQ

## General Questions

### Can I use PowerDNS-Admin and PowerDNS Operator together?

**Yes, but with caution.** The operator only supports the official PowerDNS API, while PowerDNS-Admin implements its own custom API. Both can coexist, but avoid managing the same resources with both tools to prevent conflicts.

### Can I manage multiple PowerDNS servers with a single operator?

**No.** The operator is designed to manage a single PowerDNS server. For multiple servers, deploy separate operator instances in different clusters.

### Does the operator check for configuration drift?

**Optionally.** By default (`--drift-check-interval=0`), the operator only reconciles on Kubernetes events (create, update, delete). Set a non-zero interval (for example `5m`) to periodically re-GET PowerDNS, re-apply the Kubernetes desired state for managed resources, and retry sticky `Failed` resources.

When drift checking is enabled, the operator also **detects** orphans (operator-marked PowerDNS zones/RRsets with no matching CR) via logs and [metrics](../guides/metrics.md). It does **not** delete them unless orphan cleanup is explicitly enabled — that is opt-in and destructive; see [Warnings](../guides/warnings.md).

Flag defaults and Deployment examples: [Getting Started](getting-started.md#operator-flags).

### How does the operator mark resources it manages in PowerDNS?

Every Zone the operator writes gets `account` set to `powerdns-operator`. Every RRset (including Zone-managed NS records) gets a comment with `account` set to `powerdns-operator` (comment content is the CR `spec.comment`, or empty when unset). After an upgrade, existing unmarked resources are rewritten on their next reconcile (or on the next drift-check interval if enabled).

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

With `--drift-check-interval=0` (default), sticky `Failed` resources are not re-tried against PowerDNS until the Spec changes. Enabling a non-zero interval retries them on that schedule.

### My records are not being created

Check for:
- Referenced zone exists and is healthy
- No duplicate records with the same name and type
- PowerDNS API permissions
- Record format (especially for CNAME, MX, SRV records)
- PowerDNS Operator logs
