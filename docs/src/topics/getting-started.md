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

```admonish question title=""
For more information please see the
[Linode Guide](https://www.linode.com/docs/products/tools/api/guides/manage-api-tokens/#create-an-api-token).
```

## Setting up your Linode environment

Once you have provisioned your PAT, save it in an environment variable:
```bash
export LINODE_TOKEN="<LinodePAT>"
```

## Building your first cluster

Please continue from the [setting up the environment](../developers/development.md#setting-up-the-environment)
section for creating your first Kubernetes cluster on Linode using Cluster API. 
