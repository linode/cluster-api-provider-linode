# OS Disk

This section describes how to configure the root disk for provisioned linode. By default, the OS disk will be dynamically
sized to use any size available in the linode plan that is not taken up by [data disks](./data-disks.md).


## Setting OS Disk Size
Use the `osDisk` section to specify the exact size the OS disk should be. The default behaviour if this is not set is
the OS disk will dynamically be sized to the maximum allowed by the linode plan with any data disk sizes taken into account.
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeMachineTemplate
metadata:
  name: ${CLUSTER}-control-plane
spec:
  template:
    spec:
      region: us-ord
      type: g6-standard-4
      osDisk:
        size: 100Gi



```

## Setting OS Disk Label
The default label on the root OS disk can be overridden by specifying a label in the `osDisk` field. The label can only
be set if an explicit size is being set as `size` is a required field

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeMachineTemplate
metadata:
  name: ${CLUSTER}-control-plane
  namespace: default
spec:
  template:
    spec:
      image: ""
      region: us-ord
      type: g6-standard-4
      osDisk:
        label: root-disk
        size: 10Gi
```

