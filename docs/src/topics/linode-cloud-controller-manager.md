# Linode Cloud Controller Manager

CAPL installs [linode-cloud-controller-manager (CCM)](https://github.com/linode/linode-cloud-controller-manager) by default to all child clusters
via [Cluster API Addon Provider Helm (CAAPH)](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm)

## Purpose of Linode CCM

CCM is linode specific implementation of [Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/). It implements below mentioned controllers:
* Node Controller: used for managing node objects in k8s cluster
* Service Controller: used for managing services and exposing them to outside world
* Route Controller: used for managing routes when running k8s cluster within VPC

## Installing CCM in custom environments (linode specific only)

When running CAPL in custom environments, one need to set additional environment vars. Linodego requires CA to be set so that it doesn't fail due to self signed certs. One can download the cert chain using:

```sh
echo -n | openssl s_client -showcerts -connect <URL>:443 | sed -ne '/-BEGIN CERTIFICATE-/,/-END CERTIFICATE-/p'
```
Check the cert contents and if its a CA, use it.

Additional vars which needs to be set for custom enviroments:
```sh
export LINODE_URL=<env specific API path>
export LINODE_EXTERNAL_SUBNET=<network to be marked as public network>
export LINODE_CA_BASE64=<base64 encoded value of LINODE_CA cert content>
```

When running CCM with [cilium-bgp](https://github.com/linode/linode-cloud-controller-manager?tab=readme-ov-file#shared-ip-load-balancing) mode in custom environment, one needs to also set:

```sh
export BGP_CUSTOM_ID_MAP=<custom id map to use>
export BGP_PEER_PREFIX=<peer prefix value>
```

## Additional details

Refer to the [Linode CCM Documentation](https://github.com/linode/linode-cloud-controller-manager/blob/main/README.md)
for further information on configuring and using CCM.
