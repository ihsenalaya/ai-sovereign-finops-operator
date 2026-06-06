#!/usr/bin/env bash
# Benchmark the in-cluster vLLM via port-forward, sampling GPU utilization from
# the DCGM exporter. Reuses the locally-validated gpubench harness. Writes to
# experimentation/results/gpubench.csv.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require kubectl
require go

CONCURRENCIES="${CONCURRENCIES:-1 2 4 8}"
REQUESTS="${REQUESTS:-60}"
MAXTOK="${MAXTOK:-128}"

cleanup() { kill ${PF_VLLM:-} ${PF_DCGM:-} 2>/dev/null || true; }
trap cleanup EXIT

log "port-forwarding vLLM (8000) ..."
kubectl -n vllm port-forward svc/vllm 8000:8000 >/tmp/pf-vllm.log 2>&1 &
PF_VLLM=$!

DCGM_SVC="$(kubectl -n gpu-operator get svc -o name 2>/dev/null | grep -i dcgm | head -1 || true)"
DCGM_ARG=""
if [ -n "${DCGM_SVC}" ]; then
  log "port-forwarding DCGM (${DCGM_SVC} 9400) ..."
  kubectl -n gpu-operator port-forward "${DCGM_SVC}" 9400:9400 >/tmp/pf-dcgm.log 2>&1 &
  PF_DCGM=$!
  DCGM_ARG="-dcgm http://localhost:9400/metrics"
else
  warn "DCGM exporter not found; GPU utilization will be 0 in results."
fi
sleep 6

cd "${REPO_ROOT}/experimentation"
for c in ${CONCURRENCIES}; do
  log "bench concurrency=${c} requests=${REQUESTS} model=${VLLM_SERVED_NAME}"
  go run ./cmd/gpubench \
    -base http://localhost:8000/v1 -model "${VLLM_SERVED_NAME}" \
    -concurrency "${c}" -requests "${REQUESTS}" -max-tokens "${MAXTOK}" \
    ${DCGM_ARG} -results results -label "vllm-${VLLM_SERVED_NAME}-c${c}"
done

log "done -> experimentation/results/gpubench.csv"
