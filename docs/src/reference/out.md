# API Reference

## Packages
- [infrastructure.cluster.x-k8s.io/v1alpha2](#infrastructureclusterx-k8siov1alpha2)
- [infrastructure.cluster.x-k8s.io/v1beta1](#infrastructureclusterx-k8siov1beta1)


## infrastructure.cluster.x-k8s.io/v1alpha2

Package v1alpha2 contains API Schema definitions for the infrastructure v1alpha2 API group

### Resource Types
- [AddressSet](#addressset)
- [AddressSetList](#addresssetlist)
- [FirewallRule](#firewallrule)
- [FirewallRuleList](#firewallrulelist)
- [LinodeCluster](#linodecluster)
- [LinodeClusterList](#linodeclusterlist)
- [LinodeClusterTemplate](#linodeclustertemplate)
- [LinodeClusterTemplateList](#linodeclustertemplatelist)
- [LinodeFirewall](#linodefirewall)
- [LinodeFirewallList](#linodefirewalllist)
- [LinodeMachine](#linodemachine)
- [LinodeMachineList](#linodemachinelist)
- [LinodeMachineTemplate](#linodemachinetemplate)
- [LinodeMachineTemplateList](#linodemachinetemplatelist)
- [LinodeObjectStorageBucket](#linodeobjectstoragebucket)
- [LinodeObjectStorageBucketList](#linodeobjectstoragebucketlist)
- [LinodeObjectStorageKey](#linodeobjectstoragekey)
- [LinodeObjectStorageKeyList](#linodeobjectstoragekeylist)
- [LinodePlacementGroup](#linodeplacementgroup)
- [LinodePlacementGroupList](#linodeplacementgrouplist)
- [LinodeVPC](#linodevpc)
- [LinodeVPCList](#linodevpclist)



#### AddressSet



AddressSet is the Schema for the addresssets API



_Appears in:_
- [AddressSetList](#addresssetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `AddressSet` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[AddressSetSpec](#addresssetspec)_ | spec is the desired state of the AddressSet |  | MinProperties: 1 <br /> |
| `status` _[AddressSetStatus](#addresssetstatus)_ | status is the observed state of the AddressSet |  |  |


#### AddressSetList



AddressSetList contains a list of AddressSet





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `AddressSetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AddressSet](#addressset) array_ | items is a list of AddressSet |  |  |


#### AddressSetSpec



AddressSetSpec defines the desired state of AddressSet

_Validation:_
- MinProperties: 1

_Appears in:_
- [AddressSet](#addressset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _string_ | ipv4 defines a list of IPv4 address strings |  |  |
| `ipv6` _string_ | ipv6 defines a list of IPv6 address strings |  |  |


#### AddressSetStatus



AddressSetStatus defines the observed state of AddressSet



_Appears in:_
- [AddressSet](#addressset)



#### BucketAccessRef







_Appears in:_
- [LinodeObjectStorageKeySpec](#linodeobjectstoragekeyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketName` _string_ | bucketName is the name of the bucket to grant access to. |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `permissions` _string_ | permissions is the permissions to grant to the bucket. |  | Enum: [read_only read_write] <br /> |
| `region` _string_ | region is the region of the bucket. |  | MinLength: 1 <br /> |


#### FirewallRule



FirewallRule is the Schema for the firewallrules API



_Appears in:_
- [FirewallRuleList](#firewallrulelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `FirewallRule` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[FirewallRuleSpec](#firewallrulespec)_ | spec is the desired state of the FirewallRule. |  |  |
| `status` _[FirewallRuleStatus](#firewallrulestatus)_ | status is the observed state of the FirewallRule. |  |  |


#### FirewallRuleList



FirewallRuleList contains a list of FirewallRule





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `FirewallRuleList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[FirewallRule](#firewallrule) array_ | items is a list of FirewallRule. |  |  |


#### FirewallRuleSpec



FirewallRuleSpec defines the desired state of FirewallRule.



_Appears in:_
- [FirewallRule](#firewallrule)
- [LinodeFirewallSpec](#linodefirewallspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `action` _string_ | action is the action to take when the rule matches. |  | Enum: [ACCEPT DROP] <br /> |
| `label` _string_ | label is the label of the rule. |  | MinLength: 3 <br /> |
| `description` _string_ | description is the description of the rule. |  |  |
| `ports` _string_ | ports is the ports to apply the rule to. |  |  |
| `protocol` _[NetworkProtocol](#networkprotocol)_ | protocol is the protocol to apply the rule to. |  | Enum: [TCP UDP ICMP IPENCAP] <br /> |
| `addresses` _[NetworkAddresses](#networkaddresses)_ | addresses is a list of addresses to apply the rule to. |  |  |
| `addressSetRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | addressSetRefs is a list of references to AddressSets as an alternative to<br />using Addresses but can be used in conjunction with it. |  |  |


#### FirewallRuleStatus



FirewallRuleStatus defines the observed state of FirewallRule.



_Appears in:_
- [FirewallRule](#firewallrule)



#### FirewallStatusError

_Underlying type:_ _string_

FirewallStatusError defines errors states for Firewall objects.



_Appears in:_
- [LinodeFirewallStatus](#linodefirewallstatus)

| Field | Description |
| --- | --- |
| `CreateError` | CreateFirewallError indicates that an error was encountered<br />when trying to create the Firewall.<br /> |
| `UpdateError` | UpdateFirewallError indicates that an error was encountered<br />when trying to update the Firewall.<br /> |
| `DeleteError` | DeleteFirewallError indicates that an error was encountered<br />when trying to delete the Firewall.<br /> |


#### GeneratedSecret







_Appears in:_
- [LinodeObjectStorageKeySpec](#linodeobjectstoragekeyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the generated Secret. If not set, the name is formatted as "\{name-of-obj-key\}-obj-key". |  |  |
| `namespace` _string_ | namespace for the generated Secret. If not set, defaults to the namespace of the LinodeObjectStorageKey. |  |  |
| `type` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secrettype-v1-core)_ | type of the generated Secret. | Opaque | Enum: [Opaque addons.cluster.x-k8s.io/resource-set] <br /> |
| `format` _object (keys:string, values:string)_ | format of the data stored in the generated Secret.<br />It supports Go template syntax and interpolating the following values: .AccessKey .SecretKey .BucketName .BucketEndpoint .S3Endpoint<br />If no format is supplied, then a generic one is used containing the values specified. |  |  |


#### IPv6CreateOptions



IPv6CreateOptions defines the IPv6 options for the instance.



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableSLAAC` _boolean_ | enableSLAAC is an option to enable SLAAC (Stateless Address Autoconfiguration) for the instance.<br />This is useful for IPv6 addresses, allowing the instance to automatically configure its own IPv6 address.<br />Defaults to false. |  |  |
| `enableRanges` _boolean_ | enableRanges is an option to enable IPv6 ranges for the instance.<br />If set to true, the instance will have a range of IPv6 addresses.<br />This is useful for instances that require multiple IPv6 addresses.<br />Defaults to false. |  |  |
| `isPublicIPv6` _boolean_ | isPublicIPv6 is an option to enable public IPv6 for the instance.<br />If set to true, the instance will have a publicly routable IPv6 range.<br />Defaults to false. |  |  |


#### InstanceConfigInterfaceCreateOptions



InstanceConfigInterfaceCreateOptions defines network interface config



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipamAddress` _string_ | ipamAddress is the IP address to assign to the interface. |  |  |
| `label` _string_ | label is the label of the interface. |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `purpose` _[ConfigInterfacePurpose](#configinterfacepurpose)_ | purpose is the purpose of the interface. |  |  |
| `primary` _boolean_ | primary is a boolean indicating whether the interface is primary. |  |  |
| `subnetId` _integer_ | subnetId is the ID of the subnet to use for the interface. |  |  |
| `ipv4` _[VPCIPv4](#vpcipv4)_ | ipv4 is the IPv4 configuration for the interface. |  |  |
| `ipRanges` _string array_ | ipRanges is a list of IPv4 ranges to assign to the interface. |  |  |


#### InstanceConfiguration



InstanceConfiguration defines the instance configuration



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kernel` _string_ | kernel is a Kernel ID to boot a Linode with. (e.g linode/latest-64bit). |  |  |


#### InstanceDisk



InstanceDisk defines a list of disks to use for an instance



_Appears in:_
- [InstanceDisks](#instancedisks)
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `diskID` _integer_ | diskID is the linode assigned ID of the disk. |  |  |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#quantity-resource-api)_ | size of the disk in resource.Quantity notation. |  |  |
| `label` _string_ | label for the instance disk, if nothing is provided, it will match the device name. |  |  |
| `filesystem` _string_ | filesystem of disk to provision, the default disk filesystem is "ext4". |  | Enum: [raw swap ext3 ext4 initrd] <br /> |


#### InstanceDisks







_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sdb` _[InstanceDisk](#instancedisk)_ | sdb is a disk for the instance. |  |  |
| `sdc` _[InstanceDisk](#instancedisk)_ | sdc is a disk for the instance. |  |  |
| `sdd` _[InstanceDisk](#instancedisk)_ | sdd is a disk for the instance. |  |  |
| `sde` _[InstanceDisk](#instancedisk)_ | sde is a disk for the instance. |  |  |
| `sdf` _[InstanceDisk](#instancedisk)_ | sdf is a disk for the instance. |  |  |
| `sdg` _[InstanceDisk](#instancedisk)_ | sdg is a disk for the instance. |  |  |
| `sdh` _[InstanceDisk](#instancedisk)_ | sdh is a disk for the instance. |  |  |




#### InterfaceDefaultRoute



InterfaceDefaultRoute defines the default IPv4 and IPv6 routes for an interface



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _boolean_ | ipv4 is the IPv4 default route for the interface. |  |  |
| `ipv6` _boolean_ | ipv6 is the IPv6 default route for the interface. |  |  |


#### LinodeCluster



LinodeCluster is the Schema for the linodeclusters API



_Appears in:_
- [LinodeClusterList](#linodeclusterlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeCluster` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ | spec is the desired state of the LinodeCluster. |  |  |
| `status` _[LinodeClusterStatus](#linodeclusterstatus)_ | status is the observed state of the LinodeCluster. |  |  |


#### LinodeClusterList



LinodeClusterList contains a list of LinodeCluster





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeClusterList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeCluster](#linodecluster) array_ | items is a list of LinodeCluster. |  |  |


#### LinodeClusterSpec



LinodeClusterSpec defines the desired state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)
- [LinodeClusterTemplateResource](#linodeclustertemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | region the LinodeCluster lives in. |  | MinLength: 1 <br /> |
| `controlPlaneEndpoint` _[APIEndpoint](#apiendpoint)_ | controlPlaneEndpoint represents the endpoint used to communicate with the LinodeCluster control plane<br />If ControlPlaneEndpoint is unset then the Nodebalancer ip will be used. |  |  |
| `network` _[NetworkSpec](#networkspec)_ | network encapsulates all things related to Linode network. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | vpcRef is a reference to a VPC object. This makes the Linodes use the specified VPC. |  |  |
| `vpcID` _integer_ | vpcID is the ID of an existing VPC in Linode. |  |  |
| `nodeBalancerFirewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | nodeBalancerFirewallRef is a reference to a NodeBalancer Firewall object. This makes the linode use the specified NodeBalancer Firewall. |  |  |
| `objectStore` _[ObjectStore](#objectstore)_ | objectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for<br />bootstrapping (e.g. Cloud-init). |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not<br /> supplied, then the credentials of the controller will be used. |  |  |


#### LinodeClusterStatus



LinodeClusterStatus defines the observed state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeCluster. |  |  |
| `ready` _boolean_ | ready denotes that the cluster (infrastructure) is ready. |  |  |
| `failureReason` _string_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a succinct value suitable<br />for machine interpretation. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a more verbose string suitable<br />for logging and human consumption. |  |  |


#### LinodeClusterTemplate



LinodeClusterTemplate is the Schema for the linodeclustertemplates API



_Appears in:_
- [LinodeClusterTemplateList](#linodeclustertemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeClusterTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeClusterTemplateSpec](#linodeclustertemplatespec)_ | spec is the desired state of the LinodeClusterTemplate. |  |  |


#### LinodeClusterTemplateList



LinodeClusterTemplateList contains a list of LinodeClusterTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeClusterTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeClusterTemplate](#linodeclustertemplate) array_ | items is a list of LinodeClusterTemplate. |  |  |


#### LinodeClusterTemplateResource



LinodeClusterTemplateResource describes the data needed to create a LinodeCluster from a template.



_Appears in:_
- [LinodeClusterTemplateSpec](#linodeclustertemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ | spec is the specification of the LinodeCluster. |  |  |


#### LinodeClusterTemplateSpec



LinodeClusterTemplateSpec defines the desired state of LinodeClusterTemplate



_Appears in:_
- [LinodeClusterTemplate](#linodeclustertemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeClusterTemplateResource](#linodeclustertemplateresource)_ | template defines the specification for a LinodeCluster. |  |  |


#### LinodeFirewall



LinodeFirewall is the Schema for the linodefirewalls API



_Appears in:_
- [LinodeFirewallList](#linodefirewalllist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeFirewall` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeFirewallSpec](#linodefirewallspec)_ | spec is the desired state of the LinodeFirewall. |  | MinProperties: 1 <br /> |
| `status` _[LinodeFirewallStatus](#linodefirewallstatus)_ | status is the observed state of the LinodeFirewall. |  |  |


#### LinodeFirewallList



LinodeFirewallList contains a list of LinodeFirewall





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeFirewallList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeFirewall](#linodefirewall) array_ | items is a list of LinodeFirewall. |  |  |


#### LinodeFirewallSpec



LinodeFirewallSpec defines the desired state of LinodeFirewall

_Validation:_
- MinProperties: 1

_Appears in:_
- [LinodeFirewall](#linodefirewall)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `firewallID` _integer_ | firewallID is the ID of the Firewall. |  |  |
| `enabled` _boolean_ | enabled determines if the Firewall is enabled. Defaults to false if not defined. | false |  |
| `inboundRules` _[FirewallRuleSpec](#firewallrulespec) array_ | inboundRules is a list of FirewallRules that will be applied to the Firewall. |  |  |
| `inboundRuleRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | inboundRuleRefs is a list of references to FirewallRules as an alternative to<br />using InboundRules but can be used in conjunction with it |  |  |
| `inboundPolicy` _string_ | inboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined. | ACCEPT | Enum: [ACCEPT DROP] <br /> |
| `outboundRules` _[FirewallRuleSpec](#firewallrulespec) array_ | outboundRules is a list of FirewallRules that will be applied to the Firewall. |  |  |
| `outboundRuleRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | outboundRuleRefs is a list of references to FirewallRules as an alternative to<br />using OutboundRules but can be used in conjunction with it |  |  |
| `outboundPolicy` _string_ | outboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined. | ACCEPT | Enum: [ACCEPT DROP] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this Firewall. If not<br />supplied then the credentials of the controller will be used. |  |  |


#### LinodeFirewallStatus



LinodeFirewallStatus defines the observed state of LinodeFirewall



_Appears in:_
- [LinodeFirewall](#linodefirewall)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeFirewall. |  |  |
| `ready` _boolean_ | ready is true when the provider resource is ready. | false |  |
| `failureReason` _[FirewallStatusError](#firewallstatuserror)_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the Firewall and will contain a succinct value suitable<br />for machine interpretation.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Firewall's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Firewalls<br />can be added as events to the Firewall object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the Firewall and will contain a more verbose string suitable<br />for logging and human consumption.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Firewall's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Firewalls<br />can be added as events to the Firewall object and/or logged in the<br />controller's output. |  |  |


#### LinodeInterfaceCreateOptions



LinodeInterfaceCreateOptions defines the linode network interface config



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `firewallID` _integer_ | firewallID is the ID of the firewall to use for the interface. |  |  |
| `defaultRoute` _[InterfaceDefaultRoute](#interfacedefaultroute)_ | defaultRoute is the default route for the interface. |  |  |
| `public` _[PublicInterfaceCreateOptions](#publicinterfacecreateoptions)_ | public is the public interface configuration for the interface. |  |  |
| `vpc` _[VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)_ | vpc is the VPC interface configuration for the interface. |  |  |
| `vlan` _[VLANInterface](#vlaninterface)_ | vlan is the VLAN interface configuration for the interface. |  |  |


#### LinodeMachine



LinodeMachine is the Schema for the linodemachines API



_Appears in:_
- [LinodeMachineList](#linodemachinelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachine` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ | spec defines the specification of desired behavior for the LinodeMachine. |  |  |
| `status` _[LinodeMachineStatus](#linodemachinestatus)_ | status defines the observed state of LinodeMachine. |  |  |


#### LinodeMachineList



LinodeMachineList contains a list of LinodeMachine





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachineList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachine](#linodemachine) array_ | items is a list of LinodeMachine. |  |  |


#### LinodeMachineSpec



LinodeMachineSpec defines the desired state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)
- [LinodeMachineTemplateResource](#linodemachinetemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `providerID` _string_ | providerID is the unique identifier as specified by the cloud provider. |  |  |
| `instanceID` _integer_ | instanceID is the Linode instance ID for this machine. |  |  |
| `region` _string_ | region is the Linode region to create the instance in. |  | MinLength: 1 <br /> |
| `type` _string_ | type is the Linode instance type to create. |  | MinLength: 1 <br /> |
| `group` _string_ | group is the Linode group to create the instance in.<br />Deprecated: group is a deprecated property denoting a group label for the Linode. |  |  |
| `rootPass` _string_ | rootPass is the root password for the instance. |  |  |
| `authorizedKeys` _string array_ | authorizedKeys is a list of SSH public keys to add to the instance. |  |  |
| `authorizedUsers` _string array_ | authorizedUsers is a list of usernames to add to the instance. |  |  |
| `backupID` _integer_ | backupID is the ID of the backup to restore the instance from. |  |  |
| `image` _string_ | image is the Linode image to use for the instance. |  |  |
| `interfaces` _[InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions) array_ | interfaces is a list of legacy network interfaces to use for the instance. |  |  |
| `linodeInterfaces` _[LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions) array_ | linodeInterfaces is a list of Linode network interfaces to use for the instance. Requires Linode Interfaces beta opt-in to use. |  |  |
| `backupsEnabled` _boolean_ | backupsEnabled is a boolean indicating whether backups should be enabled for the instance. |  |  |
| `privateIP` _boolean_ | privateIP is a boolean indicating whether the instance should have a private IP address. |  |  |
| `tags` _string array_ | tags is a list of tags to apply to the Linode instance. |  |  |
| `firewallID` _integer_ | firewallID is the id of the cloud firewall to apply to the Linode Instance |  |  |
| `osDisk` _[InstanceDisk](#instancedisk)_ | osDisk is a configuration for the root disk that includes the OS,<br />if not specified, this defaults to whatever space is not taken up by the DataDisks |  |  |
| `dataDisks` _[InstanceDisks](#instancedisks)_ | dataDisks is a map of any additional disks to add to an instance,<br />The sum of these disks + the OSDisk must not be more than allowed on a linodes plan |  |  |
| `diskEncryption` _string_ | diskEncryption determines if the disks of the instance should be encrypted. The default is disabled. |  | Enum: [enabled disabled] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials<br />to use for provisioning this machine. If not supplied then these<br />credentials will be used in-order:<br />  1. LinodeMachine<br />  2. Owner LinodeCluster<br />  3. Controller |  |  |
| `configuration` _[InstanceConfiguration](#instanceconfiguration)_ | configuration is the Akamai instance configuration OS,<br />if not specified, this defaults to the default configuration associated to the instance. |  |  |
| `placementGroupRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | placementGroupRef is a reference to a placement group object. This makes the linode to be launched in that specific group. |  |  |
| `firewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | firewallRef is a reference to a firewall object. This makes the linode use the specified firewall. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | vpcRef is a reference to a LinodeVPC resource. If specified, this takes precedence over<br />the cluster-level VPC configuration for multi-region support. |  |  |
| `vpcID` _integer_ | vpcID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL. |  |  |
| `ipv6Options` _[IPv6CreateOptions](#ipv6createoptions)_ | ipv6Options defines the IPv6 options for the instance.<br />If not specified, IPv6 ranges won't be allocated to instance. |  |  |
| `networkHelper` _boolean_ | networkHelper is an option usually enabled on account level. It helps configure networking automatically for instances.<br />You can use this to enable/disable the network helper for a specific instance.<br />For more information, see https://techdocs.akamai.com/cloud-computing/docs/automatically-configure-networking<br />Defaults to true. |  |  |
| `interfaceGeneration` _[InterfaceGeneration](#interfacegeneration)_ | interfaceGeneration is the generation of the interface to use for the cluster's<br />nodes in interface / linodeInterface are not specified for a LinodeMachine.<br />If not set, defaults to "legacy_config". | legacy_config | Enum: [legacy_config linode] <br /> |


#### LinodeMachineStatus



LinodeMachineStatus defines the observed state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeMachine. |  |  |
| `ready` _boolean_ | ready is true when the provider resource is ready. | false |  |
| `addresses` _MachineAddress array_ | addresses contains the Linode instance associated addresses. |  |  |
| `cloudinitMetadataSupport` _boolean_ | cloudinitMetadataSupport determines whether to use cloud-init or not.<br />Deprecated: stackscript are no longer in use, so this field is not used. | true |  |
| `instanceState` _[InstanceStatus](#instancestatus)_ | instanceState is the state of the Linode instance for this machine. |  |  |
| `failureReason` _string_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a succinct value suitable<br />for machine interpretation.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a more verbose string suitable<br />for logging and human consumption.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `tags` _string array_ | tags are the tags applied to the Linode Machine. |  |  |


#### LinodeMachineTemplate



LinodeMachineTemplate is the Schema for the linodemachinetemplates API



_Appears in:_
- [LinodeMachineTemplateList](#linodemachinetemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachineTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeMachineTemplateSpec](#linodemachinetemplatespec)_ | spec is the desired state of the LinodeMachineTemplate. |  |  |
| `status` _[LinodeMachineTemplateStatus](#linodemachinetemplatestatus)_ | status is the observed state of the LinodeMachineTemplate. |  |  |


#### LinodeMachineTemplateList



LinodeMachineTemplateList contains a list of LinodeMachineTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachineTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachineTemplate](#linodemachinetemplate) array_ | items is a list of LinodeMachineTemplate. |  |  |


#### LinodeMachineTemplateResource



LinodeMachineTemplateResource describes the data needed to create a LinodeMachine from a template.



_Appears in:_
- [LinodeMachineTemplateSpec](#linodemachinetemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ | spec is the specification of the desired behavior of the machine. |  |  |


#### LinodeMachineTemplateSpec



LinodeMachineTemplateSpec defines the desired state of LinodeMachineTemplate



_Appears in:_
- [LinodeMachineTemplate](#linodemachinetemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeMachineTemplateResource](#linodemachinetemplateresource)_ | template defines the specification for a LinodeMachine. |  |  |


#### LinodeMachineTemplateStatus



LinodeMachineTemplateStatus defines the observed state of LinodeMachineTemplate
It is used to store the status of the LinodeMachineTemplate, such as tags.



_Appears in:_
- [LinodeMachineTemplate](#linodemachinetemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeMachineTemplate |  |  |
| `tags` _string array_ | tags that are currently applied to the LinodeMachineTemplate. |  |  |
| `firewallID` _integer_ | firewallID that is currently applied to the LinodeMachineTemplate. |  |  |


#### LinodeNBPortConfig







_Appears in:_
- [NetworkSpec](#networkspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `port` _integer_ | port configured on the NodeBalancer. It must be valid port range (1-65535). |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `nodeBalancerConfigID` _integer_ | nodeBalancerConfigID is the config ID of port's NodeBalancer config. |  |  |


#### LinodeObjectStorageBucket



LinodeObjectStorageBucket is the Schema for the linodeobjectstoragebuckets API



_Appears in:_
- [LinodeObjectStorageBucketList](#linodeobjectstoragebucketlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageBucket` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeObjectStorageBucketSpec](#linodeobjectstoragebucketspec)_ | spec is the desired state of the LinodeObjectStorageBucket. |  |  |
| `status` _[LinodeObjectStorageBucketStatus](#linodeobjectstoragebucketstatus)_ | status is the observed state of the LinodeObjectStorageBucket. |  |  |


#### LinodeObjectStorageBucketList



LinodeObjectStorageBucketList contains a list of LinodeObjectStorageBucket





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageBucketList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeObjectStorageBucket](#linodeobjectstoragebucket) array_ | items is a list of LinodeObjectStorageBucket. |  |  |


#### LinodeObjectStorageBucketSpec



LinodeObjectStorageBucketSpec defines the desired state of LinodeObjectStorageBucket



_Appears in:_
- [LinodeObjectStorageBucket](#linodeobjectstoragebucket)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | region is the ID of the Object Storage region for the bucket. |  | MinLength: 1 <br /> |
| `acl` _[ObjectStorageACL](#objectstorageacl)_ | acl sets the Access Control Level of the bucket using a canned ACL string | private | Enum: [private public-read authenticated-read public-read-write] <br /> |
| `corsEnabled` _boolean_ | corsEnabled enables for all origins in the bucket .If set to false, CORS is disabled for all origins in the bucket | true |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning the bucket.<br />If not supplied then the credentials of the controller will be used. |  |  |
| `accessKeyRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | accessKeyRef is a reference to a LinodeObjectStorageBucketKey for the bucket. |  |  |
| `forceDeleteBucket` _boolean_ | forceDeleteBucket enables the object storage bucket used to be deleted even if it contains objects. |  |  |


#### LinodeObjectStorageBucketStatus



LinodeObjectStorageBucketStatus defines the observed state of LinodeObjectStorageBucket



_Appears in:_
- [LinodeObjectStorageBucket](#linodeobjectstoragebucket)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeObjectStorageBucket. |  |  |
| `ready` _boolean_ | ready denotes that the bucket has been provisioned along with access keys. | false |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the Object Storage Bucket and will contain a verbose string<br />suitable for logging and human consumption. |  |  |
| `hostname` _string_ | hostname is the address assigned to the bucket. |  |  |
| `creationTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | creationTime specifies the creation timestamp for the bucket. |  |  |


#### LinodeObjectStorageKey



LinodeObjectStorageKey is the Schema for the linodeobjectstoragekeys API



_Appears in:_
- [LinodeObjectStorageKeyList](#linodeobjectstoragekeylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageKey` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeObjectStorageKeySpec](#linodeobjectstoragekeyspec)_ | spec is the desired state of the LinodeObjectStorageKey. |  |  |
| `status` _[LinodeObjectStorageKeyStatus](#linodeobjectstoragekeystatus)_ | status is the observed state of the LinodeObjectStorageKey. |  |  |


#### LinodeObjectStorageKeyList



LinodeObjectStorageKeyList contains a list of LinodeObjectStorageKey





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageKeyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeObjectStorageKey](#linodeobjectstoragekey) array_ | Items represent the list of LinodeObjectStorageKey objects. |  |  |


#### LinodeObjectStorageKeySpec



LinodeObjectStorageKeySpec defines the desired state of LinodeObjectStorageKey



_Appears in:_
- [LinodeObjectStorageKey](#linodeobjectstoragekey)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketAccess` _[BucketAccessRef](#bucketaccessref) array_ | bucketAccess is the list of object storage bucket labels which can be accessed using the key |  | MinItems: 1 <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for generating access keys.<br />If not supplied, then the credentials of the controller will be used. |  |  |
| `keyGeneration` _integer_ | keyGeneration may be modified to trigger a rotation of the access key. | 0 |  |
| `generatedSecret` _[GeneratedSecret](#generatedsecret)_ | generatedSecret configures the Secret to generate containing access key details. |  |  |
| `secretType` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secrettype-v1-core)_ | secretType instructs the controller what type of secret to generate containing access key details.<br />Deprecated: secretType is no longer supported, Use generatedSecret.type. |  | Enum: [Opaque addons.cluster.x-k8s.io/resource-set] <br /> |
| `secretDataFormat` _object (keys:string, values:string)_ | secretDataFormat instructs the controller how to format the data stored in the secret containing access key details.<br />Deprecated: secretDataFormat is no longer supported, please use generatedSecret.format. |  |  |


#### LinodeObjectStorageKeyStatus



LinodeObjectStorageKeyStatus defines the observed state of LinodeObjectStorageKey



_Appears in:_
- [LinodeObjectStorageKey](#linodeobjectstoragekey)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeObjectStorageKey. |  |  |
| `ready` _boolean_ | ready denotes that the key has been provisioned. | false |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the Object Storage Key and will contain a verbose string<br />suitable for logging and human consumption. |  |  |
| `creationTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | creationTime specifies the creation timestamp for the secret. |  |  |
| `lastKeyGeneration` _integer_ | lastKeyGeneration tracks the last known value of .spec.keyGeneration. |  |  |
| `accessKeyRef` _integer_ | accessKeyRef stores the ID for Object Storage key provisioned. |  |  |


#### LinodePlacementGroup



LinodePlacementGroup is the Schema for the linodeplacementgroups API



_Appears in:_
- [LinodePlacementGroupList](#linodeplacementgrouplist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodePlacementGroup` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodePlacementGroupSpec](#linodeplacementgroupspec)_ | spec is the desired state of the LinodePlacementGroup. |  |  |
| `status` _[LinodePlacementGroupStatus](#linodeplacementgroupstatus)_ | status is the observed state of the LinodePlacementGroup. |  |  |


#### LinodePlacementGroupList



LinodePlacementGroupList contains a list of LinodePlacementGroup





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodePlacementGroupList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodePlacementGroup](#linodeplacementgroup) array_ | items is a list of LinodePlacementGroup. |  |  |


#### LinodePlacementGroupSpec



LinodePlacementGroupSpec defines the desired state of LinodePlacementGroup



_Appears in:_
- [LinodePlacementGroup](#linodeplacementgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pgID` _integer_ | pgID is the ID of the PlacementGroup. |  |  |
| `region` _string_ | region is the Linode region to create the PlacementGroup in. |  | MinLength: 1 <br /> |
| `placementGroupPolicy` _string_ | placementGroupPolicy defines the policy for the PlacementGroup. | strict | Enum: [strict flexible] <br /> |
| `placementGroupType` _string_ | placementGroupType defines the type of the PlacementGroup. | anti_affinity:local | Enum: [anti_affinity:local] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this PlacementGroup.<br />If not supplied, then the credentials of the controller will be used. |  |  |


#### LinodePlacementGroupStatus



LinodePlacementGroupStatus defines the observed state of LinodePlacementGroup



_Appears in:_
- [LinodePlacementGroup](#linodeplacementgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodePlacementGroup. |  |  |
| `ready` _boolean_ | ready is true when the provider resource is ready. | false |  |
| `failureReason` _[LinodePlacementGroupStatusError](#linodeplacementgroupstatuserror)_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the PlacementGroup and will contain a succinct value suitable<br />for machine interpretation.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the PlacementGroup's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of PlacementGroups<br />can be added as events to the PlacementGroup object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the PlacementGroup and will contain a more verbose string suitable<br />for logging and human consumption.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the PlacementGroup's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of PlacementGroups<br />can be added as events to the PlacementGroup object and/or logged in the<br />controller's output. |  |  |


#### LinodePlacementGroupStatusError

_Underlying type:_ _string_

LinodePlacementGroupStatusError defines errors states for PlacementGroup objects.



_Appears in:_
- [LinodePlacementGroupStatus](#linodeplacementgroupstatus)

| Field | Description |
| --- | --- |
| `CreateError` | CreatePlacementGroupError indicates that an error was encountered<br />when trying to create the PlacementGroup.<br /> |
| `DeleteError` | DeletePlacementGroupError indicates that an error was encountered<br />when trying to delete the PlacementGroup.<br /> |


#### LinodeVPC



LinodeVPC is the Schema for the linodemachines API



_Appears in:_
- [LinodeVPCList](#linodevpclist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeVPC` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeVPCSpec](#linodevpcspec)_ | spec is the desired state of the LinodeVPC. |  |  |
| `status` _[LinodeVPCStatus](#linodevpcstatus)_ | status is the observed state of the LinodeVPC. |  |  |


#### LinodeVPCList



LinodeVPCList contains a list of LinodeVPC





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeVPCList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeVPC](#linodevpc) array_ | items is a list of LinodeVPC. |  |  |


#### LinodeVPCSpec



LinodeVPCSpec defines the desired state of LinodeVPC



_Appears in:_
- [LinodeVPC](#linodevpc)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vpcID` _integer_ | vpcID is the ID of the VPC. |  |  |
| `description` _string_ | description is the description of the VPC. |  |  |
| `region` _string_ | region is the region to create the VPC in. |  | MinLength: 1 <br /> |
| `ipv6` _VPCIPv6Range array_ | ipv6 is a list of IPv6 ranges allocated to the VPC.<br />Once ranges are allocated based on the IPv6Range field, they will be<br />added to this field. |  |  |
| `ipv6Range` _[VPCCreateOptionsIPv6](#vpccreateoptionsipv6) array_ | ipv6Range is a list of IPv6 ranges to allocate to the VPC.<br />If not specified, the VPC will not have an IPv6 range allocated.<br />Once ranges are allocated, they will be added to the IPv6 field. |  |  |
| `subnets` _[VPCSubnetCreateOptions](#vpcsubnetcreateoptions) array_ | subnets is a list of subnets to create in the VPC. |  |  |
| `retain` _boolean_ | retain allows you to keep the VPC after the LinodeVPC object is deleted.<br />This is useful if you want to use an existing VPC that was not created by this controller.<br />If set to true, the controller will not delete the VPC resource in Linode.<br />Defaults to false. | false |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this VPC.<br />If not supplied, then the credentials of the controller will be used. |  |  |


#### LinodeVPCStatus



LinodeVPCStatus defines the observed state of LinodeVPC



_Appears in:_
- [LinodeVPC](#linodevpc)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeVPC. |  |  |
| `ready` _boolean_ | ready is true when the provider resource is ready. | false |  |
| `failureReason` _[VPCStatusError](#vpcstatuserror)_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the VPC and will contain a succinct value suitable<br />for machine interpretation.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the VPC's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of VPCs<br />can be added as events to the VPC object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the VPC and will contain a more verbose string suitable<br />for logging and human consumption.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the VPC's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of VPCs<br />can be added as events to the VPC object and/or logged in the<br />controller's output. |  |  |


#### NetworkAddresses



NetworkAddresses holds a list of IPv4 and IPv6 addresses.
We don't use linodego here since kubebuilder can't generate DeepCopyInto
for linodego.NetworkAddresses



_Appears in:_
- [FirewallRuleSpec](#firewallrulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _string_ | ipv4 defines a list of IPv4 address strings. |  |  |
| `ipv6` _string_ | ipv6 defines a list of IPv6 address strings. |  |  |


#### NetworkSpec



NetworkSpec encapsulates Linode networking resources.



_Appears in:_
- [LinodeClusterSpec](#linodeclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `loadBalancerType` _string_ | loadBalancerType is the type of load balancer to use, defaults to NodeBalancer if not otherwise set. | NodeBalancer | Enum: [NodeBalancer dns external] <br /> |
| `dnsProvider` _string_ | dnsProvider is the provider who manages the domain.<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, defaults linode dns |  | Enum: [linode akamai] <br /> |
| `dnsRootDomain` _string_ | dnsRootDomain is the root domain used to create a DNS entry for the control-plane endpoint.<br />Ignored if the LoadBalancerType is set to anything other than dns |  |  |
| `dnsUniqueIdentifier` _string_ | dnsUniqueIdentifier is the unique identifier for the DNS. This let clusters with the same name have unique<br />DNS record<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, CAPL will create a unique identifier for you |  |  |
| `dnsTTLsec` _integer_ | dnsTTLsec is the TTL for the domain record<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, defaults to 30 |  |  |
| `dnsSubDomainOverride` _string_ | dnsSubDomainOverride is used to override CAPL's construction of the controlplane endpoint<br />If set, this will override the DNS subdomain from <clustername>-<uniqueid>.<rootdomain> to <overridevalue>.<rootdomain> |  |  |
| `apiserverLoadBalancerPort` _integer_ | apiserverLoadBalancerPort used by the api server. It must be valid ports range (1-65535).<br />If omitted, default value is 6443. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `nodeBalancerID` _integer_ | nodeBalancerID is the id of NodeBalancer. |  |  |
| `nodeBalancerFirewallID` _integer_ | nodeBalancerFirewallID is the id of NodeBalancer Firewall. |  |  |
| `apiserverNodeBalancerConfigID` _integer_ | apiserverNodeBalancerConfigID is the config ID of api server NodeBalancer config. |  |  |
| `additionalPorts` _[LinodeNBPortConfig](#linodenbportconfig) array_ | additionalPorts contains list of ports to be configured with NodeBalancer. |  |  |
| `subnetName` _string_ | subnetName is the name/label of the VPC subnet to be used by the cluster |  |  |
| `useVlan` _boolean_ | useVlan provisions a cluster that uses VLANs instead of VPCs. IPAM is managed internally. |  |  |
| `nodeBalancerBackendIPv4Range` _string_ | nodeBalancerBackendIPv4Range is the subnet range we want to provide for creating nodebalancer in VPC.<br />example: 10.10.10.0/30 |  |  |
| `enableVPCBackends` _boolean_ | enableVPCBackends toggles VPC-scoped NodeBalancer and VPC backend IP usage.<br />If set to false (default), the NodeBalancer will not be created in a VPC and<br />backends will use Linode private IPs. If true, the NodeBalancer will be<br />created in the configured VPC (when VPCRef or VPCID is set) and backends<br />will use VPC IPs. | false |  |


#### ObjectStorageACL

_Underlying type:_ _string_





_Appears in:_
- [LinodeObjectStorageBucketSpec](#linodeobjectstoragebucketspec)

| Field | Description |
| --- | --- |
| `private` |  |
| `public-read` |  |
| `authenticated-read` |  |
| `public-read-write` |  |


#### ObjectStore



ObjectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for
bootstrapping (e.g. Cloud-init).



_Appears in:_
- [LinodeClusterSpec](#linodeclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presignedURLDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | presignedURLDuration defines the duration for which presigned URLs are valid.<br />This is used to generate presigned URLs for S3 Bucket objects, which are used by<br />control-plane and worker nodes to fetch bootstrap data. |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for accessing the Cluster Object Store. |  |  |


#### PublicInterfaceCreateOptions



PublicInterfaceCreateOptions defines the IPv4 and IPv6 public interface create options



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _[PublicInterfaceIPv4CreateOptions](#publicinterfaceipv4createoptions)_ | ipv4 is the IPv4 configuration for the public interface. |  |  |
| `ipv6` _[PublicInterfaceIPv6CreateOptions](#publicinterfaceipv6createoptions)_ | ipv6 is the IPv6 configuration for the public interface. |  |  |


#### PublicInterfaceIPv4AddressCreateOptions



PublicInterfaceIPv4AddressCreateOptions defines the public IPv4 address and whether it is primary



_Appears in:_
- [PublicInterfaceIPv4CreateOptions](#publicinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | address is the IPv4 address for the public interface. |  | MinLength: 1 <br /> |
| `primary` _boolean_ | primary is a boolean indicating whether the address is primary. |  |  |


#### PublicInterfaceIPv4CreateOptions



PublicInterfaceIPv4CreateOptions defines the PublicInterfaceIPv4AddressCreateOptions for addresses



_Appears in:_
- [PublicInterfaceCreateOptions](#publicinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[PublicInterfaceIPv4AddressCreateOptions](#publicinterfaceipv4addresscreateoptions) array_ | addresses is the IPv4 addresses for the public interface. |  |  |


#### PublicInterfaceIPv6CreateOptions



PublicInterfaceIPv6CreateOptions defines the PublicInterfaceIPv6RangeCreateOptions



_Appears in:_
- [PublicInterfaceCreateOptions](#publicinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ranges` _[PublicInterfaceIPv6RangeCreateOptions](#publicinterfaceipv6rangecreateoptions) array_ | ranges is the IPv6 ranges for the public interface. |  |  |


#### PublicInterfaceIPv6RangeCreateOptions



PublicInterfaceIPv6RangeCreateOptions defines the IPv6 range for a public interface



_Appears in:_
- [PublicInterfaceIPv6CreateOptions](#publicinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the public interface. |  | MinLength: 1 <br /> |


#### VLANInterface



VLANInterface defines the VLAN interface configuration for an instance



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vlanLabel` _string_ | vlanLabel is the label of the VLAN. |  | MinLength: 1 <br /> |
| `ipamAddress` _string_ | ipamAddress is the IP address to assign to the interface. |  |  |


#### VPCCreateOptionsIPv6



VPCCreateOptionsIPv6 defines the options for creating an IPv6 range in a VPC.
It's copied from linodego.VPCCreateOptionsIPv6 and should be kept in sync.
Values supported by the linode API should be used here.
See https://techdocs.akamai.com/linode-api/reference/post-vpc for more details.



_Appears in:_
- [LinodeVPCSpec](#linodevpcspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 prefix for the VPC. |  |  |
| `allocationClass` _string_ | allocationClass is the IPv6 inventory from which the VPC prefix should be allocated. |  |  |


#### VPCIPv4



VPCIPv4 defines VPC IPV4 settings



_Appears in:_
- [InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vpc` _string_ | vpc is the ID of the VPC to use for the interface. |  |  |
| `nat1to1` _string_ | nat1to1 is the NAT 1:1 address for the interface. |  |  |


#### VPCInterfaceCreateOptions



VPCInterfaceCreateOptions defines the VPC interface configuration for an instance



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `subnetId` _integer_ | subnetId is the ID of the subnet to use for the interface. |  |  |
| `ipv4` _[VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)_ | ipv4 is the IPv4 configuration for the interface. |  |  |
| `ipv6` _[VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)_ | ipv6 is the IPv6 configuration for the interface. |  |  |


#### VPCInterfaceIPv4AddressCreateOptions



VPCInterfaceIPv4AddressCreateOptions defines the IPv4 configuration for a VPC interface



_Appears in:_
- [VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | address is the IPv4 address for the interface. |  | MinLength: 1 <br /> |
| `primary` _boolean_ | primary is a boolean indicating whether the address is primary. |  |  |
| `nat1to1Address` _string_ | nat1to1Address is the NAT 1:1 address for the interface. |  |  |


#### VPCInterfaceIPv4CreateOptions



VPCInterfaceIPv4CreateOptions defines the IPv4 address and range configuration for a VPC interface



_Appears in:_
- [VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[VPCInterfaceIPv4AddressCreateOptions](#vpcinterfaceipv4addresscreateoptions) array_ | addresses is the IPv4 addresses for the interface. |  |  |
| `ranges` _[VPCInterfaceIPv4RangeCreateOptions](#vpcinterfaceipv4rangecreateoptions) array_ | ranges is the IPv4 ranges for the interface. |  |  |


#### VPCInterfaceIPv4RangeCreateOptions



VPCInterfaceIPv4RangeCreateOptions defines the IPv4 range for a VPC interface



_Appears in:_
- [VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv4 range for the interface. |  | MinLength: 1 <br /> |


#### VPCInterfaceIPv6CreateOptions



VPCInterfaceIPv6CreateOptions defines the IPv6 configuration for a VPC interface



_Appears in:_
- [VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `slaac` _[VPCInterfaceIPv6SLAACCreateOptions](#vpcinterfaceipv6slaaccreateoptions) array_ | slaac is the IPv6 SLAAC configuration for the interface. |  |  |
| `ranges` _[VPCInterfaceIPv6RangeCreateOptions](#vpcinterfaceipv6rangecreateoptions) array_ | ranges is the IPv6 ranges for the interface. |  |  |
| `isPublic` _boolean_ | isPublic is a boolean indicating whether the interface is public. |  |  |


#### VPCInterfaceIPv6RangeCreateOptions



VPCInterfaceIPv6RangeCreateOptions defines the IPv6 range for a VPC interface



_Appears in:_
- [VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the interface. |  | MinLength: 1 <br /> |


#### VPCInterfaceIPv6SLAACCreateOptions



VPCInterfaceIPv6SLAACCreateOptions defines the Range for IPv6 SLAAC



_Appears in:_
- [VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the interface. |  | MinLength: 1 <br /> |


#### VPCStatusError

_Underlying type:_ _string_

VPCStatusError defines errors states for VPC objects.



_Appears in:_
- [LinodeVPCStatus](#linodevpcstatus)

| Field | Description |
| --- | --- |
| `CreateError` | CreateVPCError indicates that an error was encountered<br />when trying to create the VPC.<br /> |
| `UpdateError` | UpdateVPCError indicates that an error was encountered<br />when trying to update the VPC.<br /> |
| `DeleteError` | DeleteVPCError indicates that an error was encountered<br />when trying to delete the VPC.<br /> |


#### VPCSubnetCreateOptions



VPCSubnetCreateOptions defines subnet options



_Appears in:_
- [LinodeVPCSpec](#linodevpcspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `label` _string_ | label is the label of the subnet. |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `ipv4` _string_ | ipv4 is the IPv4 address range of the subnet. |  |  |
| `ipv6` _VPCIPv6Range array_ | ipv6 is a list of IPv6 ranges allocated to the subnet.<br />Once ranges are allocated based on the IPv6Range field, they will be<br />added to this field. |  |  |
| `ipv6Range` _[VPCSubnetCreateOptionsIPv6](#vpcsubnetcreateoptionsipv6) array_ | ipv6Range is a list of IPv6 ranges to allocate to the subnet.<br />If not specified, the subnet will not have an IPv6 range allocated.<br />Once ranges are allocated, they will be added to the IPv6 field. |  |  |
| `subnetID` _integer_ | subnetID is subnet id for the subnet |  |  |
| `retain` _boolean_ | retain allows you to keep the Subnet after the LinodeVPC object is deleted.<br />This is only applicable when the parent VPC has retain set to true. | false |  |


#### VPCSubnetCreateOptionsIPv6



VPCSubnetCreateOptionsIPv6 defines the options for creating an IPv6 range in a VPC subnet.
It's copied from linodego.VPCSubnetCreateOptionsIPv6 and should be kept in sync.
Values supported by the linode API should be used here.
See https://techdocs.akamai.com/linode-api/reference/post-vpc-subnet for more details.



_Appears in:_
- [VPCSubnetCreateOptions](#vpcsubnetcreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 prefix for the subnet. |  |  |



## infrastructure.cluster.x-k8s.io/v1beta1

Package v1beta1 contains API Schema definitions for the infrastructure v1beta1 API group

### Resource Types
- [LinodeCluster](#linodecluster)
- [LinodeClusterList](#linodeclusterlist)
- [LinodeClusterTemplate](#linodeclustertemplate)
- [LinodeClusterTemplateList](#linodeclustertemplatelist)
- [LinodeMachine](#linodemachine)
- [LinodeMachineList](#linodemachinelist)
- [LinodeMachineTemplate](#linodemachinetemplate)
- [LinodeMachineTemplateList](#linodemachinetemplatelist)



#### DNSConfig







_Appears in:_
- [NetworkSpec](#networkspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dnsProvider` _string_ | Provider is the provider who manages the domain. | linode | Enum: [linode akamai] <br /> |
| `dnsRootDomain` _string_ | dnsRootDomain is the root domain used to create a DNS entry for the control-plane endpoint. |  |  |
| `dnsUniqueIdentifier` _string_ | dnsUniqueIdentifier is the unique identifier for the DNS. This let clusters with the same name have unique<br />DNS record<br />If not set, CAPL will create a unique identifier for you |  |  |
| `dnsTTLsec` _integer_ | dnsTTLsec is the TTL for the domain record | 30 |  |
| `dnsSubDomainOverride` _string_ | dnsSubDomainOverride is used to override CAPL's construction of the controlplane endpoint<br />If set, this will override the DNS subdomain from <clustername>-<uniqueid>.<rootdomain> to <overridevalue>.<rootdomain> |  |  |


#### DataDisks



DataDisks defines additional data disks for an instance from sdb to sdh



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sdb` _[InstanceDisk](#instancedisk)_ | sdb is a disk for the instance. |  |  |
| `sdc` _[InstanceDisk](#instancedisk)_ | sdc is a disk for the instance. |  |  |
| `sdd` _[InstanceDisk](#instancedisk)_ | sdd is a disk for the instance. |  |  |
| `sde` _[InstanceDisk](#instancedisk)_ | sde is a disk for the instance. |  |  |
| `sdf` _[InstanceDisk](#instancedisk)_ | sdf is a disk for the instance. |  |  |
| `sdg` _[InstanceDisk](#instancedisk)_ | sdg is a disk for the instance. |  |  |
| `sdh` _[InstanceDisk](#instancedisk)_ | sdh is a disk for the instance. |  |  |


#### IPv6CreateOptions



IPv6CreateOptions defines the IPv6 options for the instance.



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableSLAAC` _boolean_ | enableSLAAC is an option to enable SLAAC (Stateless Address Autoconfiguration) for the instance.<br />This is useful for IPv6 addresses, allowing the instance to automatically configure its own IPv6 address.<br />Defaults to false. |  |  |
| `enableRanges` _boolean_ | enableRanges is an option to enable IPv6 ranges for the instance.<br />If set to true, the instance will have a range of IPv6 addresses.<br />This is useful for instances that require multiple IPv6 addresses.<br />Defaults to false. |  |  |
| `isPublicIPv6` _boolean_ | isPublicIPv6 is an option to enable public IPv6 for the instance.<br />If set to true, the instance will have a publicly routable IPv6 range.<br />Defaults to false. |  |  |


#### InstanceConfigInterfaceCreateOptions



InstanceConfigInterfaceCreateOptions defines network interface config



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipamAddress` _string_ | ipamAddress is the IP address to assign to the interface. |  |  |
| `label` _string_ | label is the label of the interface. |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `purpose` _[ConfigInterfacePurpose](#configinterfacepurpose)_ | purpose is the purpose of the interface. |  |  |
| `primary` _boolean_ | primary is a boolean indicating whether the interface is primary. |  |  |
| `subnetId` _integer_ | subnetId is the ID of the subnet to use for the interface. |  |  |
| `ipv4` _[VPCIPv4](#vpcipv4)_ | ipv4 is the IPv4 configuration for the interface. |  |  |
| `ipRanges` _string array_ | ipRanges is a list of IPv4 ranges to assign to the interface. |  |  |


#### InstanceDisk



InstanceDisk defines a disk for an instance



_Appears in:_
- [DataDisks](#datadisks)
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `diskID` _integer_ | diskID is the linode assigned ID of the disk. |  |  |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#quantity-resource-api)_ | size of the disk in resource.Quantity notation. |  |  |
| `label` _string_ | label for the instance disk, if nothing is provided, it will match the device name. |  |  |
| `filesystem` _string_ | filesystem of disk to provision, the default disk filesystem is "ext4". |  | Enum: [raw swap ext3 ext4 initrd] <br /> |




#### InterfaceDefaultRoute



InterfaceDefaultRoute defines the default IPv4 and IPv6 routes for an interface



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _boolean_ | ipv4 is the IPv4 default route for the interface. |  |  |
| `ipv6` _boolean_ | ipv6 is the IPv6 default route for the interface. |  |  |


#### LinodeCluster



LinodeCluster is the Schema for the linodeclusters API



_Appears in:_
- [LinodeClusterList](#linodeclusterlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeCluster` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ | spec is the desired state of the LinodeCluster. |  |  |
| `status` _[LinodeClusterStatus](#linodeclusterstatus)_ | status is the observed state of the LinodeCluster. |  |  |


#### LinodeClusterList



LinodeClusterList contains a list of LinodeCluster





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeClusterList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeCluster](#linodecluster) array_ | items is a list of LinodeCluster. |  |  |


#### LinodeClusterSpec



LinodeClusterSpec defines the desired state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)
- [LinodeClusterTemplateResource](#linodeclustertemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | region the LinodeCluster lives in. |  | MinLength: 1 <br /> |
| `controlPlaneEndpoint` _[APIEndpoint](#apiendpoint)_ | controlPlaneEndpoint represents the endpoint used to communicate with the LinodeCluster control plane<br />If ControlPlaneEndpoint is unset then the Nodebalancer ip will be used. |  |  |
| `network` _[NetworkSpec](#networkspec)_ | network encapsulates all things related to Linode network. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | vpcRef is a reference to a VPC object. This makes the Linodes use the specified VPC. |  |  |
| `vpcID` _integer_ | vpcID is the ID of an existing VPC in Linode. |  |  |
| `nodeBalancerFirewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | nodeBalancerFirewallRef is a reference to a NodeBalancer Firewall object. This makes the linode use the specified NodeBalancer Firewall. |  |  |
| `objectStore` _[ObjectStore](#objectstore)_ | objectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for<br />bootstrapping (e.g. Cloud-init). |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not<br /> supplied, then the credentials of the controller will be used. |  |  |


#### LinodeClusterStatus



LinodeClusterStatus defines the observed state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeCluster. |  |  |
| `ready` _boolean_ | ready denotes that the cluster (infrastructure) is ready. |  |  |
| `failureReason` _string_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a succinct value suitable<br />for machine interpretation. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a more verbose string suitable<br />for logging and human consumption. |  |  |


#### LinodeClusterTemplate



LinodeClusterTemplate is the Schema for the linodeclustertemplates API



_Appears in:_
- [LinodeClusterTemplateList](#linodeclustertemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeClusterTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeClusterTemplateSpec](#linodeclustertemplatespec)_ | spec is the desired state of the LinodeClusterTemplate. |  |  |


#### LinodeClusterTemplateList



LinodeClusterTemplateList contains a list of LinodeClusterTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeClusterTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeClusterTemplate](#linodeclustertemplate) array_ | items is a list of LinodeClusterTemplate. |  |  |


#### LinodeClusterTemplateResource



LinodeClusterTemplateResource describes the data needed to create a LinodeCluster from a template.



_Appears in:_
- [LinodeClusterTemplateSpec](#linodeclustertemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ | spec is the specification of the LinodeCluster. |  |  |


#### LinodeClusterTemplateSpec



LinodeClusterTemplateSpec defines the desired state of LinodeClusterTemplate



_Appears in:_
- [LinodeClusterTemplate](#linodeclustertemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeClusterTemplateResource](#linodeclustertemplateresource)_ | template defines the specification for a LinodeCluster. |  |  |


#### LinodeInterfaceCreateOptions



LinodeInterfaceCreateOptions defines the linode network interface config



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `firewallID` _integer_ | firewallID is the ID of the firewall to use for the interface. |  |  |
| `defaultRoute` _[InterfaceDefaultRoute](#interfacedefaultroute)_ | defaultRoute is the default route for the interface. |  |  |
| `public` _[PublicInterfaceCreateOptions](#publicinterfacecreateoptions)_ | public is the public interface configuration for the interface. |  |  |
| `vpc` _[VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)_ | vpc is the VPC interface configuration for the interface. |  |  |
| `vlan` _[VLANInterface](#vlaninterface)_ | vlan is the VLAN interface configuration for the interface. |  |  |


#### LinodeMachine



LinodeMachine is the Schema for the linodemachines API



_Appears in:_
- [LinodeMachineList](#linodemachinelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeMachine` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ | spec defines the specification of desired behavior for the LinodeMachine. |  |  |
| `status` _[LinodeMachineStatus](#linodemachinestatus)_ | status defines the observed state of LinodeMachine. |  |  |


#### LinodeMachineList



LinodeMachineList contains a list of LinodeMachine





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeMachineList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachine](#linodemachine) array_ | items is a list of LinodeMachine. |  |  |


#### LinodeMachineSpec



LinodeMachineSpec defines the desired state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)
- [LinodeMachineTemplateResource](#linodemachinetemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `providerID` _string_ | ProviderID is the unique identifier as specified by the cloud provider. |  |  |
| `instanceID` _integer_ | InstanceID is the Linode instance ID for this machine. |  |  |
| `osDisk` _[InstanceDisk](#instancedisk)_ | OSDisk is a configuration for the root disk that includes the OS,<br />if not specified, this defaults to whatever space is not taken up by the DataDisks |  |  |
| `dataDisks` _[DataDisks](#datadisks)_ | DataDisks is a map of any additional disks to add to an instance,<br />The sum of these disks + the OSDisk must not be more than allowed on the plan type |  |  |
| `kernel` _string_ | kernel is a Kernel ID to boot a Linode with. (e.g linode/latest-64bit). |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials<br />to use for provisioning this machine. If not supplied then these<br />credentials will be used in-order:<br />  1. LinodeMachine<br />  2. Owner LinodeCluster<br />  3. Controller |  |  |
| `placementGroupRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | PlacementGroupRef is a reference to a placement group object. This makes the linode to be launched in that specific group. |  |  |
| `firewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | FirewallRef is a reference to a firewall object. This makes the linode use the specified firewall. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | vpcRef is a reference to a LinodeVPC resource. If specified, this takes precedence over<br />the cluster-level VPC configuration for multi-region support. |  |  |
| `vpcID` _integer_ | VPCID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL. |  |  |
| `ipv6Options` _[IPv6CreateOptions](#ipv6createoptions)_ | IPv6Options defines the IPv6 options for the instance.<br />If not specified, IPv6 ranges won't be allocated to instance. |  |  |
| `region` _string_ | Region is the Linode region to create the instance in. |  | MinLength: 1 <br /> |
| `type` _string_ | Type is the Linode instance type to create. |  | MinLength: 1 <br /> |
| `rootPass` _string_ | RootPass is the root password for the instance. |  |  |
| `authorizedKeys` _string array_ | AuthorizedKeys is a list of SSH public keys to add to the instance. |  |  |
| `authorizedUsers` _string array_ | AuthorizedUsers is a list of usernames to add to the instance. |  |  |
| `backupID` _integer_ | BackupID is the ID of the backup to restore the instance from. |  |  |
| `image` _string_ | Image is the Linode image to use for the instance. |  |  |
| `backupsEnabled` _boolean_ | BackupsEnabled is a boolean indicating whether backups should be enabled for the instance. |  |  |
| `privateIP` _boolean_ | PrivateIP is a boolean indicating whether the instance should have a private IP address. |  |  |
| `networkHelper` _boolean_ | NetworkHelper is an option usually enabled on account level. It helps configure networking automatically for instances.<br />You can use this to enable/disable the network helper for a specific instance.<br />For more information, see https://techdocs.akamai.com/cloud-computing/docs/automatically-configure-networking<br />Defaults to true. |  |  |
| `tags` _string array_ | Tags is a list of tags to apply to the Linode instance. |  |  |
| `firewallID` _integer_ | FirewallID is the id of the cloud firewall to apply to the Linode Instance |  |  |
| `interfaceGeneration` _[InterfaceGeneration](#interfacegeneration)_ | InterfaceGeneration is the generation of the interface to use for the cluster's<br />nodes in interface / linodeInterface are not specified for a LinodeMachine.<br />If not set, defaults to "legacy_config". | legacy_config | Enum: [legacy_config linode] <br /> |
| `interfaces` _[InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions) array_ | Interfaces is a list of legacy network interfaces to use for the instance.<br />Conflicts with LinodeInterfaces. |  |  |
| `linodeInterfaces` _[LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions) array_ | LinodeInterfaces is a list of Linode network interfaces to use for the instance. Requires Linode Interfaces beta opt-in to use.<br />Conflicts with Interfaces. |  |  |
| `diskEncryption` _[InstanceDiskEncryption](#instancediskencryption)_ | diskEncryption determines if the disks of the instance should be encrypted. The default is disabled. |  | Enum: [enabled disabled] <br /> |


#### LinodeMachineStatus



LinodeMachineStatus defines the observed state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeMachine. |  |  |
| `ready` _boolean_ | ready is true when the provider resource is ready. | false |  |
| `addresses` _MachineAddress array_ | addresses contains the Linode instance associated addresses. |  |  |
| `cloudinitMetadataSupport` _boolean_ | cloudinitMetadataSupport determines whether to use cloud-init or not.<br />Deprecated: Stackscript no longer in use, so this field is not used. | true |  |
| `instanceState` _[InstanceStatus](#instancestatus)_ | instanceState is the state of the Linode instance for this machine. |  |  |
| `failureReason` _string_ | failureReason will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a succinct value suitable<br />for machine interpretation.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | failureMessage will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a more verbose string suitable<br />for logging and human consumption.<br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `tags` _string array_ | tags are the tags applied to the Linode Machine. |  |  |


#### LinodeMachineTemplate



LinodeMachineTemplate is the Schema for the linodemachinetemplates API



_Appears in:_
- [LinodeMachineTemplateList](#linodemachinetemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeMachineTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LinodeMachineTemplateSpec](#linodemachinetemplatespec)_ | spec is the desired state of the LinodeMachineTemplate. |  |  |
| `status` _[LinodeMachineTemplateStatus](#linodemachinetemplatestatus)_ | status is the observed state of the LinodeMachineTemplate. |  |  |


#### LinodeMachineTemplateList



LinodeMachineTemplateList contains a list of LinodeMachineTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1beta1` | | |
| `kind` _string_ | `LinodeMachineTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachineTemplate](#linodemachinetemplate) array_ | items is a list of LinodeMachineTemplate. |  |  |


#### LinodeMachineTemplateResource



LinodeMachineTemplateResource describes the data needed to create a LinodeMachine from a template.



_Appears in:_
- [LinodeMachineTemplateSpec](#linodemachinetemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ | spec is the specification of the desired behavior of the machine. |  |  |


#### LinodeMachineTemplateSpec



LinodeMachineTemplateSpec defines the desired state of LinodeMachineTemplate



_Appears in:_
- [LinodeMachineTemplate](#linodemachinetemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeMachineTemplateResource](#linodemachinetemplateresource)_ | template defines the specification for a LinodeMachine. |  |  |


#### LinodeMachineTemplateStatus



LinodeMachineTemplateStatus defines the observed state of LinodeMachineTemplate
It is used to store the status of the LinodeMachineTemplate, such as tags.



_Appears in:_
- [LinodeMachineTemplate](#linodemachinetemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | conditions define the current service state of the LinodeMachineTemplate |  |  |
| `tags` _string array_ | tags that are currently applied to the LinodeMachineTemplate. |  |  |
| `firewallID` _integer_ | firewallID that is currently applied to the LinodeMachineTemplate. |  |  |


#### LinodeNBPortConfig







_Appears in:_
- [NodeBalancerConfig](#nodebalancerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `port` _integer_ | port configured on the NodeBalancer. It must be valid port range (1-65535). |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `nodeBalancerConfigID` _integer_ | nodeBalancerConfigID is the config ID of port's NodeBalancer config. |  |  |


#### NetworkSpec



NetworkSpec encapsulates Linode networking resources.



_Appears in:_
- [LinodeClusterSpec](#linodeclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `loadBalancerType` _string_ | loadBalancerType is the type of load balancer to use, defaults to NodeBalancer if not otherwise set. | NodeBalancer | Enum: [NodeBalancer dns external] <br /> |
| `dnsConfig` _[DNSConfig](#dnsconfig)_ | DNSConfig contains configuration for DNS-based load balancing. Ignored if LoadBalancerType is not set to "dns". |  |  |
| `nodeBalancerConfig` _[NodeBalancerConfig](#nodebalancerconfig)_ | NodeBalancerConfig contains configuration for NodeBalancer-based load balancing. Ignored if LoadBalancerType is not set to "NodeBalancer". |  |  |
| `apiserverLoadBalancerPort` _integer_ | apiserverLoadBalancerPort used by the api server. It must be valid ports range (1-65535).<br />If omitted, default value is 6443. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `subnetName` _string_ | subnetName is the name/label of the VPC subnet to be used by the cluster |  |  |
| `useVlan` _boolean_ | useVlan provisions a cluster that uses VLANs instead of VPCs. IPAM is managed internally. |  |  |


#### NodeBalancerConfig







_Appears in:_
- [NetworkSpec](#networkspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeBalancerID` _integer_ | nodeBalancerID is the id of NodeBalancer. |  |  |
| `nodeBalancerFirewallID` _integer_ | nodeBalancerFirewallID is the id of NodeBalancer Firewall. |  |  |
| `apiserverNodeBalancerConfigID` _integer_ | apiserverNodeBalancerConfigID is the config ID of api server NodeBalancer config. |  |  |
| `nodeBalancerBackendIPv4Range` _string_ | nodeBalancerBackendIPv4Range is the subnet range we want to provide for creating nodebalancer in VPC.<br />example: 10.10.10.0/30 |  |  |
| `additionalPorts` _[LinodeNBPortConfig](#linodenbportconfig) array_ | additionalPorts contains list of ports to be configured with NodeBalancer. |  |  |
| `enableVPCBackends` _boolean_ | enableVPCBackends toggles VPC-scoped NodeBalancer and VPC backend IP usage.<br />If set to false (default), the NodeBalancer will not be created in a VPC and<br />backends will use Linode private IPs. If true, the NodeBalancer will be<br />created in the configured VPC (when VPCRef or VPCID is set) and backends<br />will use VPC IPs. | false |  |


#### ObjectStore



ObjectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for
bootstrapping (e.g. Cloud-init).



_Appears in:_
- [LinodeClusterSpec](#linodeclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presignedURLDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | presignedURLDuration defines the duration for which presigned URLs are valid.<br />This is used to generate presigned URLs for S3 Bucket objects, which are used by<br />control-plane and worker nodes to fetch bootstrap data. |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | credentialsRef is a reference to a Secret that contains the credentials to use for accessing the Cluster Object Store. |  |  |


#### PublicInterfaceCreateOptions



PublicInterfaceCreateOptions defines the IPv4 and IPv6 public interface create options



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _[PublicInterfaceIPv4CreateOptions](#publicinterfaceipv4createoptions)_ | ipv4 is the IPv4 configuration for the public interface. |  |  |
| `ipv6` _[PublicInterfaceIPv6CreateOptions](#publicinterfaceipv6createoptions)_ | ipv6 is the IPv6 configuration for the public interface. |  |  |


#### PublicInterfaceIPv4AddressCreateOptions



PublicInterfaceIPv4AddressCreateOptions defines the public IPv4 address and whether it is primary



_Appears in:_
- [PublicInterfaceIPv4CreateOptions](#publicinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | address is the IPv4 address for the public interface. |  | MinLength: 1 <br /> |
| `primary` _boolean_ | primary is a boolean indicating whether the address is primary. |  |  |


#### PublicInterfaceIPv4CreateOptions



PublicInterfaceIPv4CreateOptions defines the PublicInterfaceIPv4AddressCreateOptions for addresses



_Appears in:_
- [PublicInterfaceCreateOptions](#publicinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[PublicInterfaceIPv4AddressCreateOptions](#publicinterfaceipv4addresscreateoptions) array_ | addresses is the IPv4 addresses for the public interface. |  |  |


#### PublicInterfaceIPv6CreateOptions



PublicInterfaceIPv6CreateOptions defines the PublicInterfaceIPv6RangeCreateOptions



_Appears in:_
- [PublicInterfaceCreateOptions](#publicinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ranges` _[PublicInterfaceIPv6RangeCreateOptions](#publicinterfaceipv6rangecreateoptions) array_ | ranges is the IPv6 ranges for the public interface. |  |  |


#### PublicInterfaceIPv6RangeCreateOptions



PublicInterfaceIPv6RangeCreateOptions defines the IPv6 range for a public interface



_Appears in:_
- [PublicInterfaceIPv6CreateOptions](#publicinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the public interface. |  | MinLength: 1 <br /> |


#### VLANInterface



VLANInterface defines the VLAN interface configuration for an instance



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vlanLabel` _string_ | vlanLabel is the label of the VLAN. |  | MinLength: 1 <br /> |
| `ipamAddress` _string_ | ipamAddress is the IP address to assign to the interface. |  |  |


#### VPCIPv4



VPCIPv4 defines VPC IPV4 settings



_Appears in:_
- [InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vpc` _string_ | vpc is the ID of the VPC to use for the interface. |  |  |
| `nat1to1` _string_ | nat1to1 is the NAT 1:1 address for the interface. |  |  |


#### VPCInterfaceCreateOptions



VPCInterfaceCreateOptions defines the VPC interface configuration for an instance



_Appears in:_
- [LinodeInterfaceCreateOptions](#linodeinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `subnetId` _integer_ | subnetId is the ID of the subnet to use for the interface. |  |  |
| `ipv4` _[VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)_ | ipv4 is the IPv4 configuration for the interface. |  |  |
| `ipv6` _[VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)_ | ipv6 is the IPv6 configuration for the interface. |  |  |


#### VPCInterfaceIPv4AddressCreateOptions



VPCInterfaceIPv4AddressCreateOptions defines the IPv4 configuration for a VPC interface



_Appears in:_
- [VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | address is the IPv4 address for the interface. |  | MinLength: 1 <br /> |
| `primary` _boolean_ | primary is a boolean indicating whether the address is primary. |  |  |
| `nat1to1Address` _string_ | nat1to1Address is the NAT 1:1 address for the interface. |  |  |


#### VPCInterfaceIPv4CreateOptions



VPCInterfaceIPv4CreateOptions defines the IPv4 address and range configuration for a VPC interface



_Appears in:_
- [VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[VPCInterfaceIPv4AddressCreateOptions](#vpcinterfaceipv4addresscreateoptions) array_ | addresses is the IPv4 addresses for the interface. |  |  |
| `ranges` _[VPCInterfaceIPv4RangeCreateOptions](#vpcinterfaceipv4rangecreateoptions) array_ | ranges is the IPv4 ranges for the interface. |  |  |


#### VPCInterfaceIPv4RangeCreateOptions



VPCInterfaceIPv4RangeCreateOptions defines the IPv4 range for a VPC interface



_Appears in:_
- [VPCInterfaceIPv4CreateOptions](#vpcinterfaceipv4createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv4 range for the interface. |  | MinLength: 1 <br /> |


#### VPCInterfaceIPv6CreateOptions



VPCInterfaceIPv6CreateOptions defines the IPv6 configuration for a VPC interface



_Appears in:_
- [VPCInterfaceCreateOptions](#vpcinterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `slaac` _[VPCInterfaceIPv6SLAACCreateOptions](#vpcinterfaceipv6slaaccreateoptions) array_ | slaac is the IPv6 SLAAC configuration for the interface. |  |  |
| `ranges` _[VPCInterfaceIPv6RangeCreateOptions](#vpcinterfaceipv6rangecreateoptions) array_ | ranges is the IPv6 ranges for the interface. |  |  |
| `isPublic` _boolean_ | isPublic is a boolean indicating whether the interface is public. |  |  |


#### VPCInterfaceIPv6RangeCreateOptions



VPCInterfaceIPv6RangeCreateOptions defines the IPv6 range for a VPC interface



_Appears in:_
- [VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the interface. |  | MinLength: 1 <br /> |


#### VPCInterfaceIPv6SLAACCreateOptions



VPCInterfaceIPv6SLAACCreateOptions defines the Range for IPv6 SLAAC



_Appears in:_
- [VPCInterfaceIPv6CreateOptions](#vpcinterfaceipv6createoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `range` _string_ | range is the IPv6 range for the interface. |  | MinLength: 1 <br /> |


