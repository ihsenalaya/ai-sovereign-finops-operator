#!/usr/bin/env bash
# Create (idempotently) the kind cluster used for the demo.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kind
require kubectl

if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  log "kind cluster '${CLUSTER_NAME}' already exists; reusing it."
else
  log "creating kind cluster '${CLUSTER_NAME}' with node image ${KIND_NODE_IMAGE}..."
  kind create cluster --name "${CLUSTER_NAME}" --image "${KIND_NODE_IMAGE}" --config "${AUTOMATISATION_DIR}/kind/kind-config.yaml"
fi

kind export kubeconfig --name "${CLUSTER_NAME}"
kubectl cluster-info --context "kind-${CLUSTER_NAME}" >/dev/null
log "cluster ready (context kind-${CLUSTER_NAME})."
