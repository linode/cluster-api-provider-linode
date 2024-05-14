#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
FLAVORS_DIR="${REPO_ROOT}/templates/flavors"
SUPPORTED_DISTROS=(
    "clusterclass-kubeadm"
    "rke2"
    "k3s"
    "kubeadm"
)

for distro in ${SUPPORTED_DISTROS[@]}; do
    if [[ $distro == "clusterclass-kubeadm" ]]; then
        # clusterctl expects clusterclass not have the "cluster-template" prefix
        # except for the actual cluster template using the clusterclass
        echo "****** Generating clusterclass-kubeadm flavor ******"
        kustomize build "${FLAVORS_DIR}/${distro}" > "${REPO_ROOT}/templates/${distro}.yaml"
        cp "${FLAVORS_DIR}/${distro}/cluster-template.yaml" "${REPO_ROOT}/templates/cluster-template-${distro}.yaml"
        continue
    fi

    for name in $(find "${FLAVORS_DIR}/${distro}/"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {}); do
        echo "****** Generating ${distro}-${name} flavor ******"
        kustomize build "${FLAVORS_DIR}/${distro}/${name}" > "${REPO_ROOT}/templates/cluster-template-${distro}-${name}.yaml"
    done
done
