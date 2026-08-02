# Common Issues and Solutions

## Record Format Requirements

### Canonical Names

Some record types require canonical format (ending with a dot `.`):

#### CNAME Records
```yaml
--8<-- "rrset-cname.yaml"
```

#### PTR Records
```yaml
--8<-- "rrset-ptr.yaml"
```

#### MX Records
```yaml
--8<-- "rrset-mx.yaml"
```

#### SRV Records
```yaml
--8<-- "rrset-srv.yaml"
```

### TXT Records

TXT records must be properly quoted. If you see this error:

```
Parsing record content: Data field in DNS should start with quote (") at position 0
```

**Solution**: Ensure TXT records start and end with escaped quotes:

```yaml
--8<-- "rrset-txt.yaml"
```

## Common Error Scenarios

### Zone Conflicts
- **Error**: Zone shows "Failed" status with message "At least another ClusterZone/Zone exists with the same name"
- **Cause**: Multiple zones with the same FQDN
- **Solution**: Remove duplicate zones or use different names

### Missing Dependencies
- **Error**: RRset shows "Pending" status
- **Cause**: Referenced zone does not exist or is unhealthy
- **Solution**: Create the zone first or fix zone issues

### API Connectivity
- **Error**: Resources stuck in "Pending" status
- **Cause**: PowerDNS API unreachable or authentication failed
- **Solution**: Check API URL, key, and network connectivity

### Orphan Cleanup

- **Risk**: `--orphan-rrset-cleanup` / `--orphan-zone-cleanup` delete operator-marked PowerDNS resources that have no matching CR after a grace period. Normal CR deletion still removes the PDNS object immediately via finalizers; grace applies only to leftovers (no matching CR).
- **Cause**: Intentional opt-in cleanup when drift checking is enabled
- **Solution**: Leave cleanup off for detect-only (metrics/logs only). Start with a longer grace. Never set a user `spec.comment` to `powerdns-operator:orphan-since:…` (RRset marker; stripped on write). Zone grace uses metadata `X-POWERDNS-OPERATOR-ORPHAN-SINCE` = unix epoch seconds

See [Getting Started](../introduction/getting-started.md#operator-flags) for flags and [Metrics](metrics.md) for gauges/counters.

## Best Practices

1. **Use canonical names** for CNAME, PTR, MX, and SRV records
2. **Quote TXT records** properly with escaped quotes
3. **Create zones before records** to avoid dependency issues
4. **Check for duplicates** before creating resources
5. **Monitor metrics** for failed reconciliations
6. **Treat orphan cleanup as destructive** — enable only after reviewing orphan gauges/logs
