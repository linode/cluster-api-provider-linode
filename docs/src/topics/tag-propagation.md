# Tag Propagation


Whenever a cluster is provisioned using CAPL, tags can be added/modified to the linodes through `LinodeMachineTemplate` via `spec.template.spec.tags`:
```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: LinodeMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-0
spec:
  template:
    spec:
      tags:
        - tag1
        - tag2
```
