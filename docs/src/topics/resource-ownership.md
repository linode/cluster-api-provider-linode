# Resource Ownership and Lifecycle

In Kubernetes, `ownerReferences` are a mechanism to specify the relationship between objects, where one object (the owner) owns another object (the dependent). This is crucial for managing the lifecycle of related resources.

## Owner References in Cluster API Provider Linode (CAPL)

Cluster API Provider Linode (CAPL) utilizes `ownerReferences` to link various Linode-specific resources to their parent `LinodeCluster`. This means that the `LinodeCluster` acts as the owner for resources such as:

*   `LinodeFirewall`
*   `LinodeObjectStorageBucket`
*   `LinodeObjectStorageKey`
*   `LinodePlacementGroup`
*   `LinodeVPC`

When a `LinodeCluster` is created, and these associated resources are also created as part of the cluster definition or by controllers, CAPL automatically sets an `ownerReference` on these dependent resources, pointing back to the `LinodeCluster`.

## Implications of Ownership

The primary implication of this ownership model is **garbage collection**. When the `LinodeCluster` object is deleted, the Kubernetes garbage collector will automatically delete all the resources that are owned by it. This simplifies cluster teardown and helps prevent orphaned resources in your Linode account.

For example, if you delete a `LinodeCluster`:
*   Any `LinodeVPC` created for that cluster will be deleted.
*   Any `LinodeFirewall` associated with that cluster will be deleted.
*   Any `LinodeObjectStorageBucket` used by that cluster (and owned by it) will be deleted.
*   And so on for other owned resources.

This ensures that the lifecycle of these infrastructure components is tightly coupled with the lifecycle of the Kubernetes cluster itself, as managed by Cluster API.

## Verifying Ownership

You can inspect the `ownerReferences` of a resource using `kubectl describe` or `kubectl get <resource> <name> -o yaml`. Look for the `metadata.ownerReferences` field.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeVPC
metadata:
  name: my-cluster-vpc
  namespace: default
  ownerReferences:
  - apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    blockOwnerDeletion: true
    controller: true
    kind: LinodeCluster
    name: my-cluster
    uid: <uid-of-linodecluster>
# ... other fields
```

In the example above, the `LinodeVPC` named `my-cluster-vpc` is owned by the `LinodeCluster` named `my-cluster`.

Understanding these ownership relationships is key to effectively managing your cluster resources with CAPL. 