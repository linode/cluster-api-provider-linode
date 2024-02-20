#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
FLAVORS_DIR="${REPO_ROOT}/templates/flavors"

for name in $(find "${FLAVORS_DIR}/"* -maxdepth 0 -type d -print0 | xargs -0 -I {} basename {} | grep -v base); do
  kustomize build "${FLAVORS_DIR}/${name}" > "${REPO_ROOT}/templates/cluster-template-${name}.yaml"
done

# move the default template to the default file expected by clusterctl
mv "${REPO_ROOT}/templates/cluster-template-default.yaml" "${REPO_ROOT}/templates/cluster-template.yaml"
