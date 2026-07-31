# Stability and Support

## Version Compatibility

| Component | Supported Versions |
|-----------|-------------------|
| **PowerDNS Authoritative** | 4.9, 5.0 |
| **Kubernetes** | 1.33, 1.34, 1.35 |
| **Go** (for development) | 1.26+ |

## Breaking Changes

### Breaking changes introduced in v0.7.x versions

* `rrset.spec.zoneRef` and `clusterrrset.spec.zoneRef` are now immutable, `kind` and `name` alike

Existing resources remain valid and are not invalidated: the rule is only evaluated on
update. However, any GitOps pipeline that rewrote `zoneRef` in place will now be rejected
by the API server. Moving a record to another zone requires deleting and recreating the
`RRset`/`ClusterRRset` — see [Common Issues and Solutions](../guides/warnings.md#immutable-zoneref).

### Breaking changes introduced in v0.4.x versions

We noticed lacks of security and delegation possibilities with <=v0.3.x versions, so we decided to split previous `Zone` in 2 differents Custom Resources: 

* `ClusterZone` (cluster-wide resource)
* `Zone` (namespaced resource)

This decision introduces breaking changes

* `Zone` was previously cluster-wide resource become namespace-scoped
* `rrset.spec.zoneRef.kind` is a new mandatory field to indicate whereas the `RRset` depends on a `Zone` or a `ClusterZone`
* `rrset.status.syncErrorDescription` is replaced by a `Status.Condition` field as adviced by the community[^1][^2]

[^1]: https://heidloff.net/article/storing-state-status-kubernetes-resources-conditions-operators-go/
[^2]: https://maelvls.dev/kubernetes-conditions/