#!/usr/bin/env bash
# Check (and help request) GPU vCPU quota for the chosen SKU family/region.
# MPN / Visual Studio subscriptions usually start at 0 GPU quota.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

log "Subscription:"; az account show --query "{name:name,id:id}" -o table

# Map SKU -> quota family (common cases).
family="standardNCASv3Family"   # T4 (NC*as_T4_v3)
case "${GPU_VMSIZE}" in
  *H100*) family="standardNCadsH100v5Family" ;;
  *A100*) family="standardNCADSA100v4Family" ;;
  *A10*)  family="standardNVadsA10v5Family" ;;
  *T4*)   family="standardNCASv3Family" ;;
esac

log "GPU SKU=${GPU_VMSIZE} -> quota family=${family} in ${LOCATION}"
log "Current usage/limit for GPU families in ${LOCATION}:"
az vm list-usage --location "${LOCATION}" -o table 2>/dev/null | grep -iE "NC|ND|NV|Total Regional" || warn "could not list usage"

cat <<EOF

If the limit for ${family} is 0, request an increase before deploying:

  Portal: Subscription > Usage + quotas > Compute > region=${LOCATION} > family ${family} > Request increase
  CLI (support ticket):
    az support tickets create --ticket-name greenops-gpu-quota \\
      --issue-type quota --quota-ticket-details ... (see docs)

Cheapest path likely to be granted on MPN: T4 (${GPU_VMSIZE}). Ask for >=4 vCPU of the family.
EOF
