# RKE2
## Specification
| Control Plane               | CNI    | Default OS   | Installs ClusterClass |
|-----------------------------|--------|--------------|-----------------------|
| [rke2](https://docs.rke2.io/) | Cilium | Ubuntu 22.04 | No                    |
## Prerequisites
* [Quickstart](../topics/getting-started.md) completed
* Select an [rke2 kubernetes version](https://github.com/rancher/rke2/releases) to set for the kubernetes version
  ```bash
  export RKE2_KUBERNETES_VERSION=v1.29.1+rke2r1
  ```
* Installed [rke2 bootstrap provider](https://github.com/rancher-sandbox/cluster-api-provider-rke2) into your management cluster
  ```shell
  clusterctl init --bootstrap rke2 --control-plane rke2
  ```
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster --infrastructure linode:0.0.0 --flavor rke2 > test-rke2-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-rke2-cluster.yaml
    ```
