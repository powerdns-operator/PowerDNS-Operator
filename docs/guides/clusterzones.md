# ClusterZone deployment

## Specification

The `ClusterZone` specification contains the following fields:

| Field | Type | Required | Description |
| ----- | ---- |:--------:| ----------- |
| kind | string | Y | Kind of the zone, one of "Native", "Master", "Slave", "Producer", "Consumer" |
| nameservers | []string | Y | List of the nameservers of the zone |
| catalog | string | N | The catalog this zone is a member of |
| soa_edit_api | string | N | The SOA-EDIT-API metadata item, one of "DEFAULT", "INCREASE", "EPOCH", defaults to "DEFAULT" |

## Example

```yaml
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: ClusterZone
metadata:
  name: helloworld.com
spec:
  nameservers:
    - ns1.helloworld.com
    - ns2.helloworld.com
  kind: Master
  catalog: catalog.helloworld
  soa_edit_api: EPOCH
```

## Reconciliation Flow

The following diagram illustrates the reconciliation flow for ClusterZone resources:

```mermaid
sequenceDiagram
    participant U as User
    participant K as Kubernetes API
    participant C as Controller
    participant P as PowerDNS API
    participant M as Metrics
    
    U->>K: kubectl apply clusterzone.yaml
    K->>C: ClusterZone Created Event
    
    Note over C: Reconciliation Loop Starts
    C->>C: Check Deletion Timestamp
    
    alt Resource is being deleted
        C->>P: DELETE /api/v1/servers/localhost/zones/example.com
        P-->>C: Zone Deleted
        C->>C: Remove Finalizers
        C->>K: Update ClusterZone
        Note over C: Deletion Complete
    else Resource is being created/updated
        C->>C: Add Finalizers if missing
        C->>C: Check for duplicate zones
        
        alt Duplicate zone found
            C->>K: Set Failed Status
            C->>K: Set Duplicated Condition
            C->>M: Update Metrics
            Note over C: Reconciliation Failed
        else No duplicates
            C->>P: GET /api/v1/servers/localhost/zones/example.com
            
            alt Zone doesn't exist in PowerDNS
                C->>P: POST /api/v1/servers/localhost/zones
                Note over P: Create Zone with NS records
                P-->>C: Zone Created Successfully
            else Zone exists in PowerDNS
                C->>C: Compare desired vs actual state
                alt Differences found
                    C->>P: PATCH /api/v1/servers/localhost/zones/example.com
                    P-->>C: Zone Updated Successfully
                end
            end
            
            C->>K: Update ClusterZone Status
            C->>K: Set Available Condition
            C->>M: Update Metrics
            Note over C: Reconciliation Succeeded
        end
    end
    
    K-->>U: ClusterZone Status Updated
```
