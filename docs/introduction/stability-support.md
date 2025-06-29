# Stability and Support

## Version Compatibility

| Component | Supported Versions |
|-----------|-------------------|
| **PowerDNS Authoritative** | 4.7, 4.8, 4.9 |
| **Kubernetes** | 1.31, 1.32, 1.33 |
| **Go** (for development) | 1.24+ |

## Breaking Changes

### v0.4.x Breaking Changes

Version 0.4.x introduced security and delegation improvements by splitting the previous `Zone` resource into two separate resources:

#### Changes Made
- **`Zone`**: Changed from cluster-wide to namespace-scoped resource
- **`ClusterZone`**: New cluster-wide resource for platform-level zones
- **`zoneRef.kind`**: New mandatory field in RRset resources
- **Status Conditions**: Replaced `syncErrorDescription` with standard Kubernetes conditions

#### Migration Impact
- Existing `Zone` resources need to be migrated to `ClusterZone` or updated with namespace
- All `RRset` resources need the new `zoneRef.kind` field
- Status field structure changed to use conditions

!!! warning "Migration Required"
    If upgrading from v0.3.x or earlier, review the migration guide and update your resources accordingly.

## Support Policy

- **Security Updates**: Backported to supported versions
- **Bug Fixes**: Applied to current and previous minor versions
- **New Features**: Only in current major version
- **Deprecation**: Announced 2 versions in advance

## Community Support

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For questions and community help
- **Documentation**: Comprehensive guides and examples available