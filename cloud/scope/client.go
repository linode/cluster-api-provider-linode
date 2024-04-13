package scope

import (
	"context"

	"github.com/linode/linodego"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LinodeClient is an interface that defines the methods that a Linode client must have to interact with Linode.
// It defines all the functions that are required to create, delete, and get resources
// from Linode such as object storage buckets, node balancers, linodes, and VPCs.
type LinodeMachineClient interface {
	LinodeNodeBalancerClient
	LinodeInstanceClient
	LinodeVPCClient
}

// LinodeInstanceClient defines the methods that a Linode client must have to interact with Linode's Instance service.
type LinodeInstanceClient interface {
	GetInstanceIPAddresses(ctx context.Context, linodeID int) (*linodego.InstanceIPAddressResponse, error)
	ListInstances(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Instance, error)
	CreateInstance(ctx context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error)
	BootInstance(ctx context.Context, linodeID int, configID int) error
	ListInstanceConfigs(ctx context.Context, linodeID int, opts *linodego.ListOptions) ([]linodego.InstanceConfig, error)
	GetInstanceDisk(ctx context.Context, linodeID int, diskID int) (*linodego.InstanceDisk, error)
	ResizeInstanceDisk(ctx context.Context, linodeID int, diskID int, size int) error
	CreateInstanceDisk(ctx context.Context, linodeID int, opts linodego.InstanceDiskCreateOptions) (*linodego.InstanceDisk, error)
	GetInstance(ctx context.Context, linodeID int) (*linodego.Instance, error)
	DeleteInstance(ctx context.Context, linodeID int) error
	GetRegion(ctx context.Context, regionID string) (*linodego.Region, error)
	GetImage(ctx context.Context, imageID string) (*linodego.Image, error)
	CreateStackscript(ctx context.Context, opts linodego.StackscriptCreateOptions) (*linodego.Stackscript, error)
	ListStackscripts(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Stackscript, error)
	WaitForInstanceDiskStatus(ctx context.Context, instanceID int, diskID int, status linodego.DiskStatus, timeoutSeconds int) (*linodego.InstanceDisk, error)
}

// LinodeVPCClient defines the methods that a Linode client must have to interact with Linode's VPC service.
type LinodeVPCClient interface {
	GetVPC(ctx context.Context, vpcID int) (*linodego.VPC, error)
}

// LinodeNodeBalancerClient defines the methods that a Linode client must have to interact with Linode's Node Balancer service.
type LinodeNodeBalancerClient interface {
	ListNodeBalancers(ctx context.Context, opts *linodego.ListOptions) ([]linodego.NodeBalancer, error)
	CreateNodeBalancer(ctx context.Context, opts linodego.NodeBalancerCreateOptions) (*linodego.NodeBalancer, error)
	CreateNodeBalancerConfig(ctx context.Context, nodebalancerID int, opts linodego.NodeBalancerConfigCreateOptions) (*linodego.NodeBalancerConfig, error)
	DeleteNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, nodeID int) error
	DeleteNodeBalancer(ctx context.Context, nodebalancerID int) error
	CreateNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, opts linodego.NodeBalancerNodeCreateOptions) (*linodego.NodeBalancerNode, error)
}

// LinodeObjectStorageClient defines the methods that a Linode client must have to interact with Linode's Object Storage service.
type LinodeObjectStorageClient interface {
	GetObjectStorageBucket(ctx context.Context, cluster, label string) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	GetObjectStorageKey(ctx context.Context, keyID int) (*linodego.ObjectStorageKey, error)
	CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(ctx context.Context, keyID int) error
}

type K8sClient interface {
	client.Client
}
