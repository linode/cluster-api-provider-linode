# Tag Propogation


Whenever a cluster is provisioned using CAPL, tags can be added to the linodes through `.Spec.Tags` field in `LinodeMachineTemplate`. Tags are immutable (i.e. cannot be updated once a linode is provisioned).

CAPI doesn't propogate`.Spec.Tags` on `LinodeMachineTemplate`. So, the annoation `linode-vm-tags` has been added which updates the tags on a linode whenever the annotation is added to a `LinodeMachine`.

Please find attached examples on how to add tags via this annotation:

- With No tags: `linode-vm-tags: "[]"`
- With tags: `linode-vm-tags: "[\"tag1\", \"tag2\"]"`

This annotation can also be added on `MachineDeployment` or `KubeadmControlPlane`. CAPI propogates the annotations added on these resources to all the `LinodeMachine` resources that belong to these two.

Note: This annotation doesn't affect any auto-generated tags that are added by CAPL.