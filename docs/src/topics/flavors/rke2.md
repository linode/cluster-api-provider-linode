# RKE2

This flavor uses RKE2 for the kubernetes distribution. By default it configures the cluster
with the [CIS profile](https://docs.rke2.io/security/hardening_guide#rke2-configuration):
> Using the generic cis profile will ensure that the cluster passes the CIS benchmark (rke2-cis-1.XX-profile-hardened) associated with the Kubernetes version that RKE2 is running. For example, RKE2 v1.28.XX with the profile: cis will pass the rke2-cis-1.7-profile-hardened in Rancher.

```admonish warning
Until [this upstream PR](https://github.com/rancher-sandbox/cluster-api-provider-rke2/pull/301) is merged, CIS profile enabling
will not work for RKE2 versions >= v1.29.
```

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
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1+rke2r1 \
        --infrastructure akamai-linode \
        --flavor rke2 > test-rke2-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-rke2-cluster.yaml
    ```
