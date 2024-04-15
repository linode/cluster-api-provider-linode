# Data Disks
This section describes how to specify additional data disks for a linode instance. These disks can use devices `sdb` through `sdh` 
for a total of 7 disks. The total size of these disks + the OS Disk cannot exceed the linode plan size.

```admonish warning
Currently SDB is being used by a swap disk, replacing this disk with a data disk will slow down linode creation by
up to 90 seconds. This will be resolved when the disk creation refactor is finished in https://github.com/linode/cluster-api-provider-linode/pull/216
```
## Specify a data disk
A LinodeMachine can be configured with additional data disks with the key being the device to be mounted as and including an optional label and sizeGB.
* `sizeGB` Required field. The size in GB to use for a data disk. The sum of all data disks must not be more than allowed by the linode plan. 
* `label`  Optional field. The label for the disk, defaults to the device name
* `DeviceID` Optional field used by the controller to track drive IDs, this should not be set unless a drive is created outside of CAPL

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: ${CLUSTER}-control-plane
spec:
  template:
    spec:
      region: us-ord
      type: g6-standard-4
      dataDisks:
        sdc:
          label: etcd_disk
          sizeGB: 16
        sdd:
          label: data_disk
          sizeGB: 10
```

## Use a data disk for an explicit etcd data disk
The following configuration can be used to configure a separate disk for etcd data on control plane nodes.
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: ${CLUSTER}-control-plane
spec:
  template:
    spec:
      region: us-ord
      type: g6-standard-4
      dataDisks:
        sdc:
          label: etcd_disk
          sizeGB: 16

---
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
    [...]
    diskSetup:
      filesystems:
        - label: etcd_data
          filesystem: ext4
          device: /dev/sdc
    mounts:
      - - LABEL=etcd_data
        - /var/lib/etcd_data
```

## 