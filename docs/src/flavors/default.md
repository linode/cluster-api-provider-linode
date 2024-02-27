# Default
## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass |
|---------------|--------|--------------|-----------------------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    |
## Prerequisites
[Quickstart](../topics/getting-started.md) completed
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster --infrastructure linode:0.0.0 --flavor clusterclass > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
