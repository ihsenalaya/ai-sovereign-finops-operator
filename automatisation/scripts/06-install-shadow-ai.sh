#!/usr/bin/env bash
# Install the gateway-independent Shadow-AI plane for the direct/local demo:
# Tetragon + rogue workload + shadow-egress ConfigMap refresh.
set -euo pipefail
DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${DIR}/common.sh"

export ENABLE_TETRAGON="${ENABLE_TETRAGON:-true}"
export ENABLE_SHADOW_ROGUE="${ENABLE_SHADOW_ROGUE:-true}"
export REQUIRE_SHADOW_EGRESS="${REQUIRE_SHADOW_EGRESS:-true}"
export TETRAGON_NS="${TETRAGON_NS:-kube-system}"
export SHADOW_NS="${SHADOW_NS:-default}"
export SHADOW_WORKLOAD_NS="${SHADOW_WORKLOAD_NS:-finance}"
export SHADOW_WAIT="${SHADOW_WAIT:-20}"
export SHADOW_REFRESH_RETRIES="${SHADOW_REFRESH_RETRIES:-6}"
export SHADOW_REFRESH_INTERVAL="${SHADOW_REFRESH_INTERVAL:-15}"

CTX="kind-${CLUSTER_NAME}"

if [ "${ENABLE_TETRAGON}" != "true" ]; then
  warn "Shadow-AI plane disabled (ENABLE_TETRAGON=false)."
  exit 0
fi

require helm
require kubectl
require python3

log "installing Shadow-AI plane (Tetragon + rogue app)..."
KCTX="${CTX}" TETRAGON_NS="${TETRAGON_NS}" "${AUTOMATISATION_DIR}/tetragon/install.sh"

if [ "${ENABLE_SHADOW_ROGUE}" != "true" ]; then
  warn "Shadow-AI rogue workload disabled (ENABLE_SHADOW_ROGUE=false)."
  kubectl --context "${CTX}" -n "${SHADOW_WORKLOAD_NS}" delete deploy/shadow-ai-rogue --ignore-not-found >/dev/null 2>&1 || true
  exit 0
fi

if ! kubectl --context "${CTX}" get namespace "${SHADOW_WORKLOAD_NS}" >/dev/null 2>&1; then
  kubectl --context "${CTX}" create namespace "${SHADOW_WORKLOAD_NS}" >/dev/null
fi

existed=false
if kubectl --context "${CTX}" -n "${SHADOW_WORKLOAD_NS}" get deploy/shadow-ai-rogue >/dev/null 2>&1; then
  existed=true
fi

kubectl --context "${CTX}" apply -f "${AUTOMATISATION_DIR}/tetragon/rogue-app.yaml"
if [ "${existed}" = "true" ]; then
  kubectl --context "${CTX}" -n "${SHADOW_WORKLOAD_NS}" rollout restart deploy/shadow-ai-rogue >/dev/null
fi
kubectl --context "${CTX}" -n "${SHADOW_WORKLOAD_NS}" rollout status deploy/shadow-ai-rogue --timeout=180s

shadow_json="[]"
for i in $(seq 1 "${SHADOW_REFRESH_RETRIES}"); do
  if [ "${i}" -eq 1 ]; then
    log "waiting ${SHADOW_WAIT}s for Tetragon to export shadow egress..."
    sleep "${SHADOW_WAIT}"
  else
    warn "shadow-egress still empty after refresh $((i - 1))/${SHADOW_REFRESH_RETRIES}; retrying in ${SHADOW_REFRESH_INTERVAL}s..."
    sleep "${SHADOW_REFRESH_INTERVAL}"
  fi

  NS="${SHADOW_NS}" KCTX="${CTX}" TETRAGON_NS="${TETRAGON_NS}" "${AUTOMATISATION_DIR}/tetragon/forwarder.sh"
  shadow_json="$(kubectl --context "${CTX}" -n "${SHADOW_NS}" get configmap shadow-egress -o jsonpath='{.data.egress\.json}' 2>/dev/null || echo '[]')"
  if [ -n "${shadow_json}" ] && [ "${shadow_json}" != "[]" ]; then
    log "shadow-egress populated: ${shadow_json}"
    break
  fi
done

kubectl --context "${CTX}" -n "${SHADOW_NS}" annotate aisovereigntypolicy regulated-france-policy \
  shadow/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true

if [ -z "${shadow_json}" ] || [ "${shadow_json}" = "[]" ]; then
  if [ "${REQUIRE_SHADOW_EGRESS}" = "true" ]; then
    die "shadow-egress is empty after ${SHADOW_REFRESH_RETRIES} refresh attempts"
  fi
  warn "shadow-egress is empty; Shadow-AI metrics will appear after real direct egress is observed."
fi

log "Shadow-AI ready. Inspect with:"
log "  kubectl --context ${CTX} -n ${SHADOW_NS} get cm shadow-egress -o jsonpath='{.data.egress\\.json}'"
log "  kubectl --context ${CTX} -n ${NS_OP:-greenops-system} port-forward svc/greenops-ai-sovereign-finops-operator-metrics 8080:8080"
