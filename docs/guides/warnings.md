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
- **Error**: Zone shows "Failed" status with "Already existing Zone" message
- **Cause**: Multiple zones with the same FQDN
- **Solution**: Remove duplicate zones or use different names

### Missing Dependencies
- **Error**: RRset shows "Pending" status
- **Cause**: Referenced zone does not exist or is unhealthy
- **Solution**: Create the zone first or fix zone issues

### Immutable zoneRef
- **Error**: `spec.zoneRef: Invalid value: "object": Value is immutable`
- **Cause**: An attempt to change `spec.zoneRef` (its `name`, its `kind`, or both) on
  an existing `RRset`/`ClusterRRset`. The field is rejected by the API server, so the
  resource is never left in an inconsistent state.
- **Why**: `zoneRef` designates the parent zone that owns the record. Changing it is not
  a record update but a change of parent: it would move the Kubernetes owner reference
  (and therefore which zone's deletion garbage-collects the resource), and it would
  require writing into two different PowerDNS zones. The PowerDNS API applies changes
  atomically **per zone**, so a cross-zone move cannot be rolled back if its second half
  fails — the operator would leave an orphaned record behind.
- **Solution**: Delete the `RRset`/`ClusterRRset` and recreate it with the new `zoneRef`.
  Be aware this briefly interrupts resolution for that record: the operator removes the
  record from the old zone on deletion, and only creates it in the new zone once the new
  resource is reconciled.
- **Note**: Migrating a zone from `Zone` to `ClusterZone` (or the reverse) keeping the
  same name also requires recreating its records, since `zoneRef.kind` is frozen too.

Changing `type`, `records`, `ttl` or `comment` does **not** require recreation. Those
stay within a single zone, and the operator handles whatever is needed on the PowerDNS
side.

### API Connectivity
- **Error**: Resources stuck in "Pending" status
- **Cause**: PowerDNS API unreachable or authentication failed
- **Solution**: Check API URL, key, and network connectivity

## Best Practices

1. **Use canonical names** for CNAME, PTR, MX, and SRV records
2. **Quote TXT records** properly with escaped quotes
3. **Create zones before records** to avoid dependency issues
4. **Check for duplicates** before creating resources
5. **Monitor metrics** for failed reconciliations
