# Testing Environment Setup

## Local Kubernetes with k3d

This guide shows how to set up a local testing environment using k3d. Other solutions like minikube, kind, or Talos can also be used.

## Prerequisites

- [k3d](https://k3d.io/stable/) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured
- [Docker](https://docs.docker.com/get-docker/) running

## Setup Steps

### 1. Create Local Registry (Optional)

```bash
k3d registry create registry.localhost --port 5000
```

### 2. Create Kubernetes Cluster

Create a 3-node cluster with the following features:

- Traefik ingress controller on port 18081
- CSI storage on `/mnt/k3d` for data persistence
- Private registry access configured

```bash
cat > ~/.k3d/k3d-cluster.yaml <<EOF
apiVersion: k3d.io/v1alpha5
kind: Simple
metadata:
  name: k3d
servers: 1
agents: 2
volumes:
  - volume: "/etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt"
  - volume: /mnt/k3d:/var/lib/rancher/k3s/storage
    nodeFilters:
      - server:0
      - agent:*
ports:
  - port: 18081:80
    nodeFilters:
      - loadbalancer
registries:
  use:
    - k3d-registry.localhost:5000
EOF

k3d cluster create --config ~/.k3d/k3d-cluster.yaml
```

### 3. Verify Setup

```bash
# Check cluster status
kubectl cluster-info

# Check nodes
kubectl get nodes

# Check ingress controller
kubectl get pods -n kube-system | grep traefik
```

## Next Steps

- [Install PowerDNS](https://github.com/powerdns-operator/powerdns-deployment)
- [Deploy PowerDNS Operator](../introduction/getting-started.md)
- [Test with examples](../../guides/clusterzones/)
