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

## Replacing Machines Scheduled for Maintenance

CAPL detects upcoming Linode infrastructure maintenance windows and sets a `MaintenanceScheduled` condition on
the corresponding CAPI `Machine` objects. This condition can be used as a trigger for `MachineHealthCheck` to
automatically replace machines before their maintenance window begins.

### How it works

During each `LinodeCluster` reconciliation, CAPL queries the Linode API for maintenance events scheduled within
the next 72 hours. For each Linode instance that matches a `LinodeMachine` in the cluster, CAPL sets:

```
condition:
  type: MaintenanceScheduled
  status: "True"
```

on the owning CAPI `Machine` object. A `MachineHealthCheck` with `unhealthyMachineConditions` targeting this
condition will then trigger remediation — replacing the machine before the maintenance window starts.

### Example MachineHealthCheck

The following `MachineHealthCheck` replaces worker machines when `MaintenanceScheduled=True` has been set for
more than 1 hour:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachineHealthCheck
metadata:
  name: ${CLUSTER_NAME}-maintenance
spec:
  clusterName: ${CLUSTER_NAME}
  selector:
    matchLabels:
      cluster.x-k8s.io/deployment-name: ${CLUSTER_NAME}
  checks:
    unhealthyMachineConditions:
      - type: MaintenanceScheduled
        status: "True"
        timeoutSeconds: 3600
  remediation:
    triggerIf:
      unhealthyLessThanOrEqualTo: 1
```

For control plane machines managed by `KubeadmControlPlane`:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachineHealthCheck
metadata:
  name: ${CLUSTER_NAME}-cp-maintenance
spec:
  clusterName: ${CLUSTER_NAME}
  selector:
    matchLabels:
      cluster.x-k8s.io/control-plane: ""
  checks:
    unhealthyMachineConditions:
      - type: MaintenanceScheduled
        status: "True"
        timeoutSeconds: 3600
  remediation:
    triggerIf:
      unhealthyLessThanOrEqualTo: 1
```

### Field reference

| Field | Description |
|-------|-------------|
| `checks.unhealthyMachineConditions` | Conditions checked on the CAPI `Machine` object (not the Node). `MaintenanceScheduled` is set here by CAPL. |
| `type: MaintenanceScheduled` | The condition type set by CAPL when a Linode maintenance event is scheduled within 72 hours. |
| `status: "True"` | The condition status that indicates maintenance is scheduled. |
| `timeoutSeconds` | How long the condition must be present before remediation is triggered. Set this to a value less than the expected lead time before the maintenance window starts. |
| `remediation.triggerIf.unhealthyLessThanOrEqualTo` | Prevents remediation if too many machines are already unhealthy. For control plane clusters, set to `1` to avoid remediating multiple control plane nodes simultaneously and losing etcd quorum. |

### Choosing a timeout

CAPL sets `MaintenanceScheduled` up to 72 hours before the maintenance window. A `timeoutSeconds` of `3600`
(1 hour) means remediation begins 71 hours before the window at the earliest. Adjust this value based on
how much lead time your workloads require for graceful draining.

### Limitations

- Only machines owned by a `MachineSet` or `KubeadmControlPlane` can be remediated by a `MachineHealthCheck`.
  Standalone machines are not eligible.
- The `MaintenanceScheduled` condition is never explicitly cleared by CAPL. Machines will be replaced by the
  `MachineHealthCheck` before the condition is removed, which is the intended behavior. 
- Control plane remediation preserves etcd quorum: CAPI will not remediate a second control plane machine
  until the replacement for the first is healthy. Set `unhealthyLessThanOrEqualTo: 1` for control plane
  `MachineHealthChecks` to prevent simultaneous replacements.
