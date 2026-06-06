#!/usr/bin/env bash
# Deploy vLLM (OpenAI-compatible) on the GPU pool. Scheduling the pod triggers
# the GPU node autoscale 0->1.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require kubectl

kubectl create namespace vllm --dry-run=client -o yaml | kubectl apply -f -

if [ -n "${HF_TOKEN}" ]; then
  kubectl -n vllm create secret generic hf-token --from-literal=token="${HF_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

log "deploying vLLM model=${VLLM_MODEL} served=${VLLM_SERVED_NAME} args='${VLLM_EXTRA_ARGS:-}'..."
sed -e "s|__MODEL__|${VLLM_MODEL}|g" -e "s|__SERVED__|${VLLM_SERVED_NAME}|g" \
    -e "s|__EXTRA_ARGS__|${VLLM_EXTRA_ARGS:-}|g" \
  "${AZURE_DIR}/manifests/vllm.yaml" | kubectl apply -f -

log "waiting for vLLM to be ready (model download + GPU scale-up can take 10-20 min)..."
kubectl -n vllm rollout status deploy/vllm --timeout=30m
log "vLLM ready at service vllm.vllm.svc:8000 (OpenAI-compatible /v1). Run: 05-run-bench.sh"
