# Flavors

In `clusterctl` the infrastructure provider authors can provide different types
of cluster templates referred to as "flavors". You can use the `--flavor` flag
to specify which flavor to use for a cluster, e.g:

```shell
clusterctl generate cluster test-cluster --flavor clusterclass
```

To use the default flavor, omit the `--flavor` flag.

See the [`clusterctl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors) for more information.

This directory contains each of the flavors for CAPL. Each directory besides `base` will be used to
create a flavor by running `kustomize build` on the directory. The name of the directory will be
appended to the end of the cluster-template.yaml, e.g cluster-template-{directory-name}.yaml. That
flavor can be used by specifying `--flavor {directory-name}`.

To generate all CAPL flavors, run `make generate-flavors`.
