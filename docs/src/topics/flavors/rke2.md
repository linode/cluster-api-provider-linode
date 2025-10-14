# RKE2

This flavor uses RKE2 for the kubernetes distribution.

## Specification
| Control Plane                 | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------------|--------|--------------|-----------------------|------|------|
| [rke2](https://docs.rke2.io/) | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |
## Prerequisites
* [Quickstart](../getting-started.md) completed
* Select an [rke2 kubernetes version](https://github.com/rancher/rke2/releases) to set for the kubernetes version
* Installed [rke2 bootstrap provider](https://github.com/rancher-sandbox/cluster-api-provider-rke2) into your management cluster
  ```shell
  clusterctl init --bootstrap rke2 --control-plane rke2
  ```

### CIS Hardening
The default configuration does not enable [CIS hardening](https://docs.rke2.io/security/hardening_guide#rke2-configuration).
To enable this, set the following variables:
```bash
export CIS_PROFILE=cis
export CIS_ENABLED=true
```

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.33.4+rke2r1 \
        --infrastructure linode-linode \
        --flavor rke2 > test-rke2-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-rke2-cluster.yaml
    ```

