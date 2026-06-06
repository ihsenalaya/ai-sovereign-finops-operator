#!/usr/bin/env bash
# Add a GPU node pool that autoscales 0..MAX (pay GPU only while a Job is pending).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

if az aks nodepool show -g "${RG}" --cluster-name "${CLUSTER}" -n "${GPU_POOL}" >/dev/null 2>&1; then
  log "GPU pool ${GPU_POOL} exists; ensuring autoscale ${GPU_MIN}..${GPU_MAX}."
  az aks nodepool update -g "${RG}" --cluster-name "${CLUSTER}" -n "${GPU_POOL}" \
    --enable-cluster-autoscaler --min-count "${GPU_MIN}" --max-count "${GPU_MAX}" -o table || true
else
  log "adding GPU pool ${GPU_POOL} (${GPU_VMSIZE}, autoscale ${GPU_MIN}..${GPU_MAX})..."
  az aks nodepool add -g "${RG}" --cluster-name "${CLUSTER}" -n "${GPU_POOL}" \
    --node-vm-size "${GPU_VMSIZE}" \
    --node-count "${GPU_MIN}" \
    --enable-cluster-autoscaler --min-count "${GPU_MIN}" --max-count "${GPU_MAX}" \
    --node-taints "${GPU_TAINT}" \
    --labels sku=gpu \
    -o table
fi
log "GPU pool ready (scales to ${GPU_MIN} when idle). Install NVIDIA stack: 03-install-stack.sh"
