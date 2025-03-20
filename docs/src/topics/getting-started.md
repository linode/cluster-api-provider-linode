# Getting started with CAPL

## Prerequisites

- A [Linode account](https://linode.com/)
- A Personal Access Token (PAT) created via [the Linode Cloud Manager](https://cloud.linode.com/profile/tokens).
Make sure to create the token with at least the following read/write permissions (or "all"):
  - Linodes
  - NodeBalancers
  - Images
  - Volumes
  - VPCs
  - IPs
  - Object Storage
  - Stackscripts
  - Firewalls
- clusterctl is [installed](https://cluster-api.sigs.k8s.io/user/quick-start#installation)
- Cluster API [management cluster](https://cluster-api.sigs.k8s.io/user/quick-start#install-andor-configure-a-kubernetes-cluster) is created
```admonish question title=""
For more information please see the
[Linode Guide](https://www.linode.com/docs/products/tools/api/guides/manage-api-tokens/#create-an-api-token).
```

## Setting up your cluster environment variables

Once you have provisioned your PAT, save it in an environment variable along with other required settings:
```bash
export LINODE_REGION=us-ord
export LINODE_TOKEN=<your linode PAT>
export LINODE_CONTROL_PLANE_MACHINE_TYPE=g6-standard-2
export LINODE_MACHINE_TYPE=g6-standard-2
```

```admonish info
Consider also setting the following environment variables: `CONTROL_PLANE_MACHINE_COUNT=1` and `WORKER_MACHINE_COUNT=1`. These counts can also be modified case-by-case with the `--control-plane-machine-count` and `--worker-machine-count` flags. `clusterctl` defaults these to 1 and 0, respectively, which results in a control plane-only cluster with one node.
```

```admonish info
This project uses [linodego](https://github.com/linode/linodego) for Linode API interaction. 
Please refer to it for more details on environment variables used for client configuration.
```

```admonish warning
For Regions and Images that do not yet support Akamai's cloud-init datasource CAPL will automatically use a stackscript shim
to provision the node. If you are using a custom image ensure the [cloud_init](https://www.linode.com/docs/api/images/#image-create) flag is set correctly on it
```

## Setup management cluster
A clusterAPI management cluster is a kubernetes cluster that is responsible for managing the lifecycle of other child k8s clusters provisioned using Cluster API (CAPI). It serves as a control plane for provisioning, scaling, upgrading and deleting child kubernetes clusters.

Use any of the following to have a base management cluster:
- Provision k8s cluster using [LKE](https://techdocs.akamai.com/cloud-computing/docs/getting-started-with-lke-linode-kubernetes-engine)
- Bring/Use your own provisioned k8s cluster to be configured as management cluster

## Install CAPL on your management cluster
```admonish warning
The `linode-linode` infrastructure provider requires clusterctl version 1.7.2 or higher
```
Install CAPL and enable the helm addon provider which is used by the majority of the CAPL flavors

```bash
export KUBECONFIG=<mgmt-cluster-kubeconfig>
clusterctl init --infrastructure linode-linode --addon helm
```

Output will be something like:
```bash
Fetching providers
Installing cert-manager version="v1.16.0"
Waiting for cert-manager to be available...
Installing provider="cluster-api" version="v1.9.5" targetNamespace="capi-system"
Installing provider="bootstrap-kubeadm" version="v1.9.5" targetNamespace="capi-kubeadm-bootstrap-system"
Installing provider="control-plane-kubeadm" version="v1.9.5" targetNamespace="capi-kubeadm-control-plane-system"
Installing provider="infrastructure-linode-linode" version="v0.8.4" targetNamespace="capl-system"
Installing provider="addon-helm" version="v0.3.1" targetNamespace="caaph-system"

Your management cluster has been initialized successfully!

You can now create your first workload cluster by running the following:

  clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
```

## Deploying your first cluster

Please refer to the [default flavor](../topics/flavors/default.md) section for creating your first Kubernetes cluster on Linode using Cluster API. 
