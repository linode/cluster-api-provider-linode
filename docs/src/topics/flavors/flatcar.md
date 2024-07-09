# Flatcar

This flavor supports provisioning k8s clusters outside of VPC using [Flatcar][flatcar] as a base OS. It uses kubeadm for
setting up control plane and uses cilium with VXLAN for pod networking.

## Specification
| Supported Control Plane | CNI    | Default OS   | Installs ClusterClass | IPv4 | IPv6 |
|-------------------------|--------|--------------|-----------------------|------|------|
| kubeadm                 | Cilium | Flatcar      | No                    | Yes  | No   |

## Notes
This flavor is identical to the default flavor with the exception that it provisions
k8s clusters without VPC using [Flatcar][flatcar] as a base OS. Since it runs outside of VPC, native routing is not
supported in this flavor and it uses VXLAN for pod to pod communication.

## Usage

### Initialization

Before generating the cluster configuration, it is required to initialize the management cluster with [Ignition][ignition] support to provision Flatcar nodes:

```bash
export EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION=true
clusterctl init --infrastructure linode-linode --addon helm
```

### Import the Flatcar image

Flatcar is not officially provided by Akamai/Linode so it is required to import a Flatcar image. Akamai support is available on Flatcar since the release [4012.0.0][release-4012]: all releases equal or greater than this major release will fit.

To import the image, it is recommended to follow this documentation: https://www.flatcar.org/docs/latest/installing/community-platforms/akamai/#importing-an-image

By following this import step, you will get the Flatcar image ID stored into `IMAGE_ID`.

### Configure and deploy the workload cluster

1. Set the Flatcar image name from the previous step:
    ```bash
    export FLATCAR_IMAGE_NAME="${IMAGE_ID}"
    ```

2. Generate cluster yaml
    ```bash
    clusterctl generate cluster test-cluster \
        --kubernetes-version v1.29.1 \
        --infrastructure linode-linode \
        --flavor kubeadm-flatcar > test-cluster.yaml
    ```
2. Apply cluster yaml
    ```bash
    kubectl apply -f test-cluster.yaml
    ```

[flatcar]: https://www.flatcar.org/
[ignition]: https://coreos.github.io/ignition/
[release-4012]: https://www.flatcar.org/releases#release-4012.0.0
