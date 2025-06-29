# Overview

## Architecture

![architecture](./../assets/architecture.png)

The PowerDNS Operator extends Kubernetes with Custom Resource Definitions (CRDs) to manage PowerDNS zones and records declaratively. The operator watches for changes to these resources and reconciles them with the PowerDNS API.

## Resource Model

![example](./../assets/example.png)

The operator supports four main resource types:

### 1. ClusterZone (Cluster-wide)
Platform teams create cluster-wide zones that are available across all namespaces.

```yaml
--8<-- "clusterzone-example.org.yaml"
```

### 2. ClusterRRset (Cluster-wide)
Platform teams create cluster-wide records for infrastructure services.

```yaml
--8<-- "clusterrrsets-example.org.yaml"
```

### 3. Zone (Namespace-scoped)
Application teams create namespace-scoped zones for their applications.

```yaml
--8<-- "zone-myapp1.example.org.yaml"
```

### 4. RRset (Namespace-scoped)
Application teams create records for their application services.

```yaml
--8<-- "rrsets-myapp1.example.org.yaml"
```

## Reconciliation Process

### Zone Reconciliation
1. **Duplicate Check**: Verify no other zone exists with the same FQDN
2. **API Operation**: Create or modify zone via PowerDNS API
3. **NS Records**: Create or modify nameserver entries
4. **Status Update**: Update resource status and metrics

### Record Reconciliation
1. **Zone Dependency**: Verify the referenced zone exists and is healthy
2. **Duplicate Check**: Verify no duplicate records exist
3. **API Operation**: Create or modify record via PowerDNS API
4. **Owner Reference**: Set ownership relationship
5. **Status Update**: Update resource status and metrics

## Roles and Responsibilities

| Role | Responsibilities | Resources |
|------|------------------|-----------|
| **Cluster Operator** | Configure PowerDNS instance and operator | Infrastructure setup |
| **Platform Team** | Define architecture and permissions | ClusterZone, Zone, ClusterRRset |
| **Application Team** | Manage application DNS records | RRset, Zone (namespace-scoped) |

!!! note "RBAC Integration"
    Each role maps to Kubernetes RBAC roles. The operator provides granular permissions for different use cases.