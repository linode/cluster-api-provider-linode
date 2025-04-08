# Node IPAM using CCM

This flavor enables linode-cloud-controller-manager to perform nodeipam allocation. Nodeipam controller is disabled within kube-controller-manager and is enabled within CCM.

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm                 | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |

## Prerequisites
[Quickstart](../getting-started.md) completed

## Notes
This flavor is identical to the default flavor with the exception that it disables nodeipam controller within kube-controller-manager and uses nodeipam controller within CCM to allocate pod cidrs to nodes.

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --infrastructure linode-linode \
        --flavor <controlplane>-nodeipam-ccm > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
