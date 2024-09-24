package clients

import (
	"context"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/go-resty/resty/v2"
	"github.com/linode/linodego"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LinodeClient is an interface that defines the methods that a Linode client must have to interact with Linode.
// It defines all the functions that are required to create, delete, and get resources
// from Linode such as object storage buckets, node balancers, linodes, and VPCs.
type LinodeClient interface {
	LinodeNodeBalancerClient
	LinodeInstanceClient
	LinodeVPCClient
	LinodeObjectStorageClient
	LinodeDNSClient
	LinodePlacementGroupClient
	LinodeFirewallClient
	LinodeTokenClient

	OnAfterResponse(m func(response *resty.Response) error)
}

type AkamClient interface {
	AkamEdgeDNSClient
}

type AkamEdgeDNSClient interface {
	GetRecord(context.Context, string, string, string) (*dns.RecordBody, error)
	CreateRecord(context.Context, *dns.RecordBody, string, ...bool) error
	UpdateRecord(context.Context, *dns.RecordBody, string, ...bool) error
	DeleteRecord(context.Context, *dns.RecordBody, string, ...bool) error
}

// LinodeInstanceClient defines the methods that interact with Linode's Instance service.
type LinodeInstanceClient interface {
	GetInstanceIPAddresses(ctx context.Context, linodeID int) (*linodego.InstanceIPAddressResponse, error)
	ListInstances(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Instance, error)
	CreateInstance(ctx context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error)
	BootInstance(ctx context.Context, linodeID int, configID int) error
	ListInstanceConfigs(ctx context.Context, linodeID int, opts *linodego.ListOptions) ([]linodego.InstanceConfig, error)
	UpdateInstanceConfig(ctx context.Context, linodeID int, configID int, opts linodego.InstanceConfigUpdateOptions) (*linodego.InstanceConfig, error)
	GetInstanceDisk(ctx context.Context, linodeID int, diskID int) (*linodego.InstanceDisk, error)
	ResizeInstanceDisk(ctx context.Context, linodeID int, diskID int, size int) error
	CreateInstanceDisk(ctx context.Context, linodeID int, opts linodego.InstanceDiskCreateOptions) (*linodego.InstanceDisk, error)
	GetInstance(ctx context.Context, linodeID int) (*linodego.Instance, error)
	DeleteInstance(ctx context.Context, linodeID int) error
	GetRegion(ctx context.Context, regionID string) (*linodego.Region, error)
	GetImage(ctx context.Context, imageID string) (*linodego.Image, error)
	CreateStackscript(ctx context.Context, opts linodego.StackscriptCreateOptions) (*linodego.Stackscript, error)
	ListStackscripts(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Stackscript, error)
	GetType(ctx context.Context, typeID string) (*linodego.LinodeType, error)
}

// LinodeVPCClient defines the methods that interact with Linode's VPC service.
type LinodeVPCClient interface {
	GetVPC(ctx context.Context, vpcID int) (*linodego.VPC, error)
	ListVPCs(ctx context.Context, opts *linodego.ListOptions) ([]linodego.VPC, error)
	CreateVPC(ctx context.Context, opts linodego.VPCCreateOptions) (*linodego.VPC, error)
	DeleteVPC(ctx context.Context, vpcID int) error
}

// LinodeNodeBalancerClient defines the methods that interact with Linode's Node Balancer service.
type LinodeNodeBalancerClient interface {
	CreateNodeBalancer(ctx context.Context, opts linodego.NodeBalancerCreateOptions) (*linodego.NodeBalancer, error)
	GetNodeBalancer(ctx context.Context, nodebalancerID int) (*linodego.NodeBalancer, error)
	ListNodeBalancerNodes(ctx context.Context, nodebalancerID int, configID int, opts *linodego.ListOptions) ([]linodego.NodeBalancerNode, error)
	GetNodeBalancerConfig(ctx context.Context, nodebalancerID int, configID int) (*linodego.NodeBalancerConfig, error)
	CreateNodeBalancerConfig(ctx context.Context, nodebalancerID int, opts linodego.NodeBalancerConfigCreateOptions) (*linodego.NodeBalancerConfig, error)
	DeleteNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, nodeID int) error
	DeleteNodeBalancer(ctx context.Context, nodebalancerID int) error
	CreateNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, opts linodego.NodeBalancerNodeCreateOptions) (*linodego.NodeBalancerNode, error)
}

// LinodeObjectStorageClient defines the methods that interact with Linode's Object Storage service.
type LinodeObjectStorageClient interface {
	GetObjectStorageBucket(ctx context.Context, regionID, label string) (*linodego.ObjectStorageBucket, error)
	CreateObjectStorageBucket(ctx context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	GetObjectStorageBucketAccess(ctx context.Context, clusterOrRegionID, label string) (*linodego.ObjectStorageBucketAccess, error)
	UpdateObjectStorageBucketAccess(ctx context.Context, clusterOrRegionID, label string, opts linodego.ObjectStorageBucketUpdateAccessOptions) error
	GetObjectStorageKey(ctx context.Context, keyID int) (*linodego.ObjectStorageKey, error)
	CreateObjectStorageKey(ctx context.Context, opts linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(ctx context.Context, keyID int) error
}

// LinodeDNSClient defines the methods that interact with Linode's Domains service.
type LinodeDNSClient interface {
	CreateDomainRecord(ctx context.Context, domainID int, recordReq linodego.DomainRecordCreateOptions) (*linodego.DomainRecord, error)
	UpdateDomainRecord(ctx context.Context, domainID int, domainRecordID int, recordReq linodego.DomainRecordUpdateOptions) (*linodego.DomainRecord, error)
	ListDomainRecords(ctx context.Context, domainID int, opts *linodego.ListOptions) ([]linodego.DomainRecord, error)
	ListDomains(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Domain, error)
	DeleteDomainRecord(ctx context.Context, domainID int, domainRecordID int) error
}

// LinodePlacementGroupClient defines the methods that interact with Linode's PlacementGroup service.
type LinodePlacementGroupClient interface {
	GetPlacementGroup(ctx context.Context, id int) (*linodego.PlacementGroup, error)
	ListPlacementGroups(ctx context.Context, options *linodego.ListOptions) ([]linodego.PlacementGroup, error)
	CreatePlacementGroup(ctx context.Context, opts linodego.PlacementGroupCreateOptions) (*linodego.PlacementGroup, error)
	DeletePlacementGroup(ctx context.Context, id int) error
	UpdatePlacementGroup(ctx context.Context, id int, options linodego.PlacementGroupUpdateOptions) (*linodego.PlacementGroup, error)
	AssignPlacementGroupLinodes(ctx context.Context, id int, options linodego.PlacementGroupAssignOptions) (*linodego.PlacementGroup, error)
	UnassignPlacementGroupLinodes(ctx context.Context, id int, options linodego.PlacementGroupUnAssignOptions) (*linodego.PlacementGroup, error)
}

// LinodeFirewallClient defines the methods that interact with Linode's Firewall service.
type LinodeFirewallClient interface {
	CreateFirewall(ctx context.Context, opts linodego.FirewallCreateOptions) (*linodego.Firewall, error)
	GetFirewall(ctx context.Context, firewallID int) (*linodego.Firewall, error)
	GetFirewallDevice(ctx context.Context, firewallID, deviceID int) (*linodego.FirewallDevice, error)
	GetFirewallRules(ctx context.Context, firewallID int) (*linodego.FirewallRuleSet, error)
	UpdateFirewall(ctx context.Context, firewallID int, opts linodego.FirewallUpdateOptions) (*linodego.Firewall, error)
	UpdateFirewallRules(ctx context.Context, firewallID int, rules linodego.FirewallRuleSet) (*linodego.FirewallRuleSet, error)
	DeleteFirewall(ctx context.Context, firewallID int) error
	DeleteFirewallDevice(ctx context.Context, firewallID, deviceID int) error
}

type K8sClient interface {
	client.Client
}

type LinodeTokenClient interface {
	SetToken(token string) *linodego.Client
}
