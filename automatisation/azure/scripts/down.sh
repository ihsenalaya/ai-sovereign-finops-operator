#!/usr/bin/env bash
# Cost control. MODE=idle scales the GPU pool to 0 (keeps cluster); MODE=delete
# removes the whole resource group.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

MODE="${MODE:-idle}"
case "${MODE}" in
  idle)
    log "scaling GPU pool ${GPU_POOL} to 0 (cluster kept)..."
    kubectl -n vllm delete deploy vllm --ignore-not-found 2>/dev/null || true
    az aks nodepool scale -g "${RG}" --cluster-name "${CLUSTER}" -n "${GPU_POOL}" --node-count 0 -o table || \
      az aks nodepool update -g "${RG}" --cluster-name "${CLUSTER}" -n "${GPU_POOL}" \
        --update-cluster-autoscaler --min-count 0 --max-count "${GPU_MAX}" -o table
    log "GPU pool at 0; you stop paying GPU. Re-deploy vLLM to scale back up."
    ;;
  delete)
    warn "deleting resource group ${RG} (everything)..."
    az group delete -n "${RG}" --yes --no-wait
    log "deletion started."
    ;;
  *) die "unknown MODE=${MODE} (use idle|delete)" ;;
esac
