# Etcd-disk

This flavor configures etcd to be on a separate disk from the OS disk.
By default it configures the size of the disk to be 10 GiB and sets
the `quota-backend-bytes` to `8589934592` (8 GiB) per recommendation from
[the etcd documentation](https://etcd.io/docs/latest/dev-guide/limit/#storage-size-limit).

## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |

## Prerequisites
[Quickstart](../getting-started.md) completed

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure akamai-linode \
        --flavor etcd-disk > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
