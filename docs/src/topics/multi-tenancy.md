# Multi-Tenancy

CAPL can manage multi-tenant workload clusters across Linode accounts. Custom resources may reference an optional Secret
containing their Linode credentials (i.e. API token) to be used for the deployment of Linode resources (e.g. Linodes,
VPCs, NodeBalancers, etc.) associated with the cluster.

The following example shows a basic credentials Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: linode-credentials
stringData:
  apiToken: <LINODE_TOKEN>
```

```admonish warning
The Linode API token data must be put in a key named `apiToken`!
```

Which may be optionally consumed by one or more custom resource objects:

```yaml
# Example: LinodeCluster
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeCluster
metadata:
  name: test-cluster
spec:
  credentialsRef:
    name: linode-credentials
  ...
---
# Example: LinodeVPC
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeVPC
metadata:
  name: test-vpc
spec:
  credentialsRef:
    name: linode-credentials
  ...
---
# Example: LinodeMachine
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: test-machine
spec:
  credentialsRef:
    name: linode-credentials
  ...
---
# Example: LinodeObjectStorageBucket
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeObjectStorageBucket
metadata:
  name: test-bucket
spec:
  credentialsRef:
    name: linode-credentials
  ...
```

Secrets from other namespaces by additionally specifying an optional
`.spec.credentialsRef.namespace` value.

```admonish warning
If `.spec.credentialsRef` is set for a LinodeCluster, it should also be set for adjacent resources (e.g. LinodeVPC).
```

## LinodeMachine

For LinodeMachines, credentials set on the LinodeMachine object will override any credentials supplied by the owner
LinodeCluster. This can allow cross-account deployment of the Linodes for a cluster.
