# K3s
## Specification
| Control Plane               | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-----------------------------|--------|--------------|-----------------------|------|------|
| [k3s](https://docs.k3s.io/) | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |
## Prerequisites
* [Quickstart](../getting-started.md) completed
* Select a [k3s kubernetes version](https://github.com/k3s-io/k3s/releases) to set for the kubernetes version
* Installed [k3s bootstrap provider](https://github.com/k3s-io/cluster-api-k3s) into your management cluster
  * Add the following to `~/.cluster-api/clusterctl.yaml` for the k3s bootstrap/control plane providers
    ```yaml
    providers:
      - name: "k3s"
        url: https://github.com/k3s-io/cluster-api-k3s/releases/latest/bootstrap-components.yaml
        type: "BootstrapProvider"
      - name: "k3s"
        url: https://github.com/k3s-io/cluster-api-k3s/releases/latest/control-plane-components.yaml
        type: "ControlPlaneProvider"
        
    ```
  * Install the k3s provider into your management cluster
    ```shell
    clusterctl init --bootstrap k3s --control-plane k3s
    ```
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.33.4+k3s2 \
        --infrastructure linode-linode \
        --flavor k3s > test-k3s-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-k3s-cluster.yaml
    ```
