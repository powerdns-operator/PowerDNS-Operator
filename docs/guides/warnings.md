# Warnings on field format

## Deal with canonical names

For some resources such as CNAME, PTR, MX, SRV, the records field MUST be in canonical format (end with a dot "."). See following examples.

### CNAME

```yaml
--8<-- "rrset-cname.yaml"
```

### PTR

```yaml
--8<-- "rrset-ptr.yaml"
```

### MX

```yaml
--8<-- "rrset-mx.yaml"
```

### SRV

```yaml
--8<-- "rrset-srv.yaml"
```

## TXT Records

Sometime, you may encounter the following error when applying a `RRset` custom resource:
```yaml
status:
  syncErrorDescription: 'Record helloworld.com./TXT ''Welcome to the helloworld.com
    domain'': Parsing record content (try ''pdnsutil check-zone''): Data field in
    DNS should start with quote (") at position 0 of ''Welcome to the helloworld.com
    domain'''
  syncStatus: Failed
```

This error is due to a wrong format for the `RRset`.  
TXT records MUST start AND end with an escaped quote (\"). See following example.  

```yaml
--8<-- "rrset-txt.yaml"
```
