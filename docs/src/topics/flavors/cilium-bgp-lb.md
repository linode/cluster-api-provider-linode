# Cilium BGP Load-Balancing

This flavor creates special labeled worker nodes for ingress which leverage Cilium's
[BGP Control Plane](https://docs.cilium.io/en/stable/network/bgp-control-plane/bgp-control-plane/)
and [LB IPAM](https://docs.cilium.io/en/stable/network/lb-ipam/) support.

With this flavor, Services exposed via `type: LoadBalancer` automatically get
assigned an `ExternalIP` provisioned as a shared IP through the
[linode-CCM](https://github.com/linode/linode-cloud-controller-manager?tab=readme-ov-file#shared-ip-load-balancing),
which is deployed with the necessary settings to perform shared IP load-balancing.

```admonish warning
There are a couple important caveats to load balancing support based on current
Linode networking and API limitations:

1. **Ingress traffic will not be split between BGP peer nodes**

   [Equal-Cost Multi-Path (ECMP)](https://en.wikipedia.org/wiki/Equal-cost_multi-path_routing)
   is not supported on the BGP routers so ingress traffic will not be split between each
   BGP Node in the cluster. One Node will be actively receiving traffic and the other(s)
   will act as standby(s). 
2. **Customer support is required to use this feature at this time**

   Since this uses additional IPv4 addresses on the nodes participating in Cilium's
   BGPPeeringPolicy, you need to [contact our Support team](https://www.linode.com/support/)
   to be permitted to add extra IPs.

```

```admonish note
Dual-stack support is enabled for clusters using this flavor since IPv6 is used for router
and neighbor solicitation.

Without enabling dual-stack support, the IPv6 traffic is blocked if the Cilium host firewall
is enabled (which it is by default in CAPL), even if there are no configured `CiliumClusterWideNetworkPolicies`
or the policy is set to audit (default) instead of enforce (see [https://github.com/cilium/cilium/issues/27484](https://github.com/cilium/cilium/issues/27484)). More information about firewalling can be found on the [Firewalling](../firewalling.md) page.
```

## Specification

| Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes  | Yes  |


## Prerequisites

1. [Quickstart](../getting-started.md) completed

## Usage

1. (Optional) Set up environment variable
    ```sh
    # Optional
    export BGP_PEER_MACHINE_COUNT=2
    ```

2. Generate cluster yaml

    ```sh
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --flavor kubeadm-cilium-bgp-lb > test-cluster.yaml
    ```

3. Apply cluster yaml

    ```sh
    kubectl apply -f test-cluster.yaml
    ```

After the cluster exists, you can create a Service exposed with `type: LoadBalancer` and
it will automatically get assigned an ExternalIP. It's recommended to set up an ingress controller
(e.g. [https://docs.cilium.io/en/stable/network/servicemesh/ingress/](https://docs.cilium.io/en/stable/network/servicemesh/ingress/))
to avoid needing to expose multiple `LoadBalancer` Services within the cluster.
