# Firewalls

CAPL supports attaching Linode [Cloud Firewalls](https://www.linode.com/docs/products/networking/cloud-firewall/get-started/)
to workload clusters to secure network traffic. There are
two different types of Cloud Firewalls that CAPL can create.

~~~admonish warning
Cloud Firewall rules are applied to traffic over the public and private
network but are not applied to traffic over a private
[VLAN](https://www.linode.com/docs/products/networking/vlans/).
~~~

## Control Plane Firewall

By default, workload clusters are created with their own Cloud Firewall
attached to each Linode instance assigned as a control plane node.

Access to these instances are automatically updated for any rule changes made to
the default control plane firewall after the cluster is created.

### Inbound Access

At cluster provisioning time, this firewall can be configured with
an allowlist of IPs to permit access. If no list is provided, all
IPs are permitted. For the Kubernetes API endpoint, all cluster nodes
are permitted access

Please refer to the below table for configured service access:

| Service (`Port`)          | Allowed IPs                                                          |
| ------------------------- | -------------------------------------------------------------------- |
| Kubernetes API (`6443`)   | `<All Node IPs>,<Authorized IPs>` (default: `[0.0.0.0/0,::/0]`)      |
| NodePorts (`30000-32767`) | `<Authorized IPs>` (default: `[0.0.0.0/0,::/0]`)                     |
| SSH (`22`)                | `<Authorized IPs>` (default: `[0.0.0.0/0,::/0]`)                     |

### Outbound Access

All outbound access from the control plane is by default permitted.

## Worker Firewall

By default, workload clusters are created with their own Cloud Firewall
attached to each Linode instance assigned as a worker node.

Access to these instances are automatically updated for any rule changes made to
the default worker firewall after the cluster is created.

### Inbound Access

At cluster provisioning time, this firewall can be configured with
an allowlist of IPs to permit access. If no list is provided, all
IPs are permitted. 

Please refer to the below table for configured service access:

| Service (`Port`)          | Allowed IPs                                                          |
| ------------------------- | -------------------------------------------------------------------- |
| NodePorts (`30000-32767`) | `<Authorized IPs>` (default: `[0.0.0.0/0,::/0]`)                     |
| SSH (`22`)                | `<Authorized IPs>` (default: `[0.0.0.0/0,::/0]`)                     |

### Outbound Access

All outbound access from the workers is by default permitted.

## Additional Cloud Firewalls

If needed, additional control plane and/or worker firewalls can be created for
one or more workload clusters. This is done by creating a `LinodeFirewall` CRD
and adding it to the Cluster's `spec.controlPlaneFirewallRefs` or
`spec.workerFirewallRefs`.

To remove the additional firewall(s) from a workload cluster, update the Cluster's
`spec.controlPlaneFirewallRefs` or `spec.workerFirewallRefs`.
