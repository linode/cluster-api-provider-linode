# Flavors

This section contains information about supported flavors in Cluster API Provider Linode

In `clusterctl` the infrastructure provider authors can provide different types
of cluster templates referred to as "flavors". You can use the `--flavor` flag
to specify which flavor to use for a cluster, e.g:

```bash
clusterctl generate cluster test-cluster --flavor clusterclass
```

To use the default flavor, omit the `--flavor` flag.

See the [`clusterctl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors) for more information.
