#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
FLAVORS_DIR="${REPO_ROOT}/templates/flavors"
SUPPORTED_DISTROS=(
    "rke2"
    "k3s"
    "kubeadm"
)
SUPPORTED_CLUSTERCLASSES=(
    "clusterclass-kubeadm"
)

for clusterclass in ${SUPPORTED_CLUSTERCLASSES[@]}; do
    # clusterctl expects clusterclass not have the "cluster-template" prefix
    # except for the actual cluster template using the clusterclass
    echo "****** Generating clusterclass-${clusterclass} flavor ******"
    kustomize build "${FLAVORS_DIR}/${clusterclass}" > "${REPO_ROOT}/templates/${clusterclass}.yaml"
    cp "${FLAVORS_DIR}/${clusterclass}/cluster-template.yaml" "${REPO_ROOT}/templates/cluster-template-${clusterclass}.yaml"
done


for distro in ${SUPPORTED_DISTROS[@]}; do
    if [[ ${distro} == "kubeadm" ]]; then
      CACERT="ca.crt"
      CERT="healthcheck-client.crt"
      KEY="healthcheck-client.key"
      CERT_HOSTPATH="/etc/kubernetes/pki"
    elif [[ ${distro} == "k3s" ]]; then
      CACERT="server-ca.crt"
      CERT="server-client.crt"
      KEY="server-client.key"
      CERT_HOSTPATH="/var/lib/rancher/k3s/server/tls"
    elif [[ ${distro} == "rke2" ]]; then
      CACERT="server-ca.crt"
      CERT="server-client.crt"
      KEY="server-client.key"
      CERT_HOSTPATH="/var/lib/rancher/rke2/server/tls"
    fi
    for name in $(find "${FLAVORS_DIR}/${distro}/"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {}); do
        if [[ ${name} == "default" ]]; then
            echo "****** Generating ${distro} flavor ******"
            kustomize build "${FLAVORS_DIR}/${distro}/${name}" > "${REPO_ROOT}/templates/cluster-template-${distro}.yaml"
        else
            echo "****** Generating ${distro}-${name} flavor ******"
            kustomize build "${FLAVORS_DIR}/${distro}/${name}" > "${REPO_ROOT}/templates/cluster-template-${distro}-${name}.yaml"
            if grep -Fq "etcd-backup-restore" "${REPO_ROOT}/templates/cluster-template-${distro}-${name}.yaml"; then
                sed -i -e "s|\${CERTPATH}|${CERT_HOSTPATH}|g; s|\${CACERTFILE}|${CACERT}|g; s|\${CERTFILE}|${CERT}|g; s|\${KEYFILE}|${KEY}|g" "${REPO_ROOT}/templates/cluster-template-${distro}-${name}.yaml"
            fi
        fi
    done
done

# move the default template to the default file expected by clusterctl
mv "${REPO_ROOT}/templates/cluster-template-kubeadm.yaml" "${REPO_ROOT}/templates/cluster-template.yaml"
