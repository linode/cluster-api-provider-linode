# Developing Cluster API Provider Linode

## Contents

<!-- TOC depthFrom:2 -->

- [Setting up](#setting-up)
  - [Base requirements](#base-requirements)
  - [Clone the source code](#clone-the-source-code)
  - [Get familiar with basic concepts](#get-familiar-with-basic-concepts)
- [Developing](#developing)
  - [Enable git hooks](#enable-git-hooks)
  - [Setting up the environment](#setting-up-the-environment)
    - [Using devbox](#using-devbox)
  - [Tilt Requirements](#tilt-requirements)
  - [Using Tilt](#using-tilt)
  - [Deploying a workload cluster](#deploying-a-workload-cluster)
    - [Customizing the cluster deployment](#customizing-the-cluster-deployment)
    - [Creating the workload cluster](#creating-the-workload-cluster)
    - [Cleaning up the workload cluster](#cleaning-up-the-workload-cluster)
  - [Automated Testing](#automated-testing)
    - [E2E Testing](#e2e-testing)

<!-- /TOC -->

## Setting up

### Base requirements

There are no requirements since development dependencies are fetched as
needed via the make targets, but a recommendation is to
[install Devbox](https://jetpack.io/devbox/docs/installing_devbox/)

### Clone the source code

```shell
git clone https://github.com/linode/cluster-api-provider-linode
cd cluster-api-provider-linode
```

### Get familiar with basic concepts

This provider is based on the [Cluster API project](https://github.com/kubernetes-sigs/cluster-api).
It's recommended to familiarize yourself with Cluster API resources, concepts, and conventions
outlined in the [Cluster API Book](https://cluster-api.sigs.k8s.io/).

## Developing

This repository uses [Go Modules](https://github.com/golang/go/wiki/Modules)
to track and vendor dependencies.

To pin a new dependency, run:
```bash
go get <repository>@<version>
```

### Enable git hooks

To enable automatic code validation on code push, execute the following commands:

```bash
PATH="$PWD/bin:$PATH" make husky && husky install
```

If you would like to temporarily disable git hook, set `SKIP_GIT_PUSH_HOOK` value:

```bash
SKIP_GIT_PUSH_HOOK=1 git push
```

### Setting up the environment

```admonish warning
Ensure you have your `LINODE_TOKEN` set as outlined in the 
[getting started prerequisites](../topics/getting-started.md#Prerequisites) section.
```

All development dependencies should be taken care of via Devbox and/or make target dependencies.

#### Using devbox

1. Install dependent packages in your project 
   ```shell
   devbox install
   ```

   ```admonish success title=""
   This will take a while, go and grab a drink of water.
   ```

2. Use devbox environment
   ```shell
   devbox shell
   ```

From this point you can use the devbox shell like a regular shell. 
The rest of the guide assumes a devbox shell is used, but the make target
dependencies will install any missing dependencies if needed when running
outside of a devbox shell.

### Using tilt
To build a kind cluster and start Tilt, simply run:
```shell
make tilt-cluster
```

Once your kind management cluster is up and running, you can
[deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the tilt-cluster, run

```shell
kind delete cluster --name tilt
```

### Deploying a workload cluster

After your kind management cluster is up and running with Tilt, you should be ready to deploy your first cluster.

#### Customizing the cluster deployment

Here is a list of required configuration parameters:

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

~~~admonish tip
You can also use `clusterctl generate` to see which variables need to be set:

```
clusterctl generate cluster $CLUSTER_NAME --from ./templates/cluster-template.yaml --list-variables
```

~~~

```admonish warning
Please note the templates require the use of `clusterctl generate` to substitute the environment variables properly.
```

#### Creating the workload cluster

Once you have all the necessary environment variables set,
you can deploy a workload cluster with the following command:

```shell
clusterctl generate cluster $CLUSTER_NAME \
  --kubernetes-version v1.29.1 \
  --from templates/cluster-template.yaml \
  | kubectl apply -f -
```

This will provision the cluster with the CNI defaulted to [cilium](../topics/addons.md#cilium)
and the [linode-ccm](../topics/addons.md#ccm) installed.

```admonish question title=""
For any issues, please refer to the [troubleshooting guide](../topics/troubleshooting.md).
```

#### Cleaning up the workload cluster

To delete the cluster, simply run:

```bash
kubectl delete cluster $CLUSTER_NAME
```

```admonish question title=""
For any issues, please refer to the [troubleshooting guide](../topics/troubleshooting.md).
```

### Automated Testing

#### E2E Testing

To run E2E locally run:
```bash
make e2etest
```

This command creates a KIND cluster, and executes all the defined tests.

```admonish warning
Please ensure you have [increased maximum open files on your host](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files)
```
