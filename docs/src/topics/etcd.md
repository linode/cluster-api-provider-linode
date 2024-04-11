# Etcd

This guide covers etcd configuration for the control plane of provisioned CAPL clusters.

## Default configuration

By default, etcd is configured to be on a separate device from the root filesystem on
control plane nodes. The etcd disk is automatically sized at 10 GB with a quota backend of 8 GB per
recommendation from [the etcd documentation](https://etcd.io/docs/latest/dev-guide/limit/#storage-size-limit)

## ETCD Backups

By default, etcd is not backed-up. To enable backups, users need to choose the etcd-backup-restore flavor.

To begin with, this will deploy a Linode OBJ bucket. This serves as the S3-compatible target to store backups.

Next up, on provisioning the cluster, [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) is deployed as a statefulset.
The pod will need the bucket details like the name, region, endpoints and access credentials which are passed using the 
bucket-details secret that is created when the OBJ bucket gets created.

### Enabling SSE
Users can also enable SSE (Server-side encryption) by passing a SSE AES-256 Key as an env var. All env vars
[here](https://github.com/linode/cluster-api-provider-linode/blob/main/templates/addons/etcd-backup-restore/etcd-backup-restore.yaml)
on the pod can be controlled during the provisioning process.

> [!WARNING]  
> This is currently under development and will be available for use once the upstream [PR](https://github.com/gardener/etcd-backup-restore/pull/719) is merged and an official image is made available

For eg:
```sh
export CLUSTER_NAME=test
export OBJ_BUCKET_REGION=us-ord-1
export ETCDBR_IMAGE=docker.io/username/your-custom-image:version
export SSE_KEY=cdQdZ3PrKgm5vmqxeqwQCuAWJ7pPVyHg
clusterctl generate cluster $CLUSTER_NAME \
  --kubernetes-version v1.29.1 \
  --infrastructure linode:0.0.0 \
  --flavor etcd-backup-restore \
  | kubectl apply -f -
```
