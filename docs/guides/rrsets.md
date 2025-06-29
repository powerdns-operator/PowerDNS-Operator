# RRset deployment

## Specification

The `RRset` specification contains the following fields:

| Field | Type | Required | Description |
| ----- | ---- |:--------:| ----------- |
| type | string | Y | Type of the record (e.g. "A", "PTR", "MX") |
| name | string | Y | Name of the record |
| ttl | uint32 | Y | DNS TTL of the records, in seconds
| records | []string | Y | All records in this Resource Record Set
| comment | string | N | Comment on RRSet |
| zoneRef | ZoneRef | Y | ZoneRef reference the zone the RRSet depends on |

The `ZoneRef` specification contains the following fields:

| Field | Type | Required | Description |
| ----- | ---- |:--------:| ----------- |
| name | string | Y | Name of the `ClusterZone`/`Zone` |
| kind | string | Y | Kind of zone (Zone/ClusterZone) |

## Example

```yaml
apiVersion: dns.cav.enablers.ob/v1alpha2
kind: RRset
metadata:
  name: test.helloworld.com
  namespace: default
spec:
  comment: nothing to tell
  type: A
  name: test
  ttl: 300
  records:
    - 1.1.1.1
    - 2.2.2.2
  zoneRef:
    name: helloworld.com
    kind: "Zone"
```

> Note: The name can be canonical or not. If not, the name of the `ClusterZone`/`Zone` will be appended

## Reconciliation Flow

The following diagram illustrates the reconciliation flow for RRset resources:

```mermaid
sequenceDiagram
    participant U as User
    participant K as Kubernetes API
    participant C as Controller
    participant P as PowerDNS API
    participant M as Metrics
    
    U->>K: kubectl apply rrset.yaml -n default
    K->>C: RRset Created Event (namespace-scoped)
    
    Note over C: Reconciliation Loop Starts
    C->>C: Check Deletion Timestamp
    
    alt Resource is being deleted
        C->>P: DELETE /api/v1/servers/localhost/zones/example.com/records
        P-->>C: RRset Deleted
        C->>C: Remove Finalizers
        C->>K: Update RRset
        Note over C: Deletion Complete
    else Resource is being created/updated
        C->>C: Add Finalizers if missing
        C->>K: Get Referenced Zone (Zone/ClusterZone)
        
        alt Zone not found
            C->>K: Set Pending Status
            C->>K: Set Zone Not Available Condition
            C->>M: Update Metrics
            Note over C: Requeue after 2s
        else Zone found
            C->>C: Check Zone Status
            
            alt Zone in Failed Status
                C->>K: Set Failed Status
                C->>K: Set Zone Unavailable Condition
                C->>M: Update Metrics
                Note over C: Reconciliation Failed
            else Zone Available
                C->>C: Check for duplicate RRsets (namespace-aware)
                
                alt Duplicate RRset found
                    C->>K: Set Failed Status
                    C->>K: Set Duplicated Condition
                    C->>M: Update Metrics
                    Note over C: Reconciliation Failed
                else No duplicates
                    C->>P: GET /api/v1/servers/localhost/zones/example.com/records
                    
                    alt RRset doesn't exist in PowerDNS
                        C->>P: POST /api/v1/servers/localhost/zones/example.com/records
                        Note over P: Create RRset with records
                        P-->>C: RRset Created Successfully
                    else RRset exists in PowerDNS
                        C->>C: Compare desired vs actual state
                        alt Differences found
                            C->>P: PATCH /api/v1/servers/localhost/zones/example.com/records
                            P-->>C: RRset Updated Successfully
                        end
                    end
                    
                    C->>K: Set Owner Reference
                    C->>K: Update RRset Status
                    C->>K: Set Available Condition
                    C->>M: Update Metrics
                    Note over C: Reconciliation Succeeded
                end
            end
        end
    end
    
    K-->>U: RRset Status Updated
```