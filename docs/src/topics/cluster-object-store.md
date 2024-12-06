# Cluster Object Store

The Cluster Object Store is optional `LinodeCluster` setting that uses an object storage bucket for certain internal cluster operations. Currently, the Cluster Object store setting enables the following features:

- Passing large bootstrap to Lindoes during `LinodeMachine` boostraping

A [Linode Object Storage](https://www.linode.com/docs/guides/platform/object-storage/) bucket and access key are provisioned as the Cluster Object Store with any of the `*-full` flavors in the `LinodeCluster`. To use BYOB (Bring Your Own Bucket) instead,  modify a `LinodeCluster` definition:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeCluster
metadata:
name: ${CLUSTER_NAME}
spec:
  objectStore:
    credentialsRef:
      name: ${CLUSTER_NAME}-object-store-credentials
```

to reference any Secret containing a object storage bucket's credentials in the following format:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ${CLUSTER_NAME}-object-store-credentials
data:
  bucket_name: ${BUCKET_NAME}
  # Service endpoint
  # See: https://docs.aws.amazon.com/general/latest/gr/s3.html
  s3_endpoint: ${S3_ENDPOINT}
  access_key: ${ACCESS_KEY}
  secret_key: ${SECRET_KEY}
```
