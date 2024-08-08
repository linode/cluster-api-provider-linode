# DNS based apiserver Load Balancing

This flavor configures DNS records that resolve to the public (ipv4 and/or IPv6) IPs of the control plane nodes where the apiserver pods are running. No NodeBalancer will be created.
The following need to be set in the `LinodeCluster` spec under `network`
```bash
kind: LinodeCluster
metadata:
    name: test-cluster
spec:
    network:
        loadBalancerType: dns
        dnsRootDomain: test.net
        dnsUniqueIdentifier: abc123
```
We support DNS management with both, [Linode Cloud Manager](https://cloud.linode.com/domains) as well as [Akamai Edge DNS](https://techdocs.akamai.com/edge-dns/reference/edge-dns-api).
We default to the linode provider but to use akamai, you'll need
```bash
kind: LinodeCluster
metadata:
    name: test-cluster
spec:
    network:
        loadBalancerType: dns
        dnsRootDomain: test.net
        dnsUniqueIdentifier: abc123
        dnsProvider: akamai
```
Along with this, the `test.net` domain needs to be registered and also be pre-configured as a domain on Linode or zone on Akamai.
With these changes, the controlPlaneEndpoint is set to `test-cluster-abc123.test.net`. This will be set as the server in the KUBECONFIG as well.
If users wish to override the subdomain format with something custom, they can pass in the override using the env var `DNS_SUBDOMAIN_OVERRIDE`.
```bash
kind: LinodeCluster
metadata:
    name: test-cluster
spec:
    network:
        loadBalancerType: dns
        dnsRootDomain: test.net
        dnsProvider: akamai
        dnsSubDomainOverride: my-special-overide
```
This will replace the subdomain creation from `test-cluster-abc123.test.net` to make the url `my-special-overide.test.net`.

The controller will create A/AAAA and TXT records under [the Domains tab in the Linode Cloud Manager.](https://cloud.linode.com/domains) or Akamai Edge DNS depending on the provider.

### Linode Domains:
Using the `LINODE_DNS_TOKEN` env var, you can pass the [API token of a different account](https://cloud.linode.com/profile/tokens) if the Domain has been created in another acount under Linode CM:

```bash
export LINODE_DNS_TOKEN=<your Linode PAT>
```

Optionally, provide an alternative Linode API URL and root CA certificate.

```bash
export LINODE_DNS_URL=custom.api.linode.com
export LINODE_DNS_CA=/path/to/cacert.pem
```

### Akamai Domains:
For the controller to authenticate with the Edge DNS API, you'll need to set the following env vars when creating the mgmt cluster.
```
AKAMAI_ACCESS_TOKEN=""
AKAMAI_CLIENT_SECRET=""
AKAMAI_CLIENT_TOKEN=""
AKAMAI_HOST=""
```
You can read about how you can create these [here](https://techdocs.akamai.com/developer/docs/create-a-client-with-custom-permissions).

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm                 | Cilium | Ubuntu 22.04 | No                    | Yes  | Yes  |

## Prerequisites
[Quickstart](../getting-started.md) completed

## Usage
1. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --control-plane-machine-count 3 --worker-machine-count 3 \
        --flavor <controlplane>-dns-loadbalancing > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```

## Check
You should in a few moments see the records created and running a nslookup against the server endpoint should return a multianswer dns record
