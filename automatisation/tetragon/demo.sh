#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
AUTO="$(cd "${HERE}/.." && pwd)"

CLUSTER="${CLUSTER:-greenops}"
KCTX="${KCTX:-kind-${CLUSTER}}"
NS="${NS:-default}"

K=(kubectl --context "${KCTX}")

echo "==> Bringing up the real AI Gateway demo"
"${AUTO}/envoy-aigw/deploy.sh" up

echo "==> Installing Tetragon"
KCTX="${KCTX}" "${HERE}/install.sh"

echo "==> Deploying the rogue shadow-AI workload"
"${K[@]}" apply -f "${HERE}/rogue-app.yaml"
"${K[@]}" -n finance rollout status deploy/shadow-ai-rogue --timeout=120s

echo "==> Waiting for Tetragon to export fresh events"
sleep "${SHADOW_WAIT:-10}"

echo "==> Refreshing shadow-egress from Tetragon"
NS="${NS}" KCTX="${KCTX}" "${HERE}/forwarder.sh"

echo "==> Nudging the sovereignty policy reconcile"
"${K[@]}" -n "${NS}" annotate aisovereigntypolicy regulated-france-policy \
  demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true

echo "==> Shadow-AI demo ready"
echo "Grafana panels: 'Shadow-AI egress details' and 'Shadow-AI hotspots by workload'"
