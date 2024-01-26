# Cluster API Provider Linode
A [Cluster API](https://cluster-api.sigs.k8s.io/) implementation for the [Linode](https://www.linode.com/) to create kubernetes clusters.

## Local development

### Enable git hooks

To enable automatic code validation on code push, execute the following commands:

```bash
PATH="$PWD/bin:$PATH" make husky && husky install
```

If you temporary would like to disable git hook, set `SKIP_GIT_PUSH_HOOK` value:

```bash
SKIP_GIT_PUSH_HOOK=1 git push
```

### Local development with Tilt

For local development execute the following `make` target:

```bash
LINODE_TOKEN=<YOUR LINODE TOKEN> make tilt-cluster
```

This command creates a Kind cluster, and deploys resources via Tilt. You can freely change the code and wait for Tilt to update provider.

### E2E testing

For local development execute the following `make` target:

```bash
LINODE_TOKEN=<YOUR LINODE TOKEN> make e2etest
```

This command creates a Kind cluster, and executes all the defined tests.

> Please ensure you have increased maximum open files on your host: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
