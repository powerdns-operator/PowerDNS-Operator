# Stability and Support

## Breaking changes introduced in v0.4.x versions

We noticed lacks of security and delegation possibilities with <=v0.3.x versions, so we decided to split previous `Zone` in 2 differents Custom Resources: 

* `ClusterZone` (cluster-wide resource)
* `Zone` (namespaced resource)

This decision introduces breaking changes

* `Zone` was previously cluster-wide resource become namespace-scoped
* `rrset.spec.zoneRef.kind` is a new mandatory field to indicate whereas the `RRset` depends on a `Zone` or a `ClusterZone`
* `rrset.status.syncErrorDescription` is replaced by a `Status.Condition` field as adviced by the community[^1][^2]

[^1]: https://heidloff.net/article/storing-state-status-kubernetes-resources-conditions-operators-go/
[^2]: https://maelvls.dev/kubernetes-conditions/