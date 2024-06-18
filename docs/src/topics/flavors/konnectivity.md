# Konnectivity

This flavor supports provisioning k8s clusters with konnectivity configured.It uses kubeadm
for setting up control plane and uses cilium with native routing for pod networking.

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm                 | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |

## Prerequisites
[Quickstart](../getting-started.md) completed

## Notes
This flavor configures apiserver with konnectivity. Traffic from apiserver to cluster flows
over the tunnels created between konnectivity-server and konnectivity-agent.

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --infrastructure linode-linode \
        --flavor <controlplane>-konnectivity > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
