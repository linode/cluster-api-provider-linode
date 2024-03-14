package scope

import (
	"context"

	"github.com/linode/linodego"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LinodeClient defines functions suitable for provisioning object storage buckets and keys.
type LinodeClient interface {
	GetObjectStorageBucket(ctx context.Context, cluster, label string) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(ctx context.Context, keyID int) error
	ListNodeBalancers(ctx context.Context, opts *linodego.ListOptions) ([]linodego.NodeBalancer, error)
	CreateNodeBalancer(ctx context.Context, opts linodego.NodeBalancerCreateOptions) (*linodego.NodeBalancer, error)
	CreateNodeBalancerConfig(ctx context.Context, nodebalancerID int, opts linodego.NodeBalancerConfigCreateOptions) (*linodego.NodeBalancerConfig, error)
	GetInstanceIPAddresses(ctx context.Context, linodeID int) (*linodego.InstanceIPAddressResponse, error)
	DeleteNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, nodeID int) error
}

// LinodeClientBuilder is a function that returns a LinodeClient.
type LinodeClientBuilder func(apiKey string) (LinodeClient, error)

// CreateLinodeClientBuilder is the main implementation of LinodeClientBuilder.
func CreateLinodeClientBuilder(apiKey string) (LinodeClient, error) {
	return CreateLinodeClient(apiKey)
}

type k8sClient interface {
	client.Client
}
