# Backups

CAPL supports performing etcd backups by provisioning an Object Storage bucket and access keys. This feature is not enabled by default and can be configured as an addon.

```admonish warning
Enabling this addon requires enabling Object Storage in the account where the resources will be provisioned. Please refer to the [Pricing](https://www.linode.com/docs/products/storage/object-storage/#pricing) information in Linode's [Object Storage documentation](https://www.linode.com/docs/products/storage/object-storage/).
```

## Enabling Backups

To enable backups, use the addon flag during provisioning to select the etcd-backup-restore addon
```sh
clusterctl generate cluster $CLUSTER_NAME \
  --kubernetes-version v1.29.1 \
  --infrastructure linode:0.0.0 \
  --flavor etcd-backup-restore \
  | kubectl apply -f -
```
For more fine-grain control and to know more about etcd backups, refer to [the backups section of the etcd page](../topics/etcd.md#etcd-backups)

## Object Storage

Additionally, CAPL can be used to provision Object Storage buckets and access keys for general purposes by configuring a `LinodeObjectStorageBucket` resource.

```admonish warning
Using this feature requires enabling Object Storage in the account where the resources will be provisioned. Please refer to the [Pricing](https://www.linode.com/docs/products/storage/object-storage/#pricing) information in Linode's [Object Storage documentation](https://www.linode.com/docs/products/storage/object-storage/).
```

### Bucket Creation

The following is the minimal required configuration needed to provision an Object Storage bucket and set of access keys.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeObjectStorageBucket
metadata:
  name: <unique-bucket-label>
  namespace: <namespace>
spec:
  cluster: <object-storage-region>
  secretType: Opaque
```

Upon creation of the resource, CAPL will provision a bucket in the region specified using the `.metadata.name` as the bucket's label.

```admonish warning
The bucket label must be unique within the region across all accounts. Otherwise, CAPL will populate the resource status fields with errors to show that the operation failed.
```

### Access Keys Creation

CAPL will also create `read_write` and `read_only` access keys for the bucket and store credentials in a secret in the same namespace where the `LinodeObjectStorageBucket` was created alongwith other details about the Linode OBJ Bucket:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <unique-bucket-label>-bucket-details
  namespace: <same-namespace-as-object-storage-bucket>
  ownerReferences:
    - apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
      kind: LinodeObjectStorageBucket
      name: <unique-bucket-label>
      controller: true
data:
  bucket_name: <unique-bucket-label>
  bucket_region: <linode-obj-bucket-region>
  bucket_endpoint: <hostname-to-access-bucket>
  access_key_rw: <base64-encoded-access-key>
  secret_key_rw: <base64-encoded-secret-key>
  access_key_ro: <base64-encoded-access-key>
  secret_key_ro: <base64-encoded-secret-key>
```

The bucket-details secret is owned and managed by CAPL during the life of the `LinodeObjectStorageBucket`.

### Access Keys Rotation

The following configuration with `keyGeneration` set to a new value (different from `.status.lastKeyGeneration`) will instruct CAPL to rotate the access keys.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeObjectStorageBucket
metadata:
  name: <unique-bucket-label>
  namespace: <namespace>
spec:
  cluster: <object-storage-region>
  secretType: Opaque
  keyGeneration: 1
# status:
#   lastKeyGeneration: 0
```

### Bucket Status

Upon successful provisioning of a bucket and keys, the `LinodeObjectStorageBucket` resource's status will resemble the following:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeObjectStorageBucket
metadata:
  name: <unique-bucket-label>
  namespace: <namespace>
spec:
  cluster: <object-storage-region>
  secretType: Opaque
  keyGeneration: 0
status:
  ready: true
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: <timestamp>
  hostname: <hostname-for-bucket>
  creationTime: <bucket-creation-timestamp>
  lastKeyGeneration: 0
  keySecretName: <unique-bucket-label>-bucket-details
  accessKeyRefs:
    - <access-key-rw-id>
    - <access-key-ro-id>
```

### Resource Deletion

When deleting a `LinodeObjectStorageBucket` resource, CAPL will deprovision the access keys and managed secret but retain the underlying bucket to avoid unintended data loss.
