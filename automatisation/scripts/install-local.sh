#!/usr/bin/env bash
# Offline / no-GitOps path: kind + image + Helm install directly (no ArgoCD).
# This is the path validated end-to-end during development.
set -euo pipefail
DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${DIR}/common.sh"

require helm
require kubectl

"${DIR}/01-create-cluster.sh"
"${DIR}/02-build-load-image.sh"

log "installing the operator Helm chart..."
helm upgrade --install greenops "${REPO_ROOT}/charts/ai-sovereign-finops-operator" \
  -n greenops-system --create-namespace \
  --set image.repository="${IMAGE_REPO}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=Never

kubectl -n greenops-system rollout status deploy/greenops-ai-sovereign-finops-operator --timeout=180s

log "applying demo catalog & policies..."
kubectl apply -k "${REPO_ROOT}/config/samples/"

log "Done. Inspect with:"
log "  kubectl get aigw,aiprov,aimodel,aibudget,aisov,aibreakeven,aireport -A"
log "  kubectl get cm monthly-ai-report-rh-report -o jsonpath='{.data.report\\.md}'"
