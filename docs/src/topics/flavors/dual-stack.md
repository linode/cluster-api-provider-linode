# Dual-Stack

This flavor enables allocating both IPv4 and IPv6 ranges to nodes within k8s cluster. This flavor disables nodeipam controller within kube-controller-manager and uses CCM specific nodeipam controller to allocate CIDRs to Nodes. IPv6 ranges are allocated to VPC, Subnets and Nodes attached to those subnets.  Pods get both ipv4 and ipv6 addresses.

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm, k3s            | Cilium | Ubuntu 22.04 | No                    | Yes  | Yes  |

## Prerequisites
[Quickstart](../getting-started.md) completed
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.33.4 \
        --infrastructure linode-linode \
        --flavor <controlplane>-dual-stack > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
