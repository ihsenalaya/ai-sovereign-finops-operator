#!/usr/bin/env bash
# E8-bis: measure CONTROL-PLANE (controller) overhead as the number of reconciled
# CRD objects grows. No paid LLM calls: we scale lightweight AIBudgetPolicy and
# AISovereigntyPolicy objects (pure reconciliation, no pods, no gateway calls).
# At each tier we scrape the operator's controller-runtime metrics.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KC="${KUBECONFIG_PATH:-$ROOT/experiments/evidence/aks-live/aks-kubeconfig}"
K="kubectl --kubeconfig $KC"
NS="${E8BIS_NS:-e8bis-controller}"
TIERS="${E8BIS_TIERS:-20 60 120}"
SETTLE="${E8BIS_SETTLE_SECONDS:-75}"
MPORT="${E8BIS_METRICS_PORT:-18086}"
RUN_ID="aks-live-$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$ROOT/reports/M2_controller_overhead/$RUN_ID"
mkdir -p "$OUT/snapshots"

log(){ printf '[E8bis] %s\n' "$*"; }

# start metrics port-forward
$K -n greenops-system port-forward svc/greenops-ai-sovereign-finops-operator-metrics "$MPORT:8080" >/dev/null 2>&1 &
PF=$!
trap 'kill $PF 2>/dev/null || true' EXIT
sleep 6
scrape(){ curl -s "http://127.0.0.1:$MPORT/metrics" > "$1"; }

$K create namespace "$NS" --dry-run=client -o yaml | $K apply -f - >/dev/null

apply_tier(){
  local n="$1"
  for i in $(seq 1 "$n"); do
    cat <<EOF
---
apiVersion: aiops.imperium.io/v1alpha1
kind: AIBudgetPolicy
metadata:
  name: e8bis-budget-$i
  namespace: $NS
  labels: {article2-e8bis: "true"}
spec:
  budgetEUR: 1000m
  period: monthly
  warningThresholdPercent: 70
  criticalThresholdPercent: 90
  hardLimitPercent: 100
  enforcementMode: reportOnly
  fallbackOnPhase: Exceeded
  actions: {onWarning: [alert], onCritical: [alert], onHardLimit: [alert]}
  target: {namespace: $NS, application: app-$i, team: e8bis}
---
apiVersion: aiops.imperium.io/v1alpha1
kind: AISovereigntyPolicy
metadata:
  name: e8bis-sov-$i
  namespace: $NS
  labels: {article2-e8bis: "true"}
spec:
  enforcementMode: reportOnly
  dataResidency: {allowedZones: [FR, EU], forbiddenZones: [US, CN, GLOBAL]}
  sensitiveData: {externalProvidersAllowed: false, requireAnonymization: true}
EOF
  done | $K apply -f - >/dev/null
}

scrape "$OUT/snapshots/tier00_baseline.prom"
log "baseline scraped"

for T in $TIERS; do
  log "applying tier: $T budget + $T sovereignty policies"
  apply_tier "$T"
  log "settling ${SETTLE}s"
  sleep "$SETTLE"
  scrape "$OUT/snapshots/tier_$(printf '%03d' "$T").prom"
  cur=$($K -n "$NS" get aibudgetpolicy -l article2-e8bis=true --no-headers 2>/dev/null | wc -l)
  log "tier $T scraped (live budget policies: $cur)"
done

log "cleanup"
$K delete namespace "$NS" --wait=false >/dev/null 2>&1 || true
echo "$RUN_ID" > "$OUT/RUN_ID.txt"
log "done -> $OUT"
