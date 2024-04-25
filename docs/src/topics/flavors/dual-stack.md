# Dual-Stack
## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes  | Yes  |
## Prerequisites
[Quickstart](../getting-started.md) completed
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --flavor dual-stack > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
