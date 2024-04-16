# Data Disks
This section describes how to specify additional data disks for a linode instance. These disks can use devices `sdb` through `sdh` 
for a total of 7 disks. 

~~~admonish warning
There are a couple caveats with specifying disks for a linode instance:
1. The total size of these disks + the OS Disk cannot exceed the linode instance plan size.
2. Instance disk configuration is currently immutable via CAPL after the instance is booted.
~~~

```admonish warning
Currently SDB is being used by a swap disk, replacing this disk with a data disk will slow down linode creation by
up to 90 seconds. This will be resolved when the disk creation refactor is finished in PR [#216](https://github.com/linode/cluster-api-provider-linode/pull/216)
```
## Specify a data disk
A LinodeMachine can be configured with additional data disks with the key being the device to be mounted as and including an optional label and size.
* `size` Required field. [resource.Quantity](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/) for the size if a disk. The sum of all data disks must not be more than allowed by the [linode plan](https://www.linode.com/pricing/#compute-shared). 
* `label`  Optional field. The label for the disk, defaults to the device name
* `diskID` Optional field used by the controller to track disk IDs, this should not be set unless a disk is created outside CAPL
* `filesystem` Optional field used to specify the type filesystem of disk to provision, the default is `ext4` and valid options are any supported linode  filesystem

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
          size: 16Gi
        sdd:
          label: data_disk
          size: 10Gi
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
          size: 16Gi

---
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
    diskSetup:
      filesystems:
        - label: etcd_data
          filesystem: ext4
          device: /dev/sdc
    mounts:
      - - LABEL=etcd_data
        - /var/lib/etcd_data
```
