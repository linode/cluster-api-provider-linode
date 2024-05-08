#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
STANDARD_ADDONS_DIR="${REPO_ROOT}/templates/addons"
DISTRO_DIR="${REPO_ROOT}/templates/distros"

SUPPORTED_DISTROS=(
    "rke2"
    "k3s"
    "kubeadm"
)

for distro in SUPPORTED_DISTROS; do
    paths=()
    kustomize_file_path="kustomization.yaml"
    paths+=(${DISTRO_DIR}/${distro}/${kustomize_file_path})
    for addon in $(find "${STANDARD_ADDONS_DIR}/"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {}); do
        paths+=(${STANDARD_ADDONS_DIR}/${addon}/${kustomize_file_path})
    done
    spruce merge --fallback-append ${paths} > kustomization.yaml
done

kustomize build "${STANDARD_ADDONS_DIR}/${addon}" > "${REPO_ROOT}/templates/cluster-template-${distro}-standard.yaml"


# move the default template to the default file expected by clusterctl
mv "${REPO_ROOT}/templates/cluster-template-default.yaml" "${REPO_ROOT}/templates/cluster-template.yaml"

