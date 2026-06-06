#!/usr/bin/env bash
# Wait for ArgoCD Applications to become Synced + Healthy, then show the result.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kubectl

apps=(greenops-operator greenops-samples)

get() { kubectl -n "${ARGOCD_NAMESPACE}" get application "$1" -o jsonpath="$2" 2>/dev/null || true; }

log "waiting for Applications to sync (up to ~10 min)..."
for i in $(seq 1 200); do
  ok=true
  for a in "${apps[@]}"; do
    sync="$(get "$a" '{.status.sync.status}')"
    health="$(get "$a" '{.status.health.status}')"
    [ "$sync" = "Synced" ] && [ "$health" = "Healthy" ] || ok=false
  done
  if $ok; then log "all Applications Synced + Healthy."; break; fi
  sleep 3
  [ "$i" = "200" ] && warn "timeout waiting for sync; see 'kubectl -n ${ARGOCD_NAMESPACE} get applications'"
done

echo
kubectl -n "${ARGOCD_NAMESPACE}" get applications
echo
kubectl get aigw,aiprov,aimodel,aibudget,aisov,aibreakeven,aireport 2>/dev/null || true
