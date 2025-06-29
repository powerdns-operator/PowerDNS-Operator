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
