# Kubeadm ClusterClass
## Specification
| Control Plane | CNI    | Default OS   | Installs ClusterClass |
|---------------|--------|--------------|-----------------------|
| Kubeadm       | Cilium | Ubuntu 22.04 | Yes                   | 
## Prerequisites
[Quickstart](../getting-started.md) completed
## Usage
### Create clusterClass and first cluster
1. Generate the ClusterClass and cluster manifests
    ```bash
    clusterctl generate cluster test-cluster --infrastructure linode:0.0.0  --flavor clusterclass-kubeadm > test-cluster.yaml
    ```
2. Apply cluster manifests
    ```bash
    kubectl apply -f test-cluster.yaml
    ```
### (Optional) Create a second cluster using the existing ClusterClass
1.  Generate cluster manifests
      ```bash
      clusterctl generate cluster test-cluster-2 --flavor clusterclass-kubeadm > test-cluster-2.yaml
      ```
      ```yaml
      apiVersion: cluster.x-k8s.io/v1beta1
      kind: Cluster
      metadata:
        labels:
          ccm: linode
          cni: cilium
          crs: test-cluster-2-crs
        name: test-cluster-2
        namespace: default
      spec:
        clusterNetwork:
          pods:
            cidrBlocks:
            - 10.192.0.0/10
        topology:
          class: kubeadm
          controlPlane:
            replicas: 1
          variables:
          - name: region
            value: us-ord
          - name: controlPlaneMachineType
            value: g6-standard-2
          - name: workerMachineType
            value: g6-standard-2
          version: v1.29.1
          workers:
            machineDeployments:
            - class: default-worker
              name: md-0
              replicas: 1
      ```
2. Apply cluster manifests
    ```bash
    kubectl apply -f test-cluster-2.yaml
    ```
