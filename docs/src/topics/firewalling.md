# Firewalling

This guide covers how Cilium and Cloud Firewalls can be used for firewalling CAPL clusters.

## Cilium Firewalls

Cilium provides cluster-wide firewalling via [Host Policies](https://docs.cilium.io/en/latest/security/policy/language/#hostpolicies)
which enforce access control over connectivity to and from cluster nodes.
Cilium's [host firewall](https://docs.cilium.io/en/latest/security/host-firewall/) is responsible for enforcing the security policies.

### Default Cilium Host Firewall Configuration
By default, the following Host Policies are set to audit mode (without any enforcement) on CAPL clusters:

* [Kubeadm](./flavors/default.md) cluster allow rules

    | Ports                   | Use-case                 | Allowed clients       |
    |-------------------------|--------------------------|-----------------------|
    | ${APISERVER_PORT:=6443} | API Server Traffic       | World                 |
    | *                       | In Cluster Communication | Intra Cluster Traffic |

```admonish note
For kubeadm clusters running outside of VPC, ports 2379 and 2380 are also allowed for etcd-traffic.
```

* [k3s](./flavors/k3s.md) cluster allow rules
    
    | Ports | Use-case                 | Allowed clients               |
    |-------|--------------------------|-------------------------------|
    | 6443  | API Server Traffic       | World                         |
    | *     | In Cluster Communication | Intra Cluster and VPC Traffic |

* [RKE2](./flavors/rke2.md) cluster allow rules

  | Ports | Use-case                 | Allowed clients               |
  |-------|--------------------------|-------------------------------|
  | 6443  | API Server Traffic       | World                         |
  | *     | In Cluster Communication | Intra Cluster and VPC Traffic |

### Enabling Cilium Host Policy Enforcement
In order to turn the Cilium Host Policies from audit to enforce mode, use the environment variable `FW_AUDIT_ONLY=false`
when generating the cluster. This will set the [policy-audit-mode](https://docs.cilium.io/en/latest/security/policy-creation/#creating-policies-from-verdicts)
on the Cilium deployment.

###  Adding Additional Cilium Host Policies
Additional rules can be added to the `default-policy`:
```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "default-external-policy"
spec:
  description: "allow cluster intra cluster traffic along api server traffic"
  nodeSelector: {}
  ingress:
    - fromEntities:
        - cluster
    - fromCIDR:
        - 10.0.0.0/8
    - fromEntities:
        - world
      toPorts:
        - ports:
            - port: "22" # added for SSH Access to the nodes
            - port: "${APISERVER_PORT:=6443}"
```
Alternatively, additional rules can be added by creating a new policy:
```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "ssh-access-policy"
spec:
  description: "allows ssh access to nodes"
  nodeSelector: {}
  ingress:
    - fromEntities:
        - world
      toPorts:
        - ports:
            - port: "22"
```

## Cloud Firewalls
Cloud firewalls are provisioned with all flavors that use VPCs. They are provisioned in disabled mode but can be enabled
with the environment variable `LINODE_FIREWALL_ENABLED=true`. The default rules allow for all intra-cluster VPC traffic 
along with any traffic going to the API server. 

### Creating Cloud Firewalls
For controlling firewalls via Linode resources, a [Cloud Firewall](https://www.linode.com/products/cloud-firewall/) can
be defined and provisioned via the `LinodeFirewall` resource in CAPL. Any updates to the cloud firewall CAPL resource
will be updated in the cloud firewall and overwrite any changes made outside the CAPL resource.

Example `LinodeFirewall` and `AddressSet`:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeFirewall
metadata:
  name: sample-fw
spec:
  enabled: true
  inboundPolicy: DROP
  inboundRules:
    - action: ACCEPT
      label: intra-cluster
      ports: "1-65535"
      protocol: "TCP"
      addresses:
        ipv4:
          - "10.0.0.0/8"
    - action: ACCEPT
      label: inbound-api-server
      ports: "6443"
      protocol: TCP
      addressSetRefs:  # Can be used together with .addresses if desired.
        - name: my-hosts
          kind: AddressSet
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: AddressSet
metadata:
  name: my-hosts
spec:
  ipv4:
    - "0.0.0.0/0"
  ipv6:
    - ::/0
```

### Cloud Firewall Machine Integration
The created Cloud Firewall can be used on a `LinodeMachine` or a `LinodeMachineTemplate` by setting the `firewallRef` field.
Alternatively, the provisioned Cloud Firewall's ID can be used in the `firewallID` field.

```admonish note
The `firewallRef` and `firewallID` fields are currently immutable for `LinodeMachines` and `LinodeMachineTemplates`. This will
be addressed in a later release. 
```

Example `LinodeMachineTemplate`:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeMachineTemplate
metadata:
  name: test-cluster-control-plane
  namespace: default
spec:
  template:
    spec:
      firewallRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
        kind: LinodeFirewall
        name: sample-fw
      image: linode/ubuntu22.04
      interfaces:
        - purpose: public
      region: us-ord
      type: g6-standard-4
```
