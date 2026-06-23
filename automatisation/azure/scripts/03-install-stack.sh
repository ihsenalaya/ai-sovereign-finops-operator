#!/usr/bin/env bash
# Install the NVIDIA GPU Operator (drivers + device plugin + DCGM exporter) with
# tolerations for the GPU taint, then the AI Sovereign FinOps Operator (Helm).
# Prometheus is optional (INSTALL_PROM=true).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require helm
require kubectl

TOLER_KEY="${GPU_TAINT%%=*}"   # "sku"

# Set SKIP_GPU_OPERATOR=true for a CPU-only base cluster (before GPU quota).
if [ "${SKIP_GPU_OPERATOR:-false}" = "true" ]; then
  warn "SKIP_GPU_OPERATOR=true → installing operator only (no NVIDIA GPU Operator)."
else
  log "installing NVIDIA GPU Operator (tolerating ${GPU_TAINT})..."
  helm repo add nvidia https://helm.ngc.nvidia.com/nvidia >/dev/null 2>&1 || true
  helm repo update >/dev/null
  helm upgrade --install gpu-operator nvidia/gpu-operator \
    -n gpu-operator --create-namespace \
    --set-json "daemonsets.tolerations=[{\"key\":\"${TOLER_KEY}\",\"operator\":\"Equal\",\"value\":\"gpu\",\"effect\":\"NoSchedule\"}]" \
    --wait --timeout 15m
fi

# If the image is on GHCR (private), create an image pull secret so AKS can pull
# it. Token from GHCR_TOKEN env, else the gh CLI credentials (needs read:packages).
PULL_ARGS=()
case "${OPERATOR_IMAGE}" in
  ghcr.io/*)
    GHCR_USER="${GHCR_USER:-$(printf '%s' "${OPERATOR_IMAGE}" | cut -d/ -f2)}"
    GHCR_TOKEN="${GHCR_TOKEN:-$(grep -E 'oauth_token:' "${HOME}/.config/gh/hosts.yml" 2>/dev/null | head -1 | awk '{print $2}')}"
    if [ -n "${GHCR_TOKEN}" ]; then
      kubectl create namespace greenops-system --dry-run=client -o yaml | kubectl apply -f - >/dev/null
      kubectl -n greenops-system create secret docker-registry ghcr-pull \
        --docker-server=ghcr.io --docker-username="${GHCR_USER}" --docker-password="${GHCR_TOKEN}" \
        --docker-email="ci@greenops.local" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
      PULL_ARGS=(--set imagePullSecrets[0].name=ghcr-pull)
      log "created GHCR image pull secret (user=${GHCR_USER})."
    else
      warn "no GHCR token; if the image is private the pull will fail. Set GHCR_TOKEN or make the package public."
    fi
    ;;
esac

log "installing AI Sovereign FinOps Operator (${OPERATOR_IMAGE}:${OPERATOR_TAG})..."
helm upgrade --install greenops "${OPERATOR_DIR}/charts/ai-sovereign-finops-operator" \
  -n greenops-system --create-namespace \
  --set image.repository="${OPERATOR_IMAGE}" \
  --set image.tag="${OPERATOR_TAG}" \
  "${PULL_ARGS[@]}" \
  --wait --timeout 5m

if [ "${INSTALL_PROM:-false}" = "true" ]; then
  log "installing kube-prometheus-stack (optional)..."
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
  helm repo update >/dev/null
  helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    -n monitoring --create-namespace --wait --timeout 10m
fi

log "stack installed. Deploy vLLM: 04-deploy-vllm.sh"
