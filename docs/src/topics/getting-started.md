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
```admonish warning
For Regions and Images that do not yet support Akamai's cloud-init datasource CAPL will automatically use a stackscript shim
to provision the node. If you are using a custom image ensure the [cloud_init](https://www.linode.com/docs/api/images/#image-create) flag is set correctly on it
```
```admonish warning
By default, clusters are provisioned within VPC. For Regions which do not have [VPC support](https://www.linode.com/docs/products/networking/vpc/#availability) yet, use the [VPCLess](./flavors/vpcless.md) flavor to have clusters provisioned.
```

## Install CAPL on your management cluster
```admonish warning
The `linode-linode` infrastructure provider requires clusterctl version 1.7.2 or higher
```
Install CAPL and enable the helm addon provider which is used by the majority of the CAPL flavors
```bash
clusterctl init --infrastructure linode-linode --addon helm
```

## Deploying your first cluster

Please refer to the [default flavor](../topics/flavors/default.md) section for creating your first Kubernetes cluster on Linode using Cluster API. 
