#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
AUTO="$(cd "${HERE}/.." && pwd)"

CLUSTER="${CLUSTER:-greenops}"
KCTX="${KCTX:-kind-${CLUSTER}}"
NS="${NS:-default}"

K=(kubectl --context "${KCTX}")

echo "==> Bringing up the real AI Gateway demo"
ENABLE_TETRAGON=true ENABLE_SHADOW_ROGUE=true SHADOW_NS="${NS}" KCTX="${KCTX}" \
  "${AUTO}/envoy-aigw/deploy.sh" up

echo "==> Shadow-AI demo ready"
echo "Grafana panels: 'Shadow-AI egress details' and 'Shadow-AI hotspots by workload'"
