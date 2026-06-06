#!/usr/bin/env bash
# Install ArgoCD into the cluster and expose its server on NodePort 30080.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kubectl

log "installing ArgoCD into namespace '${ARGOCD_NAMESPACE}'..."
kubectl create namespace "${ARGOCD_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n "${ARGOCD_NAMESPACE}" -f "${ARGOCD_MANIFEST}"

log "waiting for ArgoCD server to become available (this can take a few minutes)..."
kubectl -n "${ARGOCD_NAMESPACE}" rollout status deploy/argocd-server --timeout=300s

# Expose the UI on a stable NodePort (matches kind-config extraPortMappings).
kubectl -n "${ARGOCD_NAMESPACE}" patch svc argocd-server \
  --type merge \
  -p '{"spec":{"type":"NodePort","ports":[{"name":"http","port":80,"targetPort":8080,"nodePort":30080}]}}'

log "ArgoCD installed."
log "  UI:       http://localhost:30080  (insecure; use --insecure or port-forward for HTTPS)"
log "  user:     admin"
log "  password: kubectl -n ${ARGOCD_NAMESPACE} get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
