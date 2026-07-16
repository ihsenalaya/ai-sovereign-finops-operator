#!/usr/bin/env bash
# Delete the ai-govar-operator kind cluster.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require kind
log "deleting kind cluster '${CLUSTER_NAME}'..."
kind delete cluster --name "${CLUSTER_NAME}"
