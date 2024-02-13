# Overview

This section provides examples for addons for self-managed clusters.

```admonish note
Currently, all addons are installed via 
[Cluster API Addon Provider Helm (CAAPH)](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm).

CAAPH is installed by default in the KIND cluster created by `make tilt-cluster`.

For more information, please refer to the
[CAAPH Quick Start](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/docs/quick-start.md).
```

# CNI

```admonish warning
By default, the CNI plugin is not installed for self-managed clusters.

To install a CNI, ensure that your `Cluster` is labeled with one of the below CNI options.
```

## Cilium

To install [Cilium](https://cilium.io/) on a self-managed cluster, simply apply the `cni: cilium`
label on the `Cluster` resource if not already present.

```bash
kubectl label cluster $CLUSTER_NAME cni=cilium
```

Cilium will then be automatically installed via CAAPH into the labeled self-managed cluster.

# CCM

In order for the `InternalIP` and `ExternalIP` of the provisioned Nodes to be set correctly,
the [linode-cloud-controller-manager (linode-ccm)](https://github.com/linode/linode-cloud-controller-manager)
must be installed into provisioned clusters.


To install the linode-ccm on a self-managed cluster, simply apply the `ccm: linode`
label on the `Cluster` resource if not already present.

```bash
kubectl label cluster $CLUSTER_NAME ccm=linode
```

The linode-ccm will then be automatically installed via CAAPH into the labeled self-managed cluster.