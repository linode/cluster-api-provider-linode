# Cilium BGP Load-Balancing

This flavor creates special labeled worker nodes for ingress which leverage Cilium's
[BGP Control Plane](https://docs.cilium.io/en/stable/network/bgp-control-plane/)
and [LB IPAM](https://docs.cilium.io/en/stable/network/lb-ipam/) support.

With this flavor, Services exposed via `type: LoadBalancer` automatically get
assigned an `ExternalIP` provisioned as a shared IP through the
[linode-CCM](https://github.com/linode/linode-cloud-controller-manager/blob/shared-ip/README.md#shared-ip-load-balancing),
which is deployed with the necessary settings to perform shared IP load-balancing.

```admonish warning
There are several important caveats to load balancing support based on current
Linode networking and API limitations:

1. **Services with external IPv6 addresses are not reachable**

   While it is possible on a dual-stack cluster to use Cilium's LB IPAM and
   BGP Control Plane features to automatically assign a IPv6 address to a
   Service, this address will not be accessible outside the cluster since
   Cilium will try to advertise a /128 address that gets filtered on the routing
   tables for the BGP routers.
2. **Ingress traffic will not be split between BGP peer nodes**

   [Equal-Cost Multi-Path (ECMP)](https://en.wikipedia.org/wiki/Equal-cost_multi-path_routing)
   is not supported on the BGP routers so ingress traffic will not be split between each
   BGP Node in the cluster. One Node will be actively receiving traffic and the other(s)
   will act as standby(s). 
3. **Customer support is required to use this feature at this time**

   Since this uses additional IPv4 addresses on the nodes participating in Cilium's
   BGPPeeringPolicy, you need to [contact our Support team](https://www.linode.com/support/)
   to be permitted to add extra IPs.

```

## Specification

| Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|---------------|--------|--------------|-----------------------|------|------|
| Kubeadm       | Cilium | Ubuntu 22.04 | No                    | Yes  | No   |


## Prerequisites

1. [Quickstart](../getting-started.md) completed

## Usage

1. (Optional) Set up environment variables
    ```sh
    # Optional
    export LINODE_BGP_PEER_MACHINE_TYPE=g6-standard-2
    export BGP_PEER_MACHINE_COUNT=2
    ```

2. Generate cluster yaml

    ```sh
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --flavor cilium-bgp-lb > test-cluster.yaml
    ```

3. Apply cluster yaml

    ```sh
    kubectl apply -f test-cluster.yaml
    ```

After the cluster exists, you can create a Service exposed with `type: LoadBalancer` and
it will automatically get assigned an ExternalIP. It's recommended to set up an ingress controller
(e.g. [https://docs.cilium.io/en/stable/network/servicemesh/ingress/](https://docs.cilium.io/en/stable/network/servicemesh/ingress/))
to avoid needing to expose multiple `LoadBalancer` Services within the cluster.
