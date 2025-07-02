# API Reference

## Packages
- [infrastructure.cluster.x-k8s.io/v1alpha2](#infrastructureclusterx-k8siov1alpha2)


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
| `spec` _[AddressSetSpec](#addresssetspec)_ |  |  |  |
| `status` _[AddressSetStatus](#addresssetstatus)_ |  |  |  |


#### AddressSetList



AddressSetList contains a list of AddressSet





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `AddressSetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AddressSet](#addressset) array_ |  |  |  |


#### AddressSetSpec



AddressSetSpec defines the desired state of AddressSet



_Appears in:_
- [AddressSet](#addressset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _string_ |  |  |  |
| `ipv6` _string_ |  |  |  |


#### AddressSetStatus



AddressSetStatus defines the observed state of AddressSet



_Appears in:_
- [AddressSet](#addressset)



#### BucketAccessRef







_Appears in:_
- [LinodeObjectStorageKeySpec](#linodeobjectstoragekeyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketName` _string_ |  |  |  |
| `permissions` _string_ |  |  |  |
| `region` _string_ |  |  |  |


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
| `spec` _[FirewallRuleSpec](#firewallrulespec)_ |  |  |  |
| `status` _[FirewallRuleStatus](#firewallrulestatus)_ |  |  |  |


#### FirewallRuleList



FirewallRuleList contains a list of FirewallRule





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `FirewallRuleList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[FirewallRule](#firewallrule) array_ |  |  |  |


#### FirewallRuleSpec



FirewallRuleSpec defines the desired state of FirewallRule



_Appears in:_
- [FirewallRule](#firewallrule)
- [LinodeFirewallSpec](#linodefirewallspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `action` _string_ | INSERT ADDITIONAL SPEC FIELDS - desired state of cluster<br />Important: Run "make" to regenerate code after modifying this file |  |  |
| `label` _string_ |  |  |  |
| `description` _string_ |  |  |  |
| `ports` _string_ |  |  |  |
| `protocol` _[NetworkProtocol](#networkprotocol)_ |  |  | Enum: [TCP UDP ICMP IPENCAP] <br /> |
| `addresses` _[NetworkAddresses](#networkaddresses)_ |  |  |  |
| `addressSetRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | AddressSetRefs is a list of references to AddressSets as an alternative to<br />using Addresses but can be used in conjunction with it |  |  |


#### FirewallRuleStatus



FirewallRuleStatus defines the observed state of FirewallRule



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
| `name` _string_ | The name of the generated Secret. If not set, the name is formatted as "\{name-of-obj-key\}-obj-key". |  |  |
| `namespace` _string_ | The namespace for the generated Secret. If not set, defaults to the namespace of the LinodeObjectStorageKey. |  |  |
| `type` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secrettype-v1-core)_ | The type of the generated Secret. | Opaque | Enum: [Opaque addons.cluster.x-k8s.io/resource-set] <br /> |
| `format` _object (keys:string, values:string)_ | How to format the data stored in the generated Secret.<br />It supports Go template syntax and interpolating the following values: .AccessKey, .SecretKey .BucketName .BucketEndpoint .S3Endpoint<br />If no format is supplied then a generic one is used containing the values specified. |  |  |


#### InstanceConfigInterfaceCreateOptions



InstanceConfigInterfaceCreateOptions defines network interface config



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipamAddress` _string_ |  |  |  |
| `label` _string_ |  |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `purpose` _[ConfigInterfacePurpose](#configinterfacepurpose)_ |  |  |  |
| `primary` _boolean_ |  |  |  |
| `subnetId` _integer_ |  |  |  |
| `ipv4` _[VPCIPv4](#vpcipv4)_ |  |  |  |
| `ipRanges` _string array_ |  |  |  |


#### InstanceConfiguration



InstanceConfiguration defines the instance configuration



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kernel` _string_ | Kernel is a Kernel ID to boot a Linode with. (e.g linode/latest-64bit) |  |  |


#### InstanceDisk



InstanceDisk defines a list of disks to use for an instance



_Appears in:_
- [LinodeMachineSpec](#linodemachinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `diskID` _integer_ | DiskID is the linode assigned ID of the disk |  |  |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#quantity-resource-api)_ | Size of the disk in resource.Quantity notation |  | Required: \{\} <br /> |
| `label` _string_ | Label for the instance disk, if nothing is provided it will match the device name |  |  |
| `filesystem` _string_ | Filesystem of disk to provision, the default disk filesystem is "ext4" |  | Enum: [raw swap ext3 ext4 initrd] <br /> |




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
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ |  |  |  |
| `status` _[LinodeClusterStatus](#linodeclusterstatus)_ |  |  |  |


#### LinodeClusterList



LinodeClusterList contains a list of LinodeCluster





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeClusterList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeCluster](#linodecluster) array_ |  |  |  |


#### LinodeClusterSpec



LinodeClusterSpec defines the desired state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)
- [LinodeClusterTemplateResource](#linodeclustertemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | The Linode Region the LinodeCluster lives in. |  |  |
| `controlPlaneEndpoint` _[APIEndpoint](#apiendpoint)_ | ControlPlaneEndpoint represents the endpoint used to communicate with the LinodeCluster control plane.<br />If ControlPlaneEndpoint is unset then the Nodebalancer ip will be used. |  |  |
| `network` _[NetworkSpec](#networkspec)_ | NetworkSpec encapsulates all things related to Linode network. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ |  |  |  |
| `vpcID` _integer_ | VPCID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL. |  |  |
| `nodeBalancerFirewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | NodeBalancerFirewallRef is a reference to a NodeBalancer Firewall object. This makes the linode use the specified NodeBalancer Firewall. |  |  |
| `objectStore` _[ObjectStore](#objectstore)_ | ObjectStore defines a supporting Object Storage bucket for cluster operations. This is currently used for<br />bootstrapping (e.g. Cloud-init). |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not<br />supplied then the credentials of the controller will be used. |  |  |


#### LinodeClusterStatus



LinodeClusterStatus defines the observed state of LinodeCluster



_Appears in:_
- [LinodeCluster](#linodecluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready denotes that the cluster (infrastructure) is ready. |  |  |
| `failureReason` _string_ | FailureReason will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a succinct value suitable<br />for machine interpretation. |  |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the LinodeCluster and will contain a more verbose string suitable<br />for logging and human consumption. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions defines current service state of the LinodeCluster. |  |  |


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
| `spec` _[LinodeClusterTemplateSpec](#linodeclustertemplatespec)_ |  |  |  |


#### LinodeClusterTemplateList



LinodeClusterTemplateList contains a list of LinodeClusterTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeClusterTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeClusterTemplate](#linodeclustertemplate) array_ |  |  |  |


#### LinodeClusterTemplateResource



LinodeClusterTemplateResource describes the data needed to create a LinodeCluster from a template.



_Appears in:_
- [LinodeClusterTemplateSpec](#linodeclustertemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeClusterSpec](#linodeclusterspec)_ |  |  |  |


#### LinodeClusterTemplateSpec



LinodeClusterTemplateSpec defines the desired state of LinodeClusterTemplate



_Appears in:_
- [LinodeClusterTemplate](#linodeclustertemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeClusterTemplateResource](#linodeclustertemplateresource)_ |  |  |  |


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
| `spec` _[LinodeFirewallSpec](#linodefirewallspec)_ |  |  |  |
| `status` _[LinodeFirewallStatus](#linodefirewallstatus)_ |  |  |  |


#### LinodeFirewallList



LinodeFirewallList contains a list of LinodeFirewall





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeFirewallList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeFirewall](#linodefirewall) array_ |  |  |  |


#### LinodeFirewallSpec



LinodeFirewallSpec defines the desired state of LinodeFirewall



_Appears in:_
- [LinodeFirewall](#linodefirewall)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `firewallID` _integer_ |  |  |  |
| `enabled` _boolean_ |  | false |  |
| `inboundRules` _[FirewallRuleSpec](#firewallrulespec) array_ |  |  |  |
| `inboundRuleRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | InboundRuleRefs is a list of references to FirewallRules as an alternative to<br />using InboundRules but can be used in conjunction with it |  |  |
| `inboundPolicy` _string_ | InboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined. | ACCEPT | Enum: [ACCEPT DROP] <br /> |
| `outboundRules` _[FirewallRuleSpec](#firewallrulespec) array_ |  |  |  |
| `outboundRuleRefs` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core) array_ | OutboundRuleRefs is a list of references to FirewallRules as an alternative to<br />using OutboundRules but can be used in conjunction with it |  |  |
| `outboundPolicy` _string_ | OutboundPolicy determines if traffic by default should be ACCEPTed or DROPped. Defaults to ACCEPT if not defined. | ACCEPT | Enum: [ACCEPT DROP] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this Firewall. If not<br />supplied then the credentials of the controller will be used. |  |  |


#### LinodeFirewallStatus



LinodeFirewallStatus defines the observed state of LinodeFirewall



_Appears in:_
- [LinodeFirewall](#linodefirewall)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready is true when the provider resource is ready. | false |  |
| `failureReason` _[FirewallStatusError](#firewallstatuserror)_ | FailureReason will be set in the event that there is a terminal problem<br />reconciling the Firewall and will contain a succinct value suitable<br />for machine interpretation.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Firewall's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of Firewalls<br />can be added as events to the Firewall object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the Firewall and will contain a more verbose string suitable<br />for logging and human consumption.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Firewall's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of Firewalls<br />can be added as events to the Firewall object and/or logged in the<br />controller's output. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions defines current service state of the LinodeFirewall. |  |  |


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
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ |  |  |  |
| `status` _[LinodeMachineStatus](#linodemachinestatus)_ |  |  |  |


#### LinodeMachineList



LinodeMachineList contains a list of LinodeMachine





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachineList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachine](#linodemachine) array_ |  |  |  |


#### LinodeMachineSpec



LinodeMachineSpec defines the desired state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)
- [LinodeMachineTemplateResource](#linodemachinetemplateresource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `providerID` _string_ | ProviderID is the unique identifier as specified by the cloud provider. |  |  |
| `instanceID` _integer_ | InstanceID is the Linode instance ID for this machine. |  |  |
| `region` _string_ |  |  | Required: \{\} <br /> |
| `type` _string_ |  |  | Required: \{\} <br /> |
| `group` _string_ |  |  |  |
| `rootPass` _string_ |  |  |  |
| `authorizedKeys` _string array_ |  |  |  |
| `authorizedUsers` _string array_ |  |  |  |
| `backupID` _integer_ |  |  |  |
| `image` _string_ |  |  |  |
| `interfaces` _[InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions) array_ |  |  |  |
| `backupsEnabled` _boolean_ |  |  |  |
| `privateIP` _boolean_ |  |  |  |
| `tags` _string array_ | Deprecated: spec.tags is deprecated, use metadata.annotations.linode-vm-tags instead. |  |  |
| `firewallID` _integer_ |  |  |  |
| `osDisk` _[InstanceDisk](#instancedisk)_ | OSDisk is configuration for the root disk that includes the OS,<br />if not specified this defaults to whatever space is not taken up by the DataDisks |  |  |
| `dataDisks` _object (keys:string, values:[InstanceDisk](#instancedisk))_ | DataDisks is a map of any additional disks to add to an instance,<br />The sum of these disks + the OSDisk must not be more than allowed on a linodes plan |  |  |
| `diskEncryption` _string_ | DiskEncryption determines if the disks of the instance should be encrypted. The default is disabled. |  | Enum: [enabled disabled] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials<br />to use for provisioning this machine. If not supplied then these<br />credentials will be used in-order:<br />  1. LinodeMachine<br />  2. Owner LinodeCluster<br />  3. Controller |  |  |
| `configuration` _[InstanceConfiguration](#instanceconfiguration)_ | Configuration is the Akamai instance configuration OS,<br />if not specified this defaults to the default configuration associated to the instance. |  |  |
| `placementGroupRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | PlacementGroupRef is a reference to a placement group object. This makes the linode to be launched in that specific group. |  |  |
| `firewallRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | FirewallRef is a reference to a firewall object. This makes the linode use the specified firewall. |  |  |
| `vpcRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | VPCRef is a reference to a LinodeVPC resource. If specified, this takes precedence over<br />the cluster-level VPC configuration for multi-region support. |  |  |
| `vpcID` _integer_ | VPCID is the ID of an existing VPC in Linode. This allows using a VPC that is not managed by CAPL. |  |  |


#### LinodeMachineStatus



LinodeMachineStatus defines the observed state of LinodeMachine



_Appears in:_
- [LinodeMachine](#linodemachine)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready is true when the provider resource is ready. | false |  |
| `addresses` _MachineAddress array_ | Addresses contains the Linode instance associated addresses. |  |  |
| `cloudinitMetadataSupport` _boolean_ | CloudinitMetadataSupport determines whether to use cloud-init or not.<br />Deprecated: Stackscript no longer in use, so this field is not used. | true |  |
| `instanceState` _[InstanceStatus](#instancestatus)_ | InstanceState is the state of the Linode instance for this machine. |  |  |
| `failureReason` _string_ | FailureReason will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a succinct value suitable<br />for machine interpretation.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the Machine and will contain a more verbose string suitable<br />for logging and human consumption.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the Machine's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of Machines<br />can be added as events to the Machine object and/or logged in the<br />controller's output. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions defines current service state of the LinodeMachine. |  |  |


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
| `spec` _[LinodeMachineTemplateSpec](#linodemachinetemplatespec)_ |  |  |  |


#### LinodeMachineTemplateList



LinodeMachineTemplateList contains a list of LinodeMachineTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeMachineTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeMachineTemplate](#linodemachinetemplate) array_ |  |  |  |


#### LinodeMachineTemplateResource



LinodeMachineTemplateResource describes the data needed to create a LinodeMachine from a template.



_Appears in:_
- [LinodeMachineTemplateSpec](#linodemachinetemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spec` _[LinodeMachineSpec](#linodemachinespec)_ |  |  |  |


#### LinodeMachineTemplateSpec



LinodeMachineTemplateSpec defines the desired state of LinodeMachineTemplate



_Appears in:_
- [LinodeMachineTemplate](#linodemachinetemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[LinodeMachineTemplateResource](#linodemachinetemplateresource)_ |  |  |  |


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
| `spec` _[LinodeObjectStorageBucketSpec](#linodeobjectstoragebucketspec)_ |  |  |  |
| `status` _[LinodeObjectStorageBucketStatus](#linodeobjectstoragebucketstatus)_ |  |  |  |


#### LinodeObjectStorageBucketList



LinodeObjectStorageBucketList contains a list of LinodeObjectStorageBucket





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageBucketList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeObjectStorageBucket](#linodeobjectstoragebucket) array_ |  |  |  |


#### LinodeObjectStorageBucketSpec



LinodeObjectStorageBucketSpec defines the desired state of LinodeObjectStorageBucket



_Appears in:_
- [LinodeObjectStorageBucket](#linodeobjectstoragebucket)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | Region is the ID of the Object Storage region for the bucket. |  |  |
| `acl` _[ObjectStorageACL](#objectstorageacl)_ | Acl sets the Access Control Level of the bucket using a canned ACL string | private | Enum: [private public-read authenticated-read public-read-write] <br /> |
| `corsEnabled` _boolean_ | corsEnabled enables for all origins in the bucket .If set to false, CORS is disabled for all origins in the bucket | true |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning the bucket.<br />If not supplied then the credentials of the controller will be used. |  |  |
| `accessKeyRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core)_ | AccessKeyRef is a reference to a LinodeObjectStorageBucketKey for the bucket. |  |  |
| `forceDeleteBucket` _boolean_ | ForceDeleteBucket enables the object storage bucket used to be deleted even if it contains objects. |  |  |


#### LinodeObjectStorageBucketStatus



LinodeObjectStorageBucketStatus defines the observed state of LinodeObjectStorageBucket



_Appears in:_
- [LinodeObjectStorageBucket](#linodeobjectstoragebucket)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready denotes that the bucket has been provisioned along with access keys. | false |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the Object Storage Bucket and will contain a verbose string<br />suitable for logging and human consumption. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions specify the service state of the LinodeObjectStorageBucket. |  |  |
| `hostname` _string_ | Hostname is the address assigned to the bucket. |  |  |
| `creationTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | CreationTime specifies the creation timestamp for the bucket. |  |  |


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
| `spec` _[LinodeObjectStorageKeySpec](#linodeobjectstoragekeyspec)_ |  |  |  |
| `status` _[LinodeObjectStorageKeyStatus](#linodeobjectstoragekeystatus)_ |  |  |  |


#### LinodeObjectStorageKeyList



LinodeObjectStorageKeyList contains a list of LinodeObjectStorageKey





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeObjectStorageKeyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeObjectStorageKey](#linodeobjectstoragekey) array_ |  |  |  |


#### LinodeObjectStorageKeySpec



LinodeObjectStorageKeySpec defines the desired state of LinodeObjectStorageKey



_Appears in:_
- [LinodeObjectStorageKey](#linodeobjectstoragekey)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketAccess` _[BucketAccessRef](#bucketaccessref) array_ | BucketAccess is the list of object storage bucket labels which can be accessed using the key |  | MinItems: 1 <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for generating access keys.<br />If not supplied then the credentials of the controller will be used. |  |  |
| `keyGeneration` _integer_ | KeyGeneration may be modified to trigger a rotation of the access key. | 0 |  |
| `generatedSecret` _[GeneratedSecret](#generatedsecret)_ | GeneratedSecret configures the Secret to generate containing access key details. |  |  |
| `secretType` _[SecretType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secrettype-v1-core)_ | SecretType instructs the controller what type of secret to generate containing access key details.<br />Deprecated: Use generatedSecret.type. |  | Enum: [Opaque addons.cluster.x-k8s.io/resource-set] <br /> |
| `secretDataFormat` _object (keys:string, values:string)_ | SecretDataFormat instructs the controller how to format the data stored in the secret containing access key details.<br />Deprecated: Use generatedSecret.format. |  |  |


#### LinodeObjectStorageKeyStatus



LinodeObjectStorageKeyStatus defines the observed state of LinodeObjectStorageKey



_Appears in:_
- [LinodeObjectStorageKey](#linodeobjectstoragekey)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready denotes that the key has been provisioned. | false |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the Object Storage Key and will contain a verbose string<br />suitable for logging and human consumption. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions specify the service state of the LinodeObjectStorageKey. |  |  |
| `creationTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | CreationTime specifies the creation timestamp for the secret. |  |  |
| `lastKeyGeneration` _integer_ | LastKeyGeneration tracks the last known value of .spec.keyGeneration. |  |  |
| `accessKeyRef` _integer_ | AccessKeyRef stores the ID for Object Storage key provisioned. |  |  |


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
| `spec` _[LinodePlacementGroupSpec](#linodeplacementgroupspec)_ |  |  |  |
| `status` _[LinodePlacementGroupStatus](#linodeplacementgroupstatus)_ |  |  |  |


#### LinodePlacementGroupList



LinodePlacementGroupList contains a list of LinodePlacementGroup





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodePlacementGroupList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodePlacementGroup](#linodeplacementgroup) array_ |  |  |  |


#### LinodePlacementGroupSpec



LinodePlacementGroupSpec defines the desired state of LinodePlacementGroup



_Appears in:_
- [LinodePlacementGroup](#linodeplacementgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pgID` _integer_ |  |  |  |
| `region` _string_ |  |  |  |
| `placementGroupPolicy` _string_ |  | strict | Enum: [strict flexible] <br /> |
| `placementGroupType` _string_ |  | anti_affinity:local | Enum: [anti_affinity:local] <br /> |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this PlacementGroup. If not<br />supplied then the credentials of the controller will be used. |  |  |


#### LinodePlacementGroupStatus



LinodePlacementGroupStatus defines the observed state of LinodePlacementGroup



_Appears in:_
- [LinodePlacementGroup](#linodeplacementgroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready is true when the provider resource is ready. | false |  |
| `failureReason` _[LinodePlacementGroupStatusError](#linodeplacementgroupstatuserror)_ | FailureReason will be set in the event that there is a terminal problem<br />reconciling the PlacementGroup and will contain a succinct value suitable<br />for machine interpretation.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the PlacementGroup's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of PlacementGroups<br />can be added as events to the PlacementGroup object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the PlacementGroup and will contain a more verbose string suitable<br />for logging and human consumption.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the PlacementGroup's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of PlacementGroups<br />can be added as events to the PlacementGroup object and/or logged in the<br />controller's output. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions defines current service state of the LinodePlacementGroup. |  |  |


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
| `spec` _[LinodeVPCSpec](#linodevpcspec)_ |  |  |  |
| `status` _[LinodeVPCStatus](#linodevpcstatus)_ |  |  |  |


#### LinodeVPCList



LinodeVPCList contains a list of LinodeVPC





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `infrastructure.cluster.x-k8s.io/v1alpha2` | | |
| `kind` _string_ | `LinodeVPCList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LinodeVPC](#linodevpc) array_ |  |  |  |


#### LinodeVPCSpec



LinodeVPCSpec defines the desired state of LinodeVPC



_Appears in:_
- [LinodeVPC](#linodevpc)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vpcID` _integer_ |  |  |  |
| `description` _string_ |  |  |  |
| `region` _string_ |  |  |  |
| `subnets` _[VPCSubnetCreateOptions](#vpcsubnetcreateoptions) array_ |  |  |  |
| `retain` _boolean_ | Retain allows you to keep the VPC after the LinodeVPC object is deleted.<br />This is useful if you want to use an existing VPC that was not created by this controller.<br />If set to true, the controller will not delete the VPC resource in Linode.<br />Defaults to false. |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this VPC. If not<br />supplied then the credentials of the controller will be used. |  |  |


#### LinodeVPCStatus



LinodeVPCStatus defines the observed state of LinodeVPC



_Appears in:_
- [LinodeVPC](#linodevpc)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready is true when the provider resource is ready. | false |  |
| `failureReason` _[VPCStatusError](#vpcstatuserror)_ | FailureReason will be set in the event that there is a terminal problem<br />reconciling the VPC and will contain a succinct value suitable<br />for machine interpretation.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the VPC's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of VPCs<br />can be added as events to the VPC object and/or logged in the<br />controller's output. |  |  |
| `failureMessage` _string_ | FailureMessage will be set in the event that there is a terminal problem<br />reconciling the VPC and will contain a more verbose string suitable<br />for logging and human consumption.<br /><br />This field should not be set for transitive errors that a controller<br />faces that are expected to be fixed automatically over<br />time (like service outages), but instead indicate that something is<br />fundamentally wrong with the VPC's spec or the configuration of<br />the controller, and that manual intervention is required. Examples<br />of terminal errors would be invalid combinations of settings in the<br />spec, values that are unsupported by the controller, or the<br />responsible controller itself being critically misconfigured.<br /><br />Any transient errors that occur during the reconciliation of VPCs<br />can be added as events to the VPC object and/or logged in the<br />controller's output. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions defines current service state of the LinodeVPC. |  |  |


#### NetworkAddresses



NetworkAddresses holds a list of IPv4 and IPv6 addresses
We don't use linodego here since kubebuilder can't generate DeepCopyInto
for linodego.NetworkAddresses



_Appears in:_
- [FirewallRuleSpec](#firewallrulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4` _string_ |  |  |  |
| `ipv6` _string_ |  |  |  |


#### NetworkSpec



NetworkSpec encapsulates Linode networking resources.



_Appears in:_
- [LinodeClusterSpec](#linodeclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `loadBalancerType` _string_ | LoadBalancerType is the type of load balancer to use, defaults to NodeBalancer if not otherwise set | NodeBalancer | Enum: [NodeBalancer dns external] <br /> |
| `dnsProvider` _string_ | DNSProvider is provider who manages the domain<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, defaults linode dns |  | Enum: [linode akamai] <br /> |
| `dnsRootDomain` _string_ | DNSRootDomain is the root domain used to create a DNS entry for the control-plane endpoint<br />Ignored if the LoadBalancerType is set to anything other than dns |  |  |
| `dnsUniqueIdentifier` _string_ | DNSUniqueIdentifier is the unique identifier for the DNS. This let clusters with the same name have unique<br />DNS record<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, CAPL will create a unique identifier for you |  |  |
| `dnsTTLsec` _integer_ | DNSTTLSec is the TTL for the domain record<br />Ignored if the LoadBalancerType is set to anything other than dns<br />If not set, defaults to 30 |  |  |
| `dnsSubDomainOverride` _string_ | DNSSubDomainOverride is used to override CAPL's construction of the controlplane endpoint<br />If set, this will override the DNS subdomain from <clustername>-<uniqueid>.<rootdomain> to <overridevalue>.<rootdomain> |  |  |
| `apiserverLoadBalancerPort` _integer_ | apiserverLoadBalancerPort used by the api server. It must be valid ports range (1-65535).<br />If omitted, default value is 6443. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `nodeBalancerID` _integer_ | NodeBalancerID is the id of NodeBalancer. |  |  |
| `nodeBalancerFirewallID` _integer_ | NodeBalancerFirewallID is the id of NodeBalancer Firewall. |  |  |
| `apiserverNodeBalancerConfigID` _integer_ | apiserverNodeBalancerConfigID is the config ID of api server NodeBalancer config. |  |  |
| `additionalPorts` _[LinodeNBPortConfig](#linodenbportconfig) array_ | additionalPorts contains list of ports to be configured with NodeBalancer. |  |  |
| `subnetName` _string_ | subnetName is the name/label of the VPC subnet to be used by the cluster |  |  |
| `useVlan` _boolean_ | UseVlan provisions a cluster that uses VLANs instead of VPCs. IPAM is managed internally. |  |  |
| `nodeBalancerBackendIPv4Range` _string_ | NodeBalancerBackendIPv4Range is the subnet range we want to provide for creating nodebalancer in VPC.<br />example: 10.10.10.0/30 |  |  |


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
| `presignedURLDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta)_ | PresignedURLDuration defines the duration for which presigned URLs are valid.<br /><br />This is used to generate presigned URLs for S3 Bucket objects, which are used by<br />control-plane and worker nodes to fetch bootstrap data. |  |  |
| `credentialsRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core)_ | CredentialsRef is a reference to a Secret that contains the credentials to use for accessing the Cluster Object Store. |  |  |


#### VPCIPv4



VPCIPv4 defines VPC IPV4 settings



_Appears in:_
- [InstanceConfigInterfaceCreateOptions](#instanceconfiginterfacecreateoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `vpc` _string_ |  |  |  |
| `nat1to1` _string_ |  |  |  |


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
| `label` _string_ |  |  | MaxLength: 63 <br />MinLength: 3 <br /> |
| `ipv4` _string_ |  |  |  |
| `subnetID` _integer_ | SubnetID is subnet id for the subnet |  |  |
| `retain` _boolean_ | Retain allows you to keep the Subnet after the LinodeVPC object is deleted.<br />This is only applicable when the parent VPC has RetainVPC set to true and the<br />--enable-subnet-deletion flag is enabled on the controller. |  |  |


