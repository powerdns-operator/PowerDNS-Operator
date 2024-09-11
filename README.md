# PowerDNS-Operator

**This project is a work in progress and is not yet ready for production use.**

This project is a Kubernetes operator for PowerDNS.

It provides a way to manage PowerDNS resources in a Kubernetes cluster using Custom Resources.

## Requirements

#### Tested PowerDNS versions

Supported versions of PowerDNS Authoritative Server ("API v1"):

- 4.7
- 4.8
- 4.9

It may work on other versions, but it has not been tested.

#### Tested Kubernetes versions

- 1.29
- 1.30
- 1.31

It may work on other versions, but it has not been tested.

## Quick Start

### Installation

To install the operator, run the following commands to setup the PowerDNS configuration:

```sh
kubectl create namespace powerdns-operator-system
```

```sh
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: powerdns-operator-manager
  namespace: powerdns-operator-system
type: Opaque
stringData:
  PDNS_API_URL: https://powerdns.example.local:8081
  PDNS_API_KEY: secret
  PDNS_API_VHOST: localhost
EOF
```

Then, install the latest (or change `main` to the disired `tag`) operator using the following command:

```sh
kubectl apply -f https://raw.githubusercontent.com/orange-opensource/powerdns-operator/main/dist/install.yaml
```

### Usage

**Keep in mind that `Zone` are cluster-wide and `RRSet` are namespace scoped.**

Zone is a critical resource and may be managed by a dedicated team, while RRSet may be managed by the application team.

In either case, you can apply your own RBAC rules to restrict access to the resources.

To create a PowerDNS resource, you can use the following examples.

#### Zone

First, create a Zone resource.

```yaml
---
apiVersion: dns.cav.enablers.ob/v1alpha1
kind: Zone
metadata:
  name: example.com
spec:
  kind: Native
  nameservers:
    - ns1.example.com
    - ns2.example.com
```

#### RRSet

Then, you can create RRSets and reference the target Zone.

```yaml
---
apiVersion: dns.cav.enablers.ob/v1alpha1
kind: RRset
metadata:
  name: a.example.com
  namespace: default
spec:
  comment: nothing to tell
  type: A
  ttl: 300
  records:
    - 1.1.1.1
    - 8.8.8.8
  zoneRef:
    name: example.com

---
apiVersion: dns.cav.enablers.ob/v1alpha1
kind: RRset
metadata:
  name: cname.example.com
  namespace: default
spec:
  type: CNAME
  ttl: 300
  records:
    - a.example.com
  zoneRef:
    name: example.com
```

The operator will manage the lifecycle of the resources and update the PowerDNS server accordingly.
  * If you update the resources, the operator will update the PowerDNS server accordingly.
  * If you delete the resources, the operator will delete the resources from PowerDNS.

Check the results

```sh
kubectl get zones,rrsets -o wide

NAME                                   SERIAL       ID
zone.dns.cav.enablers.ob/example.com   2024081304   example.com.

NAME                                          ZONE           TYPE    TTL   RECORDS
rrset.dns.cav.enablers.ob/a.example.com       example.com.   A       300   ["1.1.1.1","8.8.8.8"]
rrset.dns.cav.enablers.ob/cname.example.com   example.com.   CNAME   300   ["a.example.com"]
```

Test the DNS resolution

```sh
dig @resolver_ip cname.example.com +short
a.example.com.
8.8.8.8
1.1.1.1
```

## Contributing

If you'd like to contribute to the project, refer to the [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See the [LICENSE](LICENSE) file for licensing information.
