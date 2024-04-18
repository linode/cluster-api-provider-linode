# VPCLess

This flavor supports provisioning k8s clusters outside of VPC. It uses kubeadm for
setting up control plane and uses cilium with VXLAN for pod networking.

## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |
## Prerequisites
[Quickstart](../getting-started.md) completed

## Notes
This flavor is identical to the default flavor with the exception that it provisions
k8s clusters without VPC. Since it runs outside of VPC, native routing is not
supported in this flavor and it uses VXLAN for pod to pod communication.

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --infrastructure linode:0.0.0 \
        --flavor vpcless > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
