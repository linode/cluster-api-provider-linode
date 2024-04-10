# etcd-backup-restore

## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass | Installs etcd backup | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes                  | Yes  | No   |

## Prerequisites
[Quickstart](../topics/getting-started.md) completed

## Notes
This flavor is identical to the default flavor with the addon etcd-backup-restore enabled

## Usage
Refer [backups.md](../backups.md)
