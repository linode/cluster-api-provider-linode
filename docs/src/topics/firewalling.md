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

For controlling firewalls via Linode resources, a [Cloud Firewall](https://www.linode.com/products/cloud-firewall/) can
be defined and provisioned via `LinodeFirewall` resources in CAPL. The created Cloud Firewall's ID can then be used in
a `LinodeMachine` or a `LinodeMachineTemplate`'s `firewallID` field. Note that the `firewallID` field is currently
immutable, so it must be set at creation time).

Cloud Firewalls are not automatically created for any CAPL flavor at this time.
