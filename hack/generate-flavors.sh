#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
FLAVORS_DIR="${REPO_ROOT}/templates/flavors"

for name in $(find "${FLAVORS_DIR}/"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v base); do
    # clusterctl expects clusterclass not have the "cluster-template" prefix
    # except for the actual cluster template using the clusterclass
    if [[ "$name" == clusterclass* ]]; then
        kustomize build "${FLAVORS_DIR}/${name}" > "${REPO_ROOT}/templates/${name}.yaml"
	cp "${FLAVORS_DIR}/${name}/cluster-template.yaml" "${REPO_ROOT}/templates/cluster-template-${name}.yaml"
    else
        kustomize build "${FLAVORS_DIR}/${name}" > "${REPO_ROOT}/templates/cluster-template-${name}.yaml"
    fi
done

# move the default template to the default file expected by clusterctl
mv "${REPO_ROOT}/templates/cluster-template-default.yaml" "${REPO_ROOT}/templates/cluster-template.yaml"
