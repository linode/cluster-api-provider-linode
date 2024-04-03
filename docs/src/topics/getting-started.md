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

## Register linode locally as an infrastructure provider
1. Generate local release files 
    ```bash
    make local-release
    ```
2. Add `linode` as an infrastructure provider in `~/.cluster-api/clusterctl.yaml`
    ```yaml
    providers:
       - name: linode
         url: ${PWD}/infrastructure-linode/0.0.0/infrastructure-components.yaml
         type: InfrastructureProvider
    ```

## Deploying your first cluster

Please refer to the [default flavor](../topics/flavors/default.md) section for creating your first Kubernetes cluster on Linode using Cluster API. 
