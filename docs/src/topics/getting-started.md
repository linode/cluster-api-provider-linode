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

```admonish question title=""
For more information please see the
[Linode Guide](https://www.linode.com/docs/products/tools/api/guides/manage-api-tokens/#create-an-api-token).
```

## Setting up your cluster environment variables

Once you have provisioned your PAT, save it in an environment variable along with other required settings:
```bash
# Cluster settings
export CLUSTER_NAME=capl-cluster
export KUBERNETES_VERSION=v1.29.1

# Linode settings
export LINODE_REGION=us-ord
export LINODE_TOKEN=<your linode PAT>
export LINODE_CONTROL_PLANE_MACHINE_TYPE=g6-standard-2
export LINODE_MACHINE_TYPE=g6-standard-2
```
## Specifying the region and image

### Image
The default image is linode/ubuntu22.04 if nothing is specified. This can be overridden, but currently only ubuntu is 
supported for our kubeadm based templates, any k3s/rke2 based chart should work with other images. If the 
image does not support the Akamai datasource for cloud init (see supported images via the [Linode api](https://www.linode.com/docs/api/images/))
you will have to set the `USE_STACKSCRIPT_BOOTSTRAP=true` environment variable before generating your cluster or set the
`useStackScriptBootstrap: true` field on your machine resources

### Region
Region is a required field. If you deploy to a region without the metadata service available
CAPL will automatically set `useStackScriptBootstrap: true` and cluster provisioning should still work. To look up a 
list of regions and what capabilities they have, use the regions endpoint of the [Linode api](https://www.linode.com/docs/api/regions/).

## Register linode locally as an infrastructure provider
1. Generate local release files 
    ```bash
    make local-release
    ```
2. Add `linode` as an infrastructure provider in `~/.cluster-api/clusterctl.yaml`
    ```yaml
    providers:
       - name: linode
         url: ~/cluster-api-provider-linode/infrastructure-linode/0.0.0/infrastructure-components.yaml
         type: InfrastructureProvider
    ```

## Deploying your first cluster

Please refer to the [default flavor](./flavors/default.md) section for creating your first Kubernetes cluster on Linode using Cluster API. 
