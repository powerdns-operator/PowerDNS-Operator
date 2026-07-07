# Development Setup

## Getting Started

### Prerequisites

For detailed prerequisites and compatibility information, see the [Stability and Support](docs/introduction/stability-support.md) documentation.

### Make Help

Run `make help` for more information on all potential `make` targets

### To develop localy

**Install the CRDs into the cluster:**

```sh
make install
```

**Run the controller locally and bind it to the PowerDNS API:**

```sh
export PDNS_API_URL=https://powerdns.example.local:8081
export PDNS_API_KEY=secret
export PDNS_API_VHOST=localhost
# And optionally
#export PDNS_API_CA_PATH="/tmp/caroot.crt"
#export PDNS_API_INSECURE=true 
make run
```

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/powerdns-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/powerdns-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Testing

### Unit / integration tests

These run against [envtest](https://book.kubebuilder.io/reference/envtest.html) with an
in-memory PowerDNS mock, so they need no external dependencies:

```sh
make test
```

### End-to-end (E2E) tests

The E2E suite spins up a real environment: a [Kind](https://kind.sigs.k8s.io/) cluster,
a real PowerDNS authoritative server deployed in-cluster, and the operator built from the
local sources. Each spec applies `Zone`/`RRset`/`ClusterZone`/`ClusterRRset` resources via
`kubectl` and then asserts the result directly against the PowerDNS HTTP API.

**Prerequisites:** a working Docker daemon, plus `kind` and `kubectl` in your `PATH`.

```sh
make test-e2e
```

This target:

1. creates the Kind cluster `powerdns-operator-test-e2e` (if missing),
2. deploys PowerDNS (`test/e2e/testdata/powerdns.yaml`) and the credentials secret,
3. builds the operator image, loads it into Kind, installs the CRDs and deploys the operator,
4. runs the Ginkgo specs (`go test -tags=e2e ./test/e2e/`),
5. deletes the Kind cluster.

To keep the cluster running after the tests (for debugging), set `KEEP_CLUSTER=true`:

```sh
make test-e2e KEEP_CLUSTER=true
```

You can then tear it down manually with:

```sh
make cleanup-test-e2e
```

#### Running against a specific PowerDNS version

By default the suite deploys a specific version of `powerdns/pdns-auth-XX:latest`. To run it against another PowerDNS version, set the `E2E_POWERDNS_IMAGE` environment variable to any published [PowerDNS authoritative image](https://hub.docker.com/u/powerdns):

```sh
# Run the E2E suite against PowerDNS 5.0
E2E_POWERDNS_IMAGE=powerdns/pdns-auth-50:latest make test-e2e
```

In CI, the `e2e-tests` job runs a matrix across the supported PowerDNS versions, each as its own check (e.g. `e2e-tests (PowerDNS X.Y)`). Add or remove versions by editing the `powerdns` matrix in `.github/workflows/ci.yml`.

> **NOTE**: In CI, the E2E suite runs nightly, on demand (`workflow_dispatch`), or on pull
> requests carrying the `run-e2e` label (see `.github/workflows/e2e.yml`).

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/powerdns-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/powerdns-operator/powerdns-operator/<tag or branch>/dist/install.yaml
```