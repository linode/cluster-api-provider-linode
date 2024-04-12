# Flavors

## [Flavor usage documentation](https://linode.github.io/cluster-api-provider-linode/topics/flavors/flavors.html)

## Development

This directory contains each of the flavors for CAPL. Each directory besides `base` will be used to
create a flavor by running `kustomize build` on the directory. The name of the directory will be
appended to the end of the cluster-template.yaml, e.g cluster-template-{directory-name}.yaml. That
flavor can be used by specifying `--flavor {directory-name}`.

To generate all CAPL flavors, run `make generate-flavors`.
