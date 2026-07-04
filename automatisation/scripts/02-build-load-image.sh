#!/usr/bin/env bash
# Build all platform images and load them into the kind cluster.
# Image names match the Helm chart values.yaml repository fields:
#   controller, attestation-scheduler, key-release-gateway,
#   platform-api, platform-ui, thesis-bench
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require docker
require kind

PLATFORM_UI_DIR="${REPO_ROOT}/platform-ui"

if [ "${SKIP_BUILD:-false}" != "true" ]; then
  log "building images (tag: ${IMAGE_TAG}) — sequential to avoid OOM on WSL2..."

  docker build -t "controller:${IMAGE_TAG}" "${OPERATOR_DIR}"
  docker build -t "attestation-scheduler:${IMAGE_TAG}" \
    -f "${OPERATOR_DIR}/Dockerfile.scheduler" "${OPERATOR_DIR}"
  docker build -t "key-release-gateway:${IMAGE_TAG}" \
    -f "${OPERATOR_DIR}/Dockerfile.key-release-gateway" "${OPERATOR_DIR}"
  docker build -t "platform-api:${IMAGE_TAG}" \
    -f "${OPERATOR_DIR}/Dockerfile.platform-api" "${OPERATOR_DIR}"
  docker build -t "thesis-bench:${IMAGE_TAG}" \
    -f "${OPERATOR_DIR}/Dockerfile.thesis-bench" "${OPERATOR_DIR}"

  if [ -f "${PLATFORM_UI_DIR}/src/main.tsx" ]; then
    log "building platform-ui:${IMAGE_TAG}..."
    docker build -t "platform-ui:${IMAGE_TAG}" "${PLATFORM_UI_DIR}"
  else
    log "platform-ui/src/main.tsx not found — pulling nginx placeholder..."
    docker pull nginxinc/nginx-unprivileged:1.27-alpine
    docker tag nginxinc/nginx-unprivileged:1.27-alpine "platform-ui:${IMAGE_TAG}"
  fi
else
  log "SKIP_BUILD=true — skipping all image builds."
fi

# Load all images into the kind cluster
for img in controller attestation-scheduler key-release-gateway platform-api platform-ui thesis-bench; do
  log "loading ${img}:${IMAGE_TAG} into kind cluster '${CLUSTER_NAME}'..."
  kind load docker-image "${img}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"
done

log "all 6 images available in-cluster (imagePullPolicy: IfNotPresent)."
