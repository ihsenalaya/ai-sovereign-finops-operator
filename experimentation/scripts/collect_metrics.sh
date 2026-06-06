#!/usr/bin/env bash
# Optional: scrape the operator's Prometheus ai_finops_* metrics when the operator
# is deployed (kind/AKS). The core experiment does not require this; it is for the
# live-operator integration phase.
set -euo pipefail
NS="${OPERATOR_NS:-greenops-system}"
SVC="${OPERATOR_SVC:-greenops-ai-sovereign-finops-operator-metrics}"
PORT="${METRICS_PORT:-8080}"

echo "[metrics] port-forwarding svc/$SVC in $NS ..."
kubectl -n "$NS" port-forward "svc/$SVC" "$PORT:$PORT" >/tmp/greenops-pf.log 2>&1 &
PF=$!
sleep 4
curl -s "localhost:$PORT/metrics" | grep "^ai_finops_" | tee results/operator_metrics.txt || true
kill "$PF" 2>/dev/null || true
echo "[metrics] wrote results/operator_metrics.txt"
