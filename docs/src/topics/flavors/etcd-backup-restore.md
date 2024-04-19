# etcd-backup-restore

## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass | Installs etcd backup | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes                  | Yes  | No   |

## Prerequisites
[Quickstart](../getting-started.md) completed
## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure akamai-linode \
        --flavor etcd-backup-restore > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```


## Notes
This flavor is identical to the default flavor with the addon etcd-backup-restore enabled

## Usage
Refer [backups.md](../backups.md)
