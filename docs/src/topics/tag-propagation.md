# Tag Propagation


Whenever a cluster is provisioned using CAPL, tags can be added to the linodes through one of the following options:
- `MachineDeployment` via `spec.template.metadata.annotations` (for worker instances):
    ```yaml
    apiVersion: cluster.x-k8s.io/v1beta1
    kind: MachineDeployment
    metadata:
      name: ${CLUSTER_NAME}-md-0
    spec:
      template:
        metadata:
          annotations:
            linode-vm-tags: "[\"workers\",\"example-tag1\"]"
    ```
- `KubeadmControlPlane`,  `RKE2ControlPlane` or `KThreesControlPlane` via `spec.machineTemplate.metadata.annotations` (for control plane instances):
    ```yaml
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    metadata:
     name: ${CLUSTER_NAME}-control-plane
    spec:
      machineTemplate:
        metadata:
          annotations:
            linode-vm-tags: "[\"control-plane\",\"example-tag2\"]"
    ```
- The `.metadata.annotations` field in `LinodeMachine` (if manually defining `LinodeMachine` resources)

The tags set via `LinodeMachineTemplate.spec.tags` do not propagate and should not be used. This field is deprecated and will be removed in a future release (see https://github.com/linode/cluster-api-provider-linode/issues/774)

Note: This annotation doesn't affect any auto-generated tags that are added by CAPL.
