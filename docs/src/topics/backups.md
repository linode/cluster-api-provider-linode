# Backups

CAPL supports performing etcd backups by provisioning an Object Storage bucket and access keys. This feature is not enabled by default and can be configured as an addon.

```admonish warning
Enabling this addon requires enabling Object Storage in the account where the resources will be provisioned. Please refer to the [Pricing](https://www.linode.com/docs/products/storage/object-storage/#pricing) information in Linode's [Object Storage documentation](https://www.linode.com/docs/products/storage/object-storage/).
```

## Enabling Backups

To enable backups, use the addon flag during provisioning to select the etcd-backup-restore addon
```sh
clusterctl generate cluster $CLUSTER_NAME \
  --kubernetes-version v1.33.4 \
  --infrastructure linode-linode \
  --flavor etcd-backup-restore \
  | kubectl apply -f -
```
For more fine-grain control and to know more about etcd backups, refer to [the backups section of the etcd page](../topics/etcd.md#etcd-backups)

## Object Storage

Additionally, CAPL can be used to provision Object Storage buckets and access keys for general purposes by configuring `LinodeObjectStorageBucket` and `LinodeObjectStorageKey` resources.

```admonish warning
Using this feature requires enabling Object Storage in the account where the resources will be provisioned. Please refer to the [Pricing](https://www.linode.com/docs/products/storage/object-storage/#pricing) information in Linode's [Object Storage documentation](https://www.linode.com/docs/products/storage/object-storage/).
```

### Bucket Creation

The following is the minimal required configuration needed to provision an Object Storage bucket.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeObjectStorageBucket
metadata:
  name: <unique-bucket-label>
  namespace: <namespace>
spec:
  region: <object-storage-region>
```

Upon creation of the resource, CAPL will provision a bucket in the region specified using the `.metadata.name` as the bucket's label.

```admonish warning
The bucket label must be unique within the region across all accounts. Otherwise, CAPL will populate the resource status fields with errors to show that the operation failed.
```

### Bucket Status

Upon successful provisioning of a bucket, the `LinodeObjectStorageBucket` resource's status will resemble the following:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeObjectStorageBucket
metadata:
  name: <unique-bucket-label>
  namespace: <namespace>
spec:
  region: <object-storage-region>
status:
  ready: true
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: <timestamp>
  hostname: <hostname-for-bucket>
  creationTime: <bucket-creation-timestamp>
```

### Access Key Creation

The following is the minimal required configuration needed to provision an Object Storage key.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeObjectStorageKey
metadata:
  name: <unique-key-label>
  namespace: <namespace>
spec:
  bucketAccess:
    - bucketName: <unique-bucket-label>
      permissions: read_only
      region: <object-storage-region>
  generatedSecret:
    type: Opaque
```

Upon creation of the resource, CAPL will provision an access key in the region specified using the `.metadata.name` as the key's label.

The credentials for the provisioned access key will be stored in a Secret. By default, the Secret is generated in the same namespace as the `LinodeObjectStorageKey`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <unique-bucket-label>-obj-key
  namespace: <same-namespace-as-object-storage-bucket>
  ownerReferences:
    - apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
      kind: LinodeObjectStorageBucket
      name: <unique-bucket-label>
      controller: true
      uid: <unique-uid>
data:
  access: <base64-encoded-access-key>
  secret: <base64-encoded-secret-key>
```

The secret is owned and managed by CAPL during the life of the `LinodeObjectStorageBucket`.

### Access Key Status

Upon successful provisioning of a key, the `LinodeObjectStorageKey` resource's status will resemble the following:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeObjectStorageKey
metadata:
  name: <unique-key-label>
  namespace: <namespace>
spec:
  bucketAccess:
    - bucketName: <unique-bucket-label>
      permissions: read_only
      region: <object-storage-region>
  generatedSecret:
    type: Opaque
status:
  ready: true
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: <timestamp>
  accessKeyRef: <object-storage-key-id>
  creationTime: <key-creation-timestamp>
  lastKeyGeneration: 0
```

### Access Key Rotation

The following configuration with `keyGeneration` set to a new value (different from `.status.lastKeyGeneration`) will instruct CAPL to rotate the access key.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeObjectStorageKey
metadata:
  name: <unique-key-label>
  namespace: <namespace>
spec:
  bucketAccess:
    - bucketName: <unique-bucket-label>
      permissions: read_only
      region: <object-storage-region>
  generatedSecret:
    type: Opaque
  keyGeneration: 1
# status:
#   lastKeyGeneration: 0
```

### Resource Deletion

When deleting a `LinodeObjectStorageKey` resource, CAPL will deprovision the access key and delete the managed secret. However, when deleting a `LinodeObjectStorageBucket` resource, CAPL will retain the underlying bucket to avoid unintended data loss unless `.spec.forceDeleteBucket` is set to `true` in the `LinodeObjectStorageBucket` resource (defaults to `false`).

When using etcd backups, the bucket can be cleaned up on cluster deletion by setting `FORCE_DELETE_OBJ_BUCKETS` to `true` (defaults to `false` to avoid unintended data loss).
