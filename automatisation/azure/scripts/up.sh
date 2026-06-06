#!/usr/bin/env bash
# Full Azure bring-up: AKS + GPU pool + stack + vLLM + bench. Pre-req: GPU quota.
set -euo pipefail
DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${DIR}/common.sh"

"${DIR}/00-quota-check.sh" || warn "quota check returned non-zero; continuing (verify quota!)"
"${DIR}/00b-keyvault.sh"
"${DIR}/01-create-aks.sh"
"${DIR}/02-add-gpu-nodepool.sh"
"${DIR}/03-install-stack.sh"
"${DIR}/04-deploy-vllm.sh"
"${DIR}/05-run-bench.sh"
"${DIR}/06-collect-cost.sh"

log "Azure run complete. Scale GPU to 0 when done:  MODE=idle ${DIR}/down.sh"
