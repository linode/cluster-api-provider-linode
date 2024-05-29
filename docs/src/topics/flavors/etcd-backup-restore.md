# etcd-backup-restore

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm, k3s, rke2      | Cilium | Ubuntu 22.04 | No                    | Yes  | Yes  |

## Prerequisites
[Quickstart](../getting-started.md) completed
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --flavor <controlplane>-etcd-backup-restore > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```


## Notes
This flavor is identical to the default flavor with the addon etcd-backup-restore enabled

## Usage
Refer [backups.md](../backups.md)
