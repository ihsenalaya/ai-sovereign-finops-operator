#!/usr/bin/env bash
# Record GPU node-hours and approximate cost for the break-even validation.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az
require kubectl

OUT="${REPO_ROOT}/experimentation/results/azure_cost.txt"
{
  echo "# Azure GPU cost snapshot ($(date -u +%FT%TZ))"
  echo "cluster=${CLUSTER} rg=${RG} location=${LOCATION} gpu_sku=${GPU_VMSIZE}"
  echo
  echo "## GPU nodes currently running:"
  kubectl get nodes -l sku=gpu -o wide 2>/dev/null || true
  echo
  echo "## Recent usage (consumption API; may lag a few hours):"
  az consumption usage list --top 20 \
    --query "[?contains(instanceName, '${CLUSTER}')].{date:date,meter:meterDetails.meterName,qty:quantity,cost:pretaxCost,cur:currency}" \
    -o table 2>/dev/null || echo "(consumption data not yet available)"
} | tee "${OUT}"
log "wrote ${OUT}"
warn "Set GPU_VMSIZE hourly price in experimentation/internal/catalog for the modeled/measured break-even comparison."
