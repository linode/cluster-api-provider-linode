package scope

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/linodego"
)

// LinodeObjectStorageClient defines functions suitable for provisioning object storage buckets and keys.
type LinodeObjectStorageClient interface {
	ListObjectStorageBucketsInCluster(ctx context.Context, opts *linodego.ListOptions, cluster string) ([]linodego.ObjectStorageBucket, error)
	CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(ctx context.Context, keyID int) error
}

// LinodeObjectStorageClientFactory is a function that returns a LinodeObjectStorageClient.
type LinodeObjectStorageClientFactory func(apiKey string) LinodeObjectStorageClient

type k8sClient interface {
	client.Reader
	client.Writer
}
