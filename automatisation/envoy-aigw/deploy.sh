#!/usr/bin/env bash
# ============================================================================
# greenops — full reproducible REAL demo, from scratch, no manual steps.
#
# Real apps consume tokens through Envoy AI Gateway; the operator reads the
# gateway's real per-app token usage and computes cost / sovereignty / budget;
# Prometheus + Grafana show truthful values.
#
# Brings up, in order and idempotently:
#   1.  kind cluster (created if missing)
#   2a. operator image — built from THIS source (default) and loaded into kind, so
#       the deployed operator always matches the repo/dashboard (GHCR is private)
#   2.  the operator (Helm chart + CRDs)
#   3.  Envoy Gateway v1.5.0 (with AI Gateway values) + Envoy AI Gateway v0.7.0
#   4.  Gateway + Azure Foundry Cohere route (+ key Secret) + catalog + 3 consumer apps
#   4b. Mistral EU app (Azure Foundry) — 4th app, enabled by default
#   5.  Prometheus + Grafana with the AI-FinOps dashboard
#   6.  Tetragon + shadow-AI rogue workload + shadow-egress refresh
#
# Pinned versions are deliberate (see EG_VERSION / AIGW_VERSION below). The operator
# tag is derived from the chart appVersion (no drifting hardcoded tag).
#
# Usage: ./deploy.sh up | test | grafana | down
# ============================================================================
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AUTO="$(cd "${HERE}/.." && pwd)"
REPO="$(cd "${AUTO}/.." && pwd)"

CLUSTER="${CLUSTER:-greenops}"
CTX="${KCTX:-kind-${CLUSTER}}"
K="kubectl --context ${CTX}"
NS_OP="${NS_OP:-greenops-system}"
RESET_CLUSTER="${RESET_CLUSTER:-false}"
# Image tag is DERIVED from the chart's appVersion so the deployed operator always
# matches this repo (and thus the dashboard's metric names) — never a hardcoded,
# drifting tag again. Override OPERATOR_IMG to pin a specific image.
CHART_APPVER="$(sed -nE 's/^appVersion:[[:space:]]*"?([0-9][0-9.]*)"?.*/\1/p' \
  "${REPO}/charts/ai-sovereign-finops-operator/Chart.yaml" | head -1)"
OPERATOR_IMG="${OPERATOR_IMG:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator:${CHART_APPVER:-dev}}"
# BUILD_OPERATOR=true (default) builds the image from the current source and loads
# it into kind — the most reliable path (no GHCR pull, always matches this repo's
# code). Set BUILD_OPERATOR=false to instead pull/use a prebuilt OPERATOR_IMG.
BUILD_OPERATOR="${BUILD_OPERATOR:-true}"
EG_VERSION="${EG_VERSION:-v1.5.0}"      # IMPORTANT: v1.3.0 is too old (extproc not injected)
AIGW_VERSION="${AIGW_VERSION:-v0.7.0}"
SKIP_PROVIDER_PREFLIGHT="${SKIP_PROVIDER_PREFLIGHT:-false}"
REAL_DEMO_ISOLATE_GITOPS="${REAL_DEMO_ISOLATE_GITOPS:-true}"
ENABLE_MISTRAL_DEMO="${ENABLE_MISTRAL_DEMO:-true}"
ENABLE_TETRAGON="${ENABLE_TETRAGON:-true}"
ENABLE_SHADOW_ROGUE="${ENABLE_SHADOW_ROGUE:-true}"
REQUIRE_SHADOW_EGRESS="${REQUIRE_SHADOW_EGRESS:-true}"
REQUIRE_LATENCY_TELEMETRY="${REQUIRE_LATENCY_TELEMETRY:-true}"
SHADOW_NS="${SHADOW_NS:-default}"
TETRAGON_NS="${TETRAGON_NS:-kube-system}"
SHADOW_WAIT="${SHADOW_WAIT:-20}"
SHADOW_REFRESH_RETRIES="${SHADOW_REFRESH_RETRIES:-6}"
SHADOW_REFRESH_INTERVAL="${SHADOW_REFRESH_INTERVAL:-15}"
EVIDENCE_DIR="${EVIDENCE_DIR:-}"
AUTO_STOP_APPS="${AUTO_STOP_APPS:-false}"
DELETE_CLUSTER_ON_EXIT="${DELETE_CLUSTER_ON_EXIT:-false}"
FOUNDRY_ENDPOINT="${FOUNDRY_ENDPOINT:-https://greenops-foundry.services.ai.azure.com}"
FOUNDRY_HOST="${FOUNDRY_HOST:-greenops-foundry.services.ai.azure.com}"
FOUNDRY_API_VERSION="${FOUNDRY_API_VERSION:-2024-05-01-preview}"
FOUNDRY_PRIMARY_DEPLOYMENT="${FOUNDRY_PRIMARY_DEPLOYMENT:-cohere-command-a-latest}"
MISTRAL_READY="false"

if [ -n "${EVIDENCE_DIR}" ] && [[ "${EVIDENCE_DIR}" != /* ]]; then
  EVIDENCE_DIR="${REPO}/${EVIDENCE_DIR}"
fi

# helm OCI pulls fail when the WSL docker credential helper is used -> anonymous.
export DOCKER_CONFIG="${DOCKER_CONFIG:-/tmp/emptydockercfg}"
mkdir -p "${DOCKER_CONFIG}"; [ -f "${DOCKER_CONFIG}/config.json" ] || echo '{}' > "${DOCKER_CONFIG}/config.json"

step() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 1; }; }

cluster_exists() {
  kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"
}

apply_file() {
  local file="$1" attempt
  for attempt in 1 2 3 4; do
    if ${K} apply --request-timeout=2m -f "${file}"; then
      return 0
    fi
    echo "kubectl apply failed for ${file}; retry ${attempt}/4" >&2
    sleep $((attempt * 5))
  done
  return 1
}

apply_stdin() {
  ${K} apply --request-timeout=2m -f -
}

capture_cmd() {
  local file="$1"
  shift
  mkdir -p "$(dirname "${file}")"
  "$@" >"${file}" 2>&1 || true
}

collect_evidence() {
  local status="${1:-0}" dir pod
  [ -n "${EVIDENCE_DIR}" ] || return 0
  dir="${EVIDENCE_DIR}"
  mkdir -p "${dir}/logs" "${dir}/yaml" "${dir}/metrics"
  {
    echo "status=${status}"
    echo "timestamp=$(date -Is)"
    echo "cluster=${CLUSTER}"
    echo "context=${CTX}"
    echo "operator_image=${OPERATOR_IMG}"
    echo "build_operator=${BUILD_OPERATOR}"
    echo "enable_mistral_demo=${ENABLE_MISTRAL_DEMO}"
    echo "enable_tetragon=${ENABLE_TETRAGON}"
    echo "enable_shadow_rogue=${ENABLE_SHADOW_ROGUE}"
    echo "require_latency_telemetry=${REQUIRE_LATENCY_TELEMETRY}"
  } >"${dir}/run.env"

  if ! cluster_exists; then
    echo "cluster ${CLUSTER} not found; no Kubernetes evidence collected" >"${dir}/NO_CLUSTER.txt"
    return 0
  fi

  step "collect evidence -> ${dir}"
  capture_cmd "${dir}/kubectl_pods.txt" kubectl --context "${CTX}" get pods -A -o wide
  capture_cmd "${dir}/kubectl_deployments.txt" kubectl --context "${CTX}" get deploy -A
  capture_cmd "${dir}/kubectl_services.txt" kubectl --context "${CTX}" get svc -A
  capture_cmd "${dir}/kubectl_events.txt" kubectl --context "${CTX}" get events -A --sort-by=.lastTimestamp
  capture_cmd "${dir}/kubectl_aiops.txt" kubectl --context "${CTX}" get aigw,aiprov,aimodel,aisov,aibudget,aireport -A
  capture_cmd "${dir}/yaml/aiops.yaml" kubectl --context "${CTX}" get aigw,aiprov,aimodel,aisov,aibudget,aireport -A -o yaml
  capture_cmd "${dir}/yaml/aigateway-envoy.yaml" kubectl --context "${CTX}" get gateway,gatewayclass,aigatewayroute,aiservicebackend,backend,backendtlspolicy -A -o yaml
  capture_cmd "${dir}/shadow-egress.json" kubectl --context "${CTX}" -n "${SHADOW_NS}" get configmap shadow-egress -o jsonpath='{.data.egress\.json}'
  capture_cmd "${dir}/logs/operator.log" kubectl --context "${CTX}" -n "${NS_OP}" logs deploy/greenops-ai-sovereign-finops-operator --tail=500
  capture_cmd "${dir}/logs/chatbot-rh.log" kubectl --context "${CTX}" -n rh logs deploy/chatbot-rh --all-containers --tail=200
  capture_cmd "${dir}/logs/risk-assistant.log" kubectl --context "${CTX}" -n finance logs deploy/risk-assistant --all-containers --tail=200
  capture_cmd "${dir}/logs/contract-review.log" kubectl --context "${CTX}" -n legal logs deploy/contract-review --all-containers --tail=200
  capture_cmd "${dir}/logs/content-writer.log" kubectl --context "${CTX}" -n marketing logs deploy/content-writer --all-containers --tail=200
  capture_cmd "${dir}/logs/shadow-ai-rogue.log" kubectl --context "${CTX}" -n finance logs deploy/shadow-ai-rogue --all-containers --tail=200
  capture_cmd "${dir}/logs/envoy-gateway.log" kubectl --context "${CTX}" -n envoy-gateway-system logs deploy/envoy-gateway --tail=300
  capture_cmd "${dir}/logs/aigw-controller.log" kubectl --context "${CTX}" -n envoy-ai-gateway-system logs deploy/ai-gateway-controller --tail=300
  capture_cmd "${dir}/metrics/gateway_metrics.txt" kubectl --context "${CTX}" -n default run metrics-scrape --rm -i --restart=Never --image=curlimages/curl:8.10.1 --quiet -- curl -s -m 20 http://greenops-aigw-metrics.envoy-gateway-system.svc.cluster.local:1064/metrics
  capture_cmd "${dir}/metrics/operator_metrics.txt" kubectl --context "${CTX}" -n default run operator-metrics-scrape --rm -i --restart=Never --image=curlimages/curl:8.10.1 --quiet -- curl -s -m 20 "http://greenops-ai-sovereign-finops-operator-metrics.${NS_OP}.svc.cluster.local:8080/metrics"

  pod="$(kubectl --context "${CTX}" -n "${TETRAGON_NS}" get pods -l app.kubernetes.io/name=tetragon -o name 2>/dev/null | head -1 || true)"
  if [ -n "${pod}" ]; then
    capture_cmd "${dir}/logs/tetragon-export-file.jsonl" kubectl --context "${CTX}" -n "${TETRAGON_NS}" exec "${pod}" -c tetragon -- sh -c "tail -n 2000 /var/run/cilium/tetragon/tetragon.log"
    capture_cmd "${dir}/logs/tetragon-export-stdout.log" kubectl --context "${CTX}" -n "${TETRAGON_NS}" logs "${pod}" -c export-stdout --tail=2000
    capture_cmd "${dir}/logs/tetragon.log" kubectl --context "${CTX}" -n "${TETRAGON_NS}" logs "${pod}" -c tetragon --tail=500
  fi
}

stop_consuming_apps() {
  cluster_exists || return 0
  step "stop consuming apps"
  kubectl --context "${CTX}" -n rh scale deploy/chatbot-rh --replicas=0 >/dev/null 2>&1 || true
  kubectl --context "${CTX}" -n finance scale deploy/risk-assistant --replicas=0 >/dev/null 2>&1 || true
  kubectl --context "${CTX}" -n legal scale deploy/contract-review --replicas=0 >/dev/null 2>&1 || true
  kubectl --context "${CTX}" -n marketing scale deploy/content-writer --replicas=0 >/dev/null 2>&1 || true
  kubectl --context "${CTX}" -n finance scale deploy/shadow-ai-rogue --replicas=0 >/dev/null 2>&1 || true
}

verify_cleanup() {
  local status=$?
  trap - EXIT
  collect_evidence "${status}"
  if [ "${AUTO_STOP_APPS}" = "true" ]; then
    stop_consuming_apps
  fi
  if [ "${DELETE_CLUSTER_ON_EXIT}" = "true" ]; then
    step "delete kind cluster '${CLUSTER}'"
    kind delete cluster --name "${CLUSTER}" >/dev/null 2>&1 || true
  fi
  exit "${status}"
}

load_foundry_key() {
  local path
  for path in "${REPO}/docs/foundrykey.txt" "${REPO}/docs/mistralkey.txt"; do
    [ -f "${path}" ] || continue
    tr -d '\r\n' < "${path}"
    return 0
  done
  echo "missing docs/foundrykey.txt (preferred) or docs/mistralkey.txt" >&2
  return 1
}

probe_foundry() {
  local key body code
  key="$(load_foundry_key)" || return 1
  [ -n "${key}" ] || { echo "empty Foundry key" >&2; return 1; }
  body="$(mktemp)"
  code="$(
    curl -sS -o "${body}" -w '%{http_code}' \
      "${FOUNDRY_ENDPOINT}/openai/deployments/${FOUNDRY_PRIMARY_DEPLOYMENT}/chat/completions?api-version=${FOUNDRY_API_VERSION}" \
      -H "Authorization: Bearer ${key}" \
      -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"Reply with exactly: ok"}],"max_tokens":8}'
  )"
  if [ "${code}" = "200" ]; then
    rm -f "${body}"
    return 0
  fi
  echo "Azure Foundry Cohere preflight failed (HTTP ${code})." >&2
  sed -n '1,20p' "${body}" >&2
  rm -f "${body}"
  return 1
}

probe_mistral() {
  local key body code
  key="$(load_foundry_key)" || return 1
  [ -n "${key}" ] || return 1
  body="$(mktemp)"
  code="$(
    curl -sS -o "${body}" -w '%{http_code}' \
      "${FOUNDRY_ENDPOINT}/openai/deployments/mistral-large-latest/chat/completions?api-version=${FOUNDRY_API_VERSION}" \
      -H "Authorization: Bearer ${key}" \
      -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"ping"}],"max_tokens":1}'
  )"
  if [ "${code}" = "200" ]; then
    rm -f "${body}"
    return 0
  fi
  echo "Mistral Foundry preflight failed (HTTP ${code}); skipping the optional EU app." >&2
  sed -n '1,20p' "${body}" >&2
  rm -f "${body}"
  return 1
}

preflight() {
  for t in kubectl helm kind docker curl; do need "$t"; done
  if [ "${ENABLE_TETRAGON}" = "true" ]; then
    need python3
  fi
  if [ "${SKIP_PROVIDER_PREFLIGHT}" = "true" ]; then
    echo "provider preflight skipped (SKIP_PROVIDER_PREFLIGHT=true)"
    return 0
  fi
  step "0/5 provider preflight"
  probe_foundry || exit 1
  if [ "${ENABLE_MISTRAL_DEMO}" = "true" ]; then
    if probe_mistral; then
      MISTRAL_READY="true"
      echo "Mistral Foundry probe OK."
    else
      MISTRAL_READY="false"
    fi
  fi
}

ensure_cluster() {
  step "1/5 kind cluster '${CLUSTER}'"
  if [ "${RESET_CLUSTER}" = "true" ]; then
    echo "RESET_CLUSTER=true: deleting any existing kind cluster '${CLUSTER}' first."
    kind delete cluster --name "${CLUSTER}" >/dev/null 2>&1 || true
  fi
  if cluster_exists; then
    echo "cluster exists."
  else
    kind create cluster --name "${CLUSTER}" --config "${AUTO}/kind/kind-config.yaml" 2>/dev/null \
      || kind create cluster --name "${CLUSTER}"
  fi
  ${K} cluster-info >/dev/null
}

isolate_existing_gitops() {
  step "1b/6 isolate real-demo from stale GitOps state"
  if [ "${REAL_DEMO_ISOLATE_GITOPS}" != "true" ]; then
    echo "GitOps isolation disabled (REAL_DEMO_ISOLATE_GITOPS=false)."
    return 0
  fi

  if ${K} api-resources --api-group=argoproj.io 2>/dev/null | awk '{print $1}' | grep -qx applications; then
    for app in greenops-operator greenops-samples; do
      if ${K} -n argocd get application "${app}" >/dev/null 2>&1; then
        ${K} -n argocd patch application "${app}" --type merge \
          -p '{"spec":{"syncPolicy":null}}' >/dev/null || true
        echo "disabled ArgoCD auto-sync for ${app}."
      fi
    done
  fi

  # Older GitOps demos used release name greenops-operator. Keep only the
  # Helm-managed real-demo release active, otherwise two managers race on status.
  ${K} -n "${NS_OP}" scale deployment/greenops-operator-ai-sovereign-finops-operator \
    --replicas=0 >/dev/null 2>&1 || true
}

ensure_operator_image() {
  step "2a/6 operator image ${OPERATOR_IMG} (build=${BUILD_OPERATOR})"
  # The GHCR package is private, so a fresh kind node cannot pull it; we therefore
  # make the image available locally and load it into kind (pullPolicy:IfNotPresent
  # then finds it without any registry access). Build from source by default so the
  # operator always matches this repo's metrics/dashboard.
  if [ "${BUILD_OPERATOR}" = "true" ]; then
    DOCKER_CONFIG="${HOME}/.docker" docker build -t "${OPERATOR_IMG}" "${REPO}"
  elif ! docker image inspect "${OPERATOR_IMG}" >/dev/null 2>&1; then
    DOCKER_CONFIG="${HOME}/.docker" docker pull "${OPERATOR_IMG}" \
      || { echo "cannot pull ${OPERATOR_IMG}; 'docker login ghcr.io' or set BUILD_OPERATOR=true" >&2; exit 1; }
  fi
  kind load docker-image "${OPERATOR_IMG}" --name "${CLUSTER}"
}

ensure_operator() {
  step "2/6 operator (Helm, ${OPERATOR_IMG})"
  # Helm does not upgrade CRDs from charts/ on `upgrade`; apply them explicitly so
  # reusing an existing kind cluster still picks up new API fields.
  ${K} apply -f "${REPO}/charts/ai-sovereign-finops-operator/crds" >/dev/null
  helm --kube-context "${CTX}" upgrade --install greenops "${REPO}/charts/ai-sovereign-finops-operator" \
    --namespace "${NS_OP}" --create-namespace \
    --set image.repository="${OPERATOR_IMG%:*}" --set image.tag="${OPERATOR_IMG##*:}" \
    --set image.pullPolicy=IfNotPresent
  ${K} -n "${NS_OP}" rollout status deploy/greenops-ai-sovereign-finops-operator --timeout=180s
}

cleanup_legacy_aiops_state() {
  step "2b/6 clean stale samples before applying real-demo catalog"
  if [ "${REAL_DEMO_ISOLATE_GITOPS}" != "true" ]; then
    echo "Legacy sample cleanup disabled with REAL_DEMO_ISOLATE_GITOPS=false."
    return 0
  fi

  ${K} delete -k "${REPO}/config/samples" --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${AUTO}/demo/demo-extra.yaml" --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete \
    aigatewayroute.aigateway.envoyproxy.io/greenops-openai \
    aiservicebackend.aigateway.envoyproxy.io/greenops-openai \
    backendsecuritypolicy.aigateway.envoyproxy.io/greenops-openai-apikey \
    backend.gateway.envoyproxy.io/greenops-openai \
    backendtlspolicy.gateway.networking.k8s.io/greenops-openai-tls \
    aiproviders.aiops.imperium.io/openai-us \
    aiproviders.aiops.imperium.io/openai-us-mini \
    aimodels.aiops.imperium.io/gpt-4o \
    aimodels.aiops.imperium.io/gpt-4o-mini \
    --ignore-not-found >/dev/null 2>&1 || true
}

ensure_envoy() {
  step "3/5 Envoy Gateway ${EG_VERSION} + Envoy AI Gateway ${AIGW_VERSION}"
  curl -fsSL -o /tmp/eg-values.yaml \
    "https://raw.githubusercontent.com/envoyproxy/ai-gateway/${AIGW_VERSION}/manifests/envoy-gateway-values.yaml"
  helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm --version "${EG_VERSION}" \
    -f /tmp/eg-values.yaml -n envoy-gateway-system --create-namespace
  ${K} -n envoy-gateway-system rollout status deploy/envoy-gateway --timeout=180s
  helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm --version "${AIGW_VERSION}" \
    -n envoy-ai-gateway-system --create-namespace
  # metricsRequestHeaderAttributes: map request headers -> metric labels so token
  # usage is attributed PER NAMESPACE/APP (works even when two apps in different
  # namespaces use the same model).
  helm upgrade -i aieg oci://docker.io/envoyproxy/ai-gateway-helm --version "${AIGW_VERSION}" \
    -n envoy-ai-gateway-system --create-namespace \
    --set "controller.metricsRequestHeaderAttributes=x-greenops-namespace:k8s.namespace\,x-greenops-app:k8s.app"
  ${K} wait --timeout=180s -n envoy-ai-gateway-system deployment/ai-gateway-controller --for=condition=Available
}

deploy_gateway_and_apps() {
  step "4/5 Gateway + Cohere Foundry route + catalog + consumer apps"
  local fkey
  fkey="$(load_foundry_key)" || exit 1
  [ -n "${fkey}" ] || { echo "no Foundry key found" >&2; exit 1; }
  ${K} -n default create secret generic greenops-foundry-apikey \
    --from-literal=apiKey="${fkey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/01-gateway-cohere.yaml"
  ${K} wait pods --timeout=180s -l gateway.envoyproxy.io/owning-gateway-name=greenops-aigw \
    -n envoy-gateway-system --for=condition=Ready
  apply_file "${HERE}/02-metrics-and-catalog.yaml"
  apply_file "${HERE}/03-consumer-apps.yaml"
}

deploy_mistral_eu() {
  # 2nd provider: Mistral on Azure AI Foundry (EU zone), enabled by default so
  # the real demo always has four app flows when the Foundry deployment is ready.
  step "4b/6 Mistral EU app (sovereignty zone test)"
  if [ "${ENABLE_MISTRAL_DEMO}" != "true" ]; then
    echo "Mistral demo disabled (ENABLE_MISTRAL_DEMO=false) - ensuring it is absent."
    ${K} delete -f "${HERE}/05-mistral-eu.yaml" --ignore-not-found >/dev/null 2>&1 || true
    return 0
  fi
  if [ "${MISTRAL_READY}" != "true" ]; then
    echo "Mistral Foundry not preflight-ready — skipping the optional EU app."
    echo "  get the key: az cognitiveservices account keys list -n greenops-foundry -g greenops-rg --query key1 -o tsv > docs/foundrykey.txt"
    return 0
  fi
  local mkey
  mkey="$(load_foundry_key)" || exit 1
  ${K} -n default create secret generic greenops-mistral-apikey \
    --from-literal=apiKey="${mkey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/05-mistral-eu.yaml"
}

wait_consumer_apps() {
  step "4c/6 wait for the four verification apps"
  ${K} -n rh rollout status deploy/chatbot-rh --timeout=180s
  ${K} -n finance rollout status deploy/risk-assistant --timeout=180s
  ${K} -n legal rollout status deploy/contract-review --timeout=180s
  if [ "${ENABLE_MISTRAL_DEMO}" = "true" ] && [ "${MISTRAL_READY}" = "true" ]; then
    ${K} -n marketing rollout status deploy/content-writer --timeout=180s
  fi
}

deploy_observability() {
  step "5/5 Prometheus + Grafana (dashboard)"
  ${K} -n "${NS_OP}" create configmap demo-grafana-dashboard \
    --from-file=ai-finops-overview.json="${REPO}/dashboards/ai-finops-overview.json" \
    --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} label cm demo-grafana-dashboard -n "${NS_OP}" aiops.imperium.io/demo=true --overwrite >/dev/null
  apply_file "${AUTO}/demo/observability.yaml"
  ${K} -n "${NS_OP}" rollout status deploy/demo-prometheus --timeout=120s
  ${K} -n "${NS_OP}" rollout status deploy/demo-grafana --timeout=120s
}

ensure_tetragon() {
  step "6/6 Tetragon shadow-AI plane"
  if [ "${ENABLE_TETRAGON}" != "true" ]; then
    echo "Tetragon disabled (ENABLE_TETRAGON=false)."
    return 0
  fi
  KCTX="${CTX}" TETRAGON_NS="${TETRAGON_NS}" "${AUTO}/tetragon/install.sh"
}

deploy_shadow_ai() {
  local i shadow_json
  step "6b/6 Shadow-AI rogue workload"
  if [ "${ENABLE_TETRAGON}" != "true" ]; then
    echo "Skipping shadow-AI workload because Tetragon is disabled."
    return 0
  fi
  if [ "${ENABLE_SHADOW_ROGUE}" != "true" ]; then
    echo "Shadow-AI rogue workload disabled (ENABLE_SHADOW_ROGUE=false) — ensuring it is absent."
    ${K} -n finance delete deploy/shadow-ai-rogue --ignore-not-found >/dev/null 2>&1 || true
    ${K} -n "${SHADOW_NS}" delete configmap shadow-egress --ignore-not-found >/dev/null 2>&1 || true
    return 0
  fi

  apply_file "${AUTO}/tetragon/rogue-app.yaml"
  ${K} -n finance rollout status deploy/shadow-ai-rogue --timeout=120s

  shadow_json="[]"
  for i in $(seq 1 "${SHADOW_REFRESH_RETRIES}"); do
    if [ "${i}" -eq 1 ]; then
      echo "waiting ${SHADOW_WAIT}s for Tetragon to export fresh events"
      sleep "${SHADOW_WAIT}"
    else
      echo "shadow-egress still empty after refresh $((i - 1))/${SHADOW_REFRESH_RETRIES}; retrying in ${SHADOW_REFRESH_INTERVAL}s"
      sleep "${SHADOW_REFRESH_INTERVAL}"
    fi
    NS="${SHADOW_NS}" KCTX="${CTX}" TETRAGON_NS="${TETRAGON_NS}" "${AUTO}/tetragon/forwarder.sh"
    shadow_json="$(${K} -n "${SHADOW_NS}" get configmap shadow-egress -o jsonpath='{.data.egress\.json}' 2>/dev/null || echo '[]')"
    if [ -n "${shadow_json}" ] && [ "${shadow_json}" != "[]" ]; then
      echo "shadow-egress populated: ${shadow_json}"
      break
    fi
  done

  ${K} -n "${SHADOW_NS}" annotate aisovereigntypolicy regulated-france-policy \
    demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true

  if [ -z "${shadow_json}" ] || [ "${shadow_json}" = "[]" ]; then
    if [ "${REQUIRE_SHADOW_EGRESS}" = "true" ]; then
      echo "shadow-egress is empty after ${SHADOW_REFRESH_RETRIES} refresh attempts." >&2
      return 1
    fi
    echo "WARNING: Tetragon is installed but shadow-egress is still empty on this platform."
    echo "         The Grafana Shadow-AI panels stay empty until real egress is captured."
  fi
}

refresh_operator_status() {
  step "6c/6 refresh operator status from live traffic"
  sleep "${STATUS_REFRESH_WAIT:-45}"
  ${K} -n default annotate aifinopsreport ai-report-all \
    demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
  ${K} -n default annotate aisovereigntypolicy regulated-france-policy \
    demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
  for budget in finance-budget rh-budget legal-budget marketing-budget; do
    ${K} -n default annotate aibudgetpolicy "${budget}" \
      demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
  done
  sleep 10
  ${K} get aigw,aiprov,aimodel,aisov,aibudget,aireport -A
}

scrape_gateway_metrics() {
  ${K} -n default run "gateway-metrics-check-$(date +%s%N)" --rm -i --restart=Never \
    --image=curlimages/curl:8.10.1 --quiet -- \
    curl -s -m 20 http://greenops-aigw-metrics.envoy-gateway-system.svc.cluster.local:1064/metrics
}

scrape_operator_metrics() {
  ${K} -n default run "operator-metrics-check-$(date +%s%N)" --rm -i --restart=Never \
    --image=curlimages/curl:8.10.1 --quiet -- \
    curl -s -m 20 "http://greenops-ai-sovereign-finops-operator-metrics.${NS_OP}.svc.cluster.local:8080/metrics"
}

validate_latency_telemetry() {
  local available score latencies gateway_metrics operator_metrics metric
  step "6d/6 validate real latency telemetry"
  if [ "${REQUIRE_LATENCY_TELEMETRY}" != "true" ]; then
    echo "Latency telemetry validation skipped (REQUIRE_LATENCY_TELEMETRY=false)."
    return 0
  fi

  gateway_metrics="$(scrape_gateway_metrics || true)"
  if ! printf '%s\n' "${gateway_metrics}" | grep -q '^gen_ai_server_request_duration_seconds_'; then
    echo "Envoy AI Gateway did not expose gen_ai_server_request_duration_seconds_*; refusing to claim measured latency." >&2
    return 1
  fi

  available="$(${K} -n default get aifinopsreport ai-report-all -o jsonpath='{.status.latencyTelemetryAvailable}' 2>/dev/null || true)"
  if [ "${available}" != "true" ]; then
    echo "AIFinOpsReport status latencyTelemetryAvailable=${available:-<empty>}; expected true for the real AIGW demo." >&2
    ${K} -n default get aifinopsreport ai-report-all -o yaml >&2 || true
    return 1
  fi

  score="$(${K} -n default get aifinopsreport ai-report-all -o jsonpath='{.status.routingScores[0].score}' 2>/dev/null || true)"
  if [ -z "${score}" ]; then
    echo "AIFinOpsReport has no routingScores; latency score cannot be verified." >&2
    ${K} -n default get aifinopsreport ai-report-all -o yaml >&2 || true
    return 1
  fi

  latencies="$(${K} -n default get aifinopsreport ai-report-all -o jsonpath='{.status.routingScores[*].observedLatencyMillis}' 2>/dev/null || true)"
  if ! printf '%s\n' "${latencies}" | grep -Eq '(^| )[1-9][0-9]*(\.[0-9]+)?($| )'; then
    echo "No positive observedLatencyMillis found in routingScores: ${latencies:-<empty>}." >&2
    return 1
  fi

  operator_metrics="$(scrape_operator_metrics || true)"
  for metric in ai_finops_latency_score ai_finops_measured_latency_millis ai_finops_latency_telemetry_available ai_finops_routing_score; do
    if ! printf '%s\n' "${operator_metrics}" | grep -q "^${metric}"; then
      echo "Operator /metrics does not expose ${metric}; dashboard latency panel would have no real source." >&2
      return 1
    fi
  done
  echo "Latency telemetry OK: gateway duration histogram observed, report status true, routing score=${score}."
}

up() {
  preflight; ensure_cluster; isolate_existing_gitops; ensure_operator_image; ensure_operator; cleanup_legacy_aiops_state; ensure_envoy
  deploy_gateway_and_apps; deploy_mistral_eu; wait_consumer_apps; deploy_observability; ensure_tetragon; deploy_shadow_ai; refresh_operator_status; validate_latency_telemetry
  step "Done — real demo is live"
  echo "Apps in ns rh/finance/legal call Azure Foundry Cohere; marketing calls Mistral EU."
  echo "The operator reads real token usage and computes cost/sovereignty/budget per app."
  echo "Budgets stay fully real, but no fake cheaper-model fallback is configured while only one"
  echo "non-French live deployment exists in this Foundry account."
  echo "Shadow-AI is enabled: finance/shadow-ai-rogue bypasses the gateway and Tetragon feeds shadow-egress."
  echo
  echo "Open Grafana:  ./deploy.sh grafana   (then http://localhost:3000)"
  echo "Watch cost:    ${K} -n default get aifinopsreport ai-report-all -o yaml | grep -A4 'status:'"
  echo "Let it run a few minutes so cost/tokens accumulate to meaningful values."
}

verify() {
  AUTO_STOP_APPS="${VERIFY_AUTO_STOP_APPS:-true}"
  DELETE_CLUSTER_ON_EXIT="${VERIFY_DELETE_CLUSTER_ON_EXIT:-true}"
  if [ -z "${EVIDENCE_DIR}" ]; then
    EVIDENCE_DIR="${REPO}/experimentation/results-live-kind-$(date +%Y%m%d-%H%M%S)"
  fi
  export AUTO_STOP_APPS DELETE_CLUSTER_ON_EXIT EVIDENCE_DIR
  trap verify_cleanup EXIT
  up
}

grafana() {
  echo "Grafana → http://localhost:3000   (anonymous admin)"
  ${K} -n "${NS_OP}" port-forward svc/demo-grafana 3000:3000
}

test_call() {
  local svc ep
  svc="$(${K} get svc -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-name=greenops-aigw -o jsonpath='{.items[0].metadata.name}')"
  ep="http://${svc}.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions"
  ${K} -n default run aigw-test --rm -i --restart=Never --image=curlimages/curl:8.10.1 --quiet -- \
    sh -c "curl -s -m 60 -H 'Content-Type: application/json' -d '{\"model\":\"cohere-command-a-latest\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly: ok\"}],\"max_tokens\":20}' ${ep}"
}

down() {
  step "Removing the real demo"
  ${K} -n finance delete deploy/shadow-ai-rogue --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n "${SHADOW_NS}" delete configmap shadow-egress --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${HERE}/05-mistral-eu.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/03-consumer-apps.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/02-metrics-and-catalog.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/01-gateway-cohere.yaml" --ignore-not-found
  ${K} -n default delete \
    aigatewayroute.aigateway.envoyproxy.io/greenops-openai \
    aiservicebackend.aigateway.envoyproxy.io/greenops-openai \
    backendsecuritypolicy.aigateway.envoyproxy.io/greenops-openai-apikey \
    backend.gateway.envoyproxy.io/greenops-openai \
    backendtlspolicy.gateway.networking.k8s.io/greenops-openai-tls \
    aiproviders.aiops.imperium.io/openai-us \
    aiproviders.aiops.imperium.io/openai-us-mini \
    aimodels.aiops.imperium.io/gpt-4o \
    aimodels.aiops.imperium.io/gpt-4o-mini \
    --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete secret greenops-foundry-apikey greenops-openai-apikey greenops-mistral-apikey --ignore-not-found
  ${K} -n "${NS_OP}" delete deploy,svc,configmap -l aiops.imperium.io/demo=true --ignore-not-found
  echo "Operator + Envoy control planes kept (helm uninstall greenops / eg / aieg to remove)."
}

case "${1:-up}" in
  up) up ;;
  verify) verify ;;
  grafana) grafana ;;
  test) test_call ;;
  down) down ;;
  *) echo "usage: $0 [up|verify|grafana|test|down]" >&2; exit 1 ;;
esac
