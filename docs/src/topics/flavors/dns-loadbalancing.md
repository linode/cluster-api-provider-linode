# DNS based apiserver Load Balancing

This flavor configures DNS records that resolve to the public (ipv4 and/or IPv6) IPs of the control plane nodes where the apiserver pods are running. No NodeBalancer will be created.
This needs the following to be set in the `LinodeCluster` spec under `network`
```bash
kind: LinodeCluster
spec:
    network:
        loadBalancerType: dns
        dnsRootDomain: test.net
        dnsUniqueIdentifier: abc123
```
Along with this, the `test.net` domain needs to be registered and also be pre-configured as a domain on Linode CM. Using the `LINODE_DNS_TOKEN` env var, you can pass the API token of a different account if the Domain has been created in another acount under Linode CM

With these changes, the controlPlaneEndpoint is set to `<domain-name>-<uniqueid>.<root-domain>`. This will set as the server in the KUBECONFIG as well.
The controller will create A/AAAA and TXT records under the domain in Linode CM


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
