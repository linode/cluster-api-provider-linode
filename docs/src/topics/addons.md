# Overview

This section documents addons for self-managed clusters.

```admonish note
Currently, all addons are installed via 
[Cluster API Addon Provider Helm (CAAPH)](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm).

CAAPH is installed by default in the KIND cluster created by `make tilt-cluster`.

For more information, please refer to the
[CAAPH Quick Start](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/docs/quick-start.md).
```

```admonish note
The [Linode Cloud Controller Manager](#linode-cloud-controller-manager) and
[Linode CSI Driver](#linode-csi-driver) addons require the `ClusterResourceSet` feature flag
to be set on the management cluster.

This feature flag is enabled by default in the KIND cluster created by `make tilt-cluster`.

For more information, please refer to [the ClusterResourceSet page in The Cluster API Book](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-resource-set).
```


## Contents

<!-- TOC depthFrom:2 -->

- [CNI](#cni)
  - [Cilium](#cilium)
- [CCM](#ccm)
  - [Linode Cloud Controller Manager](#linode-cloud-controller-manager)
- [Container Storage](#container-storage)
  - [Linode CSI Driver](#linode-csi-driver)

<!-- /TOC -->

## CNI

In order for pod networking to work properly, a Container Network Interface (CNI) must be installed.

### Cilium

```admonish success title=""
Installed by default
```

To install [Cilium](https://cilium.io/) on a self-managed cluster, simply apply the `cni: cilium`
label on the `Cluster` resource if not already present.

```bash
kubectl label cluster $CLUSTER_NAME cni=cilium --overwrite
```

Cilium will then be automatically installed via CAAPH into the labeled cluster.

#### Enabled Features
By default, Cilium's [BGP Control Plane](https://docs.cilium.io/en/stable/network/bgp-control-plane/)
is enabled when using Cilium as the CNI.

## CCM

In order for the `InternalIP` and `ExternalIP` of the provisioned Nodes to be set correctly,
a Cloud Controller Manager (CCM) must be installed.

### Linode Cloud Controller Manager

```admonish success title=""
Installed by default
```

To install the [linode-cloud-controller-manager (linode-ccm)](https://github.com/linode/linode-cloud-controller-manager)
on a self-managed cluster, simply apply the `ccm: linode`
label on the `Cluster` resource if not already present.

```bash
kubectl label cluster $CLUSTER_NAME ccm=linode --overwrite
```

The linode-ccm will then be automatically installed via CAAPH into the labeled cluster.

## Container Storage

In order for stateful workloads to create PersistentVolumes (PVs), a storage driver must be installed.

### Linode CSI Driver

```admonish success title=""
Installed by default
```

To install the [csi-driver-linode](https://github.com/linode/linode-blockstorage-csi-driver)
on a self-managed cluster, simply apply the `csi: linode`
label on the `Cluster` resource if not already present.

```bash
kubectl label cluster $CLUSTER_NAME csi=linode --overwrite
```

The csi-driver-linode will then be automatically installed via CAAPH into the labeled cluster.
