package scope

import (
	"context"

	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/mock"
)

// LinodeObjectStorageClient defines functions suitable for provisioning object storage buckets and keys.
type LinodeObjectStorageClient interface {
	ListObjectStorageBucketsInCluster(ctx context.Context, opts *linodego.ListOptions, cluster string) ([]linodego.ObjectStorageBucket, error)
	CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(ctx context.Context, keyID int) error
}

// LinodeObjectStorageClientBuilder is a function that returns a LinodeObjectStorageClient.
type LinodeObjectStorageClientBuilder func(apiKey string) (LinodeObjectStorageClient, error)

// CreateLinodeObjectStorageClient is the main implementation of LinodeObjectStorageClientBuilder.
func CreateLinodeObjectStorageClient(apiKey string) (LinodeObjectStorageClient, error) {
	return CreateLinodeClient(apiKey)
}

// CreateMockLinodeObjectStorageClient returns a mock LinodeObjectStorageClientBuilder for tests.
func CreateMockLinodeObjectStorageClientBuilder(m *mock.MockLinodeObjectStorageClient) LinodeObjectStorageClientBuilder {
	return func(_ string) (LinodeObjectStorageClient, error) {
		return m, nil
	}
}

type k8sClient interface {
	client.Client
}

type PatchHelper interface {
	Patch(ctx context.Context, obj client.Object, opts ...patch.Option) error
}
