package scope

import (
	"context"

	"github.com/linode/linodego"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LinodeClient defines around the LinodeGo Client. It defines all the functions that are required to create, delete, and get resources
// from Linode such as object storage buckets, node balancers, linodes, VPCs, etc.
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
	DeleteNodeBalancer(ctx context.Context, nodebalancerID int) error
	CreateNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, opts linodego.NodeBalancerNodeCreateOptions) (*linodego.NodeBalancerNode, error)
	ListInstances(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Instance, error)
	CreateInstance(ctx context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error)
	BootInstance(ctx context.Context, linodeID int, configID int) error
	ListInstanceConfigs(ctx context.Context, linodeID int, opts *linodego.ListOptions) ([]linodego.InstanceConfig, error)
	GetInstanceDisk(ctx context.Context, linodeID int, diskID int) (*linodego.InstanceDisk, error)
	ResizeInstanceDisk(ctx context.Context, linodeID int, diskID int, size int) error
	CreateInstanceDisk(ctx context.Context, linodeID int, opts linodego.InstanceDiskCreateOptions) (*linodego.InstanceDisk, error)
	GetInstance(ctx context.Context, linodeID int) (*linodego.Instance, error)
	DeleteInstance(ctx context.Context, linodeID int) error
	GetVPC(ctx context.Context, vpcID int) (*linodego.VPC, error)
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
