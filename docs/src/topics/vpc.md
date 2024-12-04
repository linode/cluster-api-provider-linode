# VPC

This guide covers how [VPC](https://www.linode.com/docs/products/networking/vpc/) is used with CAPL clusters. By default, CAPL clusters are provisioned within VPC.

## Default configuration
Each linode within a cluster gets provisioned with two interfaces:
1. eth0 (connected to VPC, for pod-to-pod traffic and public traffic)
2. eth1 (for nodebalancer traffic)

Key facts about VPC network configuration:
1. VPCs are provisioned with a private subnet 10.0.0.0/8.
2. All pod-to-pod communication happens over the VPC interface (eth0).
3. We assign a pod CIDR of range 10.192.0.0/10 for pod-to-pod communication.
3. By default, cilium is configured with [native routing](https://docs.cilium.io/en/stable/network/concepts/routing/#native-routing)
4. [Kubernetes host-scope IPAM mode](https://docs.cilium.io/en/stable/network/concepts/ipam/kubernetes/) is used to assign pod CIDRs to nodes. We run [linode CCM](https://github.com/linode/linode-cloud-controller-manager) with [route-controller enabled](https://github.com/linode/linode-cloud-controller-manager?tab=readme-ov-file#routes) which automatically adds/updates routes within VPC when pod cidrs are added/updated by k8s. This enables pod-to-pod traffic to be routable within the VPC.
5. kube-proxy is disabled by default.


## Configuring the VPC interface
In order to configure the VPC interface beyond the default above, an explicit interface can be configured in the `LinodeMachineTemplate`.
When the `LinodeMachine` controller find an interface with `purpose: vpc` it will automatically inject the `SubnetID` from the
`VPCRef`. 

_Example template where the VPC interface is not the primary interface_
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeMachineTemplate
metadata:
  name: test-cluster-md-0
  namespace: default
spec:
  template:
    spec:
      region: "us-mia"
      type: "g6-standard-4"
      image: linode/ubuntu22.04
      interfaces:
      - purpose: vpc
        primary: false
      - purpose: public
        primary: true
```
## How VPC is provisioned
A VPC is tied to a region. CAPL generates LinodeVPC manifest which contains the VPC name, region and subnet information. By defult, VPC name is set to cluster name but can be overwritten by specifying relevant environment variable.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeVPC
metadata:
  name: ${VPC_NAME:=${CLUSTER_NAME}}
  labels:
    cluster.x-k8s.io/cluster-name: ${CLUSTER_NAME}
spec:
  region: ${LINODE_REGION}
  subnets:
    - ipv4: 10.0.0.0/8
      label: default
```

Reference to LinodeVPC object is added to LinodeCluster object which then uses the specified VPC to provision resources.

## Troubleshooting
### If pod-to-pod connectivity is failing
If a pod can't ping pod ips on different node, check and make sure pod CIDRs are added to ip_ranges of VPC interface.

```sh
curl --header 'Authorization: Bearer $LINODE_API_TOKEN' -X GET https://api.linode.com/v4/linode/instances/${LINODEID}/configs | jq .data[0].interfaces[].ip_ranges
```

```admonish note
CIDR returned in the output of above command should match with the pod CIDR present in node's spec `k get node <nodename> -o yaml | yq .spec.podCIDRs`
```

### Running cilium connectivity tests
One can also run cilium connectivity tests to make sure networking works fine within VPC. Follow the steps defined in [cilium e2e tests](https://docs.cilium.io/en/stable/contributing/testing/e2e/) guide to install cilium binary, set the KUBECONFIG variable and then run `cilium connectivity tests`.
