# Flavors

This section contains information about supported flavors in Cluster API Provider Linode

In `clusterctl` the infrastructure provider authors can provide different types
of cluster templates referred to as "flavors". You can use the `--flavor` flag
to specify which flavor to use for a cluster, e.g:

```bash
clusterctl generate cluster test-cluster --flavor clusterclass-kubeadm
```

To use the default flavor, omit the `--flavor` flag.

See the [`clusterctl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors) for more information.

Supported CAPL flavors

| Control Plane  | Flavor                     | Notes                                                |
|----------------|----------------------------|------------------------------------------------------|
| kubeadm        | default                    | Installs Linode infra resources, kubeadm resources,  |
|                |                            | CNI, CSI driver, CCM and clusterresourceset          |
|                | kubeadm-cluster-autoscalar | Installs default along with the cluster autoscalar   |
|                |                            | add-on                                               |
|                | kubeadm-etcd-disk          | Installs default along with the disk configuration   |
|                |                            | for etcd disk                                        |
|                | kubeadm-etcd-backup-restore| Installs default along with etcd-backup-restore addon|
|                | kubeadm-vpcless            | Installs default without a VPC                       |
|                | kubeadm-dualstack          | Installs vpcless and enables IPv6 along with IPv4    |
|                | kubeadm-self-healing       | Installs default along with the machine-health-check |
|                |                            | add-on                                               |
|                | kubeadm-full               | Installs all non-vpcless based flavors combinations  |
|                | kubeadm-fullvpcless        | Installs all vpcless based flavors combinations      |
|----------------|----------------------------|------------------------------------------------------|
| k3s            | default                    | Installs Linode infra resources, k3s resources and   |
|                |                            | cilium network policies                              |
|                | kubeadm-cluster-autoscalar | Installs default along with the cluster autoscalar   |
|                |                            | add-on                                               |
|                | kubeadm-etcd-backup-restore| Installs default along with etcd-backup-restore addon|
|                | kubeadm-vpcless            | Installs default without a VPC                       |
|                | kubeadm-dualstack          | Installs vpcless and enables IPv6 along with IPv4    |
|                | kubeadm-self-healing       | Installs default along with the machine-health-check |
|                |                            | add-on                                               |
|                | kubeadm-full               | Installs all non-vpcless based flavors combinations  |
|                | kubeadm-fullvpcless        | Installs all vpcless based flavors combinations      |
|----------------|----------------------------|------------------------------------------------------|
| rke2           | default                    | Installs Linode infra resources, rke2 resources,     |
|                |                            | cilium and cilium network policies                   |
|                | kubeadm-cluster-autoscalar | Installs default along with the cluster autoscalar   |
|                |                            | add-on                                               |
|                | kubeadm-etcd-disk          | Installs default along with the disk configuration   |
|                |                            | for etcd disk                                        |
|                | kubeadm-etcd-backup-restore| Installs default along with etcd-backup-restore addon|
|                | kubeadm-vpcless            | Installs default without a VPC                       |
|                | kubeadm-self-healing       | Installs default along with the machine-health-check |
|                |                            | add-on                                               |
|                | kubeadm-full               | Installs all non-vpcless based flavors combinations  |
|                | kubeadm-fullvpcless        | Installs all vpcless based flavors combinations      |
|----------------|----------------------------|------------------------------------------------------|
