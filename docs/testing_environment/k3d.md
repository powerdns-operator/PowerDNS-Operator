# Kubernetes cluster with k3d (https://k3d.io/stable/)

Other solutions such as Talos, minikube, kind, ... can be used

## Private registry

To create a private local registry:
```bash
k3d registry create registry.localhost --port 5000
```

## Kubernetes Cluster

The following configuration file is used to deploy a 3-node cluster for testing purposes with the below features:

* Traefik ingress controller on http port 18081
* CSI default Storage on the Host directory `/mnt/k3d` (Usefaul to persist mariadb data)
* Private registry access configured

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
```
```bash
k3d cluster create --config ~/.k3d/k3d-cluster.yaml
```
