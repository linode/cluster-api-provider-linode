# Machine Health Checks

CAPL supports auto-remediation of workload cluster Nodes considered to be unhealthy
via [`MachineHealthChecks`](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking).

## Enabling Machine Health Checks

While it is possible to manually create and apply a `MachineHealthCheck` resource into the management cluster,
using the `self-healing` flavor is the quickest way to get started:
```sh
clusterctl generate cluster $CLUSTER_NAME \
  --kubernetes-version v1.33.4 \
  --infrastructure linode-linode \
  --flavor self-healing \
  | kubectl apply -f -
```

This flavor deploys a `MachineHealthCheck` for the workers and another `MachineHealthCheck` for the control plane
of the cluster. It also configures the remediation strategy of the kubeadm control plane to prevent unnecessary load
on the infrastructure provider.

## Configuring Machine Health Checks

Refer to the [Cluster API documentation](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking)
for further information on configuring and using `MachineHealthChecks`.
