#!/usr/bin/env bash
# Install the NVIDIA GPU Operator (drivers + device plugin + DCGM exporter) with
# tolerations for the GPU taint, then the AI Sovereign FinOps Operator (Helm).
# Prometheus is optional (INSTALL_PROM=true).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require helm
require kubectl

TOLER_KEY="${GPU_TAINT%%=*}"   # "sku"

log "installing NVIDIA GPU Operator (tolerating ${GPU_TAINT})..."
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia >/dev/null 2>&1 || true
helm repo update >/dev/null
helm upgrade --install gpu-operator nvidia/gpu-operator \
  -n gpu-operator --create-namespace \
  --set-json "daemonsets.tolerations=[{\"key\":\"${TOLER_KEY}\",\"operator\":\"Equal\",\"value\":\"gpu\",\"effect\":\"NoSchedule\"}]" \
  --wait --timeout 15m

log "installing AI Sovereign FinOps Operator (${OPERATOR_IMAGE}:${OPERATOR_TAG})..."
helm upgrade --install greenops "${REPO_ROOT}/charts/ai-sovereign-finops-operator" \
  -n greenops-system --create-namespace \
  --set image.repository="${OPERATOR_IMAGE}" \
  --set image.tag="${OPERATOR_TAG}" \
  --wait --timeout 5m

if [ "${INSTALL_PROM:-false}" = "true" ]; then
  log "installing kube-prometheus-stack (optional)..."
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
  helm repo update >/dev/null
  helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    -n monitoring --create-namespace --wait --timeout 10m
fi

log "stack installed. Deploy vLLM: 04-deploy-vllm.sh"
