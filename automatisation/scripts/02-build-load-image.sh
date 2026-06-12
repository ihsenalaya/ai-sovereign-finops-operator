#!/usr/bin/env bash
# Build the operator image and load it into the kind cluster.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require docker
require kind

log "building image ${IMAGE_REPO}:${IMAGE_TAG}..."
DOCKER_BUILDKIT=0 docker build -t "${IMAGE_REPO}:${IMAGE_TAG}" "${REPO_ROOT}"

log "loading image into kind cluster '${CLUSTER_NAME}'..."
kind load docker-image "${IMAGE_REPO}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"
log "image available in-cluster (pullPolicy: Never)."
