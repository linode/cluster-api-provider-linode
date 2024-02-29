# Etcd

This guide covers etcd configuration for the control plane of provisioned CAPL clusters.

## Default configuration

By default, etcd is configured to be on a separate device from the root filesystem on
control plane nodes. The etcd disk is automatically sized at 10 GB with a quota backend of 8 GB per
recommendation from [the etcd documentation](https://etcd.io/docs/latest/dev-guide/limit/#storage-size-limit)
