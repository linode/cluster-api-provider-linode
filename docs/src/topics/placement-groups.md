# Placement Groups

This guide covers how configure [placement groups](https://techdocs.akamai.com/cloud-computing/docs/work-with-placement-groups) within a CAPL cluster. 
Placement groups are currently provisioned with any of the `*-full` flavors in the `LinodeMachineTemplate` for the control plane machines only.
```admonish note
Currently only 5 nodes are allowed in a single placement group
```

## Placement Group Creation

For controlling placement groups via Linode resources, a [placement groups](https://techdocs.akamai.com/cloud-computing/docs/work-with-placement-groups) can
be defined and provisioned via the `PlacementGroup` resource in CAPL.


Example `PlacementGroup`:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodePlacementGroup
metadata:
  name: test-cluster
spec:
  region: us-ord
```

## PlacementGroup Machine Integration
In order to use a placement group with a machine, a `PlacementGroupRef` can be used in the `LinodeMachineTemplate` spec
to assign any nodes used in that template to the placement group. Due to the limited size of the placement group our templates
currently only integrate with this for control plane nodes

Example `LinodeMachineTemplate`:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeMachineTemplate
metadata:
  name: test-cluster-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      interfaces:
        - purpose: public
      placementGroupRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
        kind: LinodePlacementGroup
        name: test-cluster
      region: us-ord
      type: g6-standard-4
```
