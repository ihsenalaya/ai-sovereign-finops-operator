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
#   4c. optional third compliant QualityScore provider: GPT-4.1 Mini on Foundry
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
OPERATOR="${REPO}/operateur"

CLUSTER="${CLUSTER:-greenops}"
CTX="${KCTX:-kind-${CLUSTER}}"
K="kubectl --context ${CTX}"
NS_OP="${NS_OP:-greenops-system}"
RESET_CLUSTER="${RESET_CLUSTER:-false}"
# Image tag is DERIVED from the chart's appVersion so the deployed operator always
# matches this repo (and thus the dashboard's metric names) — never a hardcoded,
# drifting tag again. Override OPERATOR_IMG to pin a specific image.
CHART_APPVER="$(sed -nE 's/^appVersion:[[:space:]]*"?([0-9][0-9.]*)"?.*/\1/p' \
  "${OPERATOR}/charts/ai-sovereign-finops-operator/Chart.yaml" | head -1)"
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
ENABLE_THIRD_QUALITY_PROVIDER="${ENABLE_THIRD_QUALITY_PROVIDER:-${ENABLE_MISTRAL_SMALL_PROVIDER:-true}}"
REQUIRE_THIRD_QUALITY_PROVIDER="${REQUIRE_THIRD_QUALITY_PROVIDER:-true}"
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
THIRD_QUALITY_DEPLOYMENT="${THIRD_QUALITY_DEPLOYMENT:-gpt-foundry-eu-mini}"
THIRD_QUALITY_PROVIDER_FILE="${HERE}/05b-openai-foundry-eu.yaml"
MISTRAL_READY="false"
THIRD_QUALITY_READY="false"

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
    echo "enable_third_quality_provider=${ENABLE_THIRD_QUALITY_PROVIDER}"
    echo "require_third_quality_provider=${REQUIRE_THIRD_QUALITY_PROVIDER}"
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
  capture_cmd "${dir}/kubectl_aiops.txt" kubectl --context "${CTX}" get aigw,aiprov,aimodel,aiqgate,aisov,aibudget,aireport -A
  capture_cmd "${dir}/kubectl_quality_jobs.txt" kubectl --context "${CTX}" get jobs,pods -A -l aiops.imperium.io/quality-evaluator=true -o wide
  capture_cmd "${dir}/yaml/aiops.yaml" kubectl --context "${CTX}" get aigw,aiprov,aimodel,aiqgate,aisov,aibudget,aireport -A -o yaml
  capture_cmd "${dir}/yaml/quality-jobs.yaml" kubectl --context "${CTX}" get jobs,pods -A -l aiops.imperium.io/quality-evaluator=true -o yaml
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
  capture_cmd "${dir}/logs/quality-eval-pods.log" kubectl --context "${CTX}" -n default logs -l aiops.imperium.io/quality-evaluator=true --all-containers --tail=300 --prefix
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
  for path in "${OPERATOR}/docs/foundrykey.txt" "${OPERATOR}/docs/mistralkey.txt"; do
    [ -f "${path}" ] || continue
    tr -d '\r\n' < "${path}"
    return 0
  done
  echo "missing operateur/docs/foundrykey.txt (preferred) or operateur/docs/mistralkey.txt" >&2
  return 1
}

probe_foundry_deployment() {
  local deploy="$1" label="$2" key body code
  key="$(load_foundry_key)" || return 1
  [ -n "${key}" ] || { echo "empty Foundry key" >&2; return 1; }
  body="$(mktemp)"
  code="$(
    curl -sS -o "${body}" -w '%{http_code}' \
      "${FOUNDRY_ENDPOINT}/openai/deployments/${deploy}/chat/completions?api-version=${FOUNDRY_API_VERSION}" \
      -H "Authorization: Bearer ${key}" \
      -H 'Content-Type: application/json' \
      -d '{"messages":[{"role":"user","content":"Reply with exactly: ok"}],"max_tokens":8}'
  )"
  if [ "${code}" = "200" ]; then
    rm -f "${body}"
    return 0
  fi
  echo "${label} Foundry preflight failed for deployment ${deploy} (HTTP ${code})." >&2
  sed -n '1,20p' "${body}" >&2
  rm -f "${body}"
  return 1
}

probe_foundry() {
  probe_foundry_deployment "${FOUNDRY_PRIMARY_DEPLOYMENT}" "Azure Foundry Cohere"
}

probe_mistral() {
  probe_foundry_deployment "mistral-large-latest" "Mistral Large"
}

probe_third_quality_provider() {
  probe_foundry_deployment "${THIRD_QUALITY_DEPLOYMENT}" "Third QualityScore Foundry provider"
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
      echo "Mistral Large is required by the demo quality gates; aborting before applying partial state." >&2
      return 1
    fi
  fi
  if [ "${ENABLE_THIRD_QUALITY_PROVIDER}" = "true" ]; then
    if probe_third_quality_provider; then
      THIRD_QUALITY_READY="true"
      echo "Third QualityScore Foundry provider probe OK (${THIRD_QUALITY_DEPLOYMENT})."
    else
      THIRD_QUALITY_READY="false"
      if [ "${REQUIRE_THIRD_QUALITY_PROVIDER}" = "true" ]; then
        echo "Third QualityScore provider is required, but ${THIRD_QUALITY_DEPLOYMENT} is not reachable." >&2
        echo "Provision it as an Azure AI Foundry DataZoneStandard deployment, or rerun with REQUIRE_THIRD_QUALITY_PROVIDER=false." >&2
        return 1
      fi
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
    DOCKER_CONFIG="${HOME}/.docker" docker build -t "${OPERATOR_IMG}" "${OPERATOR}"
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
  ${K} apply -f "${OPERATOR}/charts/ai-sovereign-finops-operator/crds" >/dev/null
  helm --kube-context "${CTX}" upgrade --install greenops "${OPERATOR}/charts/ai-sovereign-finops-operator" \
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

  ${K} delete -k "${OPERATOR}/config/samples" --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${HERE}/08-quality-gates.yaml" --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete aiqualitygate finance-risk-assistant-mistral-small-quality --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${THIRD_QUALITY_PROVIDER_FILE}" --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${HERE}/05b-mistral-small-eu.yaml" --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete jobs -l aiops.imperium.io/quality-evaluator=true --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete configmap \
    finance-quality-evidence finance-mistral-small-quality-evidence rh-quality-evidence legal-quality-evidence marketing-quality-evidence \
    --ignore-not-found >/dev/null 2>&1 || true
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
  step "4/5 Gateway + routage + catalog (sans consumer apps — voir étapes 4b/4c/4d)"
  # Créer le gateway, les routes de base et le catalogue de souveraineté.
  # Les consumer apps arrivent dans les étapes suivantes avec leur provider respectif.
  local fkey
  fkey="$(load_foundry_key)" || exit 1
  [ -n "${fkey}" ] || { echo "no Foundry key found" >&2; exit 1; }
  ${K} -n default create secret generic greenops-foundry-apikey \
    --from-literal=apiKey="${fkey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/01-gateway-cohere.yaml"
  ${K} wait pods --timeout=180s -l gateway.envoyproxy.io/owning-gateway-name=greenops-aigw \
    -n envoy-gateway-system --for=condition=Ready
  apply_file "${HERE}/02-metrics-and-catalog.yaml"
}

deploy_openai_fr() {
  # Provider France — Azure OpenAI France Central (greenops-fr-ec0e82-06181356.openai.azure.com)
  # Apps: rh/chatbot-rh + legal/contract-review — zone FR → conforme
  step "4b/6 OpenAI France (souverain FR) — rh + legal"
  local fr_key_file="${OPERATOR}/docs/openai-fr-key.txt"
  if [ ! -f "${fr_key_file}" ]; then
    # Récupération automatique via az CLI si disponible
    if command -v az >/dev/null 2>&1; then
      az cognitiveservices account keys list \
        --name greenops-fr-ec0e82-06181356 --resource-group greenops-rg \
        --query key1 -o tsv > "${fr_key_file}" 2>/dev/null || true
    fi
  fi
  if [ ! -f "${fr_key_file}" ] || [ ! -s "${fr_key_file}" ]; then
    echo "Clé Azure OpenAI France manquante — skip."
    echo "  az cognitiveservices account keys list -n greenops-fr-ec0e82-06181356 -g greenops-rg --query key1 -o tsv > operateur/docs/openai-fr-key.txt"
    return 0
  fi
  local frkey
  frkey="$(tr -d '[:space:]' < "${fr_key_file}")"
  ${K} -n default create secret generic greenops-openai-fr-apikey \
    --from-literal=apiKey="${frkey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/06-openai-fr.yaml"
  echo "OpenAI France déployé (rh/chatbot-rh + legal/contract-review → gpt-france-mini)"
}

deploy_openai_us() {
  # Provider US — Azure OpenAI US East (greenops-us-ec0e82-06181356.openai.azure.com)
  # App: finance/risk-assistant — zone US → violation de souveraineté
  step "4c/6 OpenAI US (non-souverain US) — finance"
  local us_key_file="${OPERATOR}/docs/openai-us-key.txt"
  if [ ! -f "${us_key_file}" ]; then
    if command -v az >/dev/null 2>&1; then
      az cognitiveservices account keys list \
        --name greenops-us-ec0e82-06181356 --resource-group greenops-rg \
        --query key1 -o tsv > "${us_key_file}" 2>/dev/null || true
    fi
  fi
  if [ ! -f "${us_key_file}" ] || [ ! -s "${us_key_file}" ]; then
    echo "Clé Azure OpenAI US manquante — skip."
    echo "  az cognitiveservices account keys list -n greenops-us-ec0e82-06181356 -g greenops-rg --query key1 -o tsv > operateur/docs/openai-us-key.txt"
    return 0
  fi
  local uskey
  uskey="$(tr -d '[:space:]' < "${us_key_file}")"
  ${K} -n default create secret generic greenops-openai-us-apikey \
    --from-literal=apiKey="${uskey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/07-openai-us.yaml"
  echo "OpenAI US déployé (finance/risk-assistant → gpt-us-mini)"
}

deploy_mistral_eu() {
  # Provider EU — Mistral sur Azure AI Foundry (greenops-foundry.services.ai.azure.com)
  # App: marketing/content-writer — zone EU → conforme
  step "4d/6 Mistral EU (souverain EU) — marketing"
  if [ "${ENABLE_MISTRAL_DEMO}" != "true" ]; then
    echo "Mistral demo disabled (ENABLE_MISTRAL_DEMO=false)."
    ${K} delete -f "${HERE}/05-mistral-eu.yaml" --ignore-not-found >/dev/null 2>&1 || true
    return 0
  fi
  if [ "${MISTRAL_READY}" != "true" ]; then
    echo "Mistral Foundry pas disponible — skip."
    return 0
  fi
  local mkey
  mkey="$(load_foundry_key)" || exit 1
  ${K} -n default create secret generic greenops-mistral-apikey \
    --from-literal=apiKey="${mkey}" --dry-run=client -o yaml | apply_stdin >/dev/null
  apply_file "${HERE}/05-mistral-eu.yaml"
}

deploy_third_quality_provider() {
  # Optional provider EU — GPT-4.1 Mini on Azure AI Foundry.
  # It exists only to give the QualityScore radar a third real compliant provider
  # polygon; no consumer app is needed because the AIQualityGate job calls it.
  step "4d2/6 Foundry OpenAI EU (third QualityScore provider)"
  if [ "${ENABLE_THIRD_QUALITY_PROVIDER}" != "true" ]; then
    echo "Third QualityScore provider disabled (ENABLE_THIRD_QUALITY_PROVIDER=false)."
    ${K} delete -f "${THIRD_QUALITY_PROVIDER_FILE}" --ignore-not-found >/dev/null 2>&1 || true
    return 0
  fi
  if [ "${THIRD_QUALITY_READY}" != "true" ]; then
    echo "Third QualityScore Foundry deployment ${THIRD_QUALITY_DEPLOYMENT} is not available — skip."
    return 0
  fi
  apply_file "${THIRD_QUALITY_PROVIDER_FILE}"
}

wait_consumer_apps() {
  step "4e/6 wait for all consumer apps"
  for ns_app in "rh/chatbot-rh" "finance/risk-assistant" "legal/contract-review"; do
    ns="${ns_app%%/*}"; app="${ns_app##*/}"
    kubectl --context "${CTX}" -n "${ns}" get deploy "${app}" >/dev/null 2>&1 && \
      ${K} -n "${ns}" rollout status "deploy/${app}" --timeout=180s || true
  done
  if [ "${ENABLE_MISTRAL_DEMO}" = "true" ] && [ "${MISTRAL_READY}" = "true" ]; then
    ${K} -n marketing rollout status deploy/content-writer --timeout=180s
  fi
}

deploy_quality_gates() {
  local i jobs
  step "4f/6 AI Quality Score gates (real gateway evaluation jobs)"
  if [ "${ENABLE_THIRD_QUALITY_PROVIDER}" = "true" ] && [ "${THIRD_QUALITY_READY}" = "true" ]; then
    local tmp
    tmp="$(mktemp)"
    awk -v model="${THIRD_QUALITY_DEPLOYMENT}" '
      /name: marketing-content-quality/ { in_marketing=1 }
      in_marketing && /candidateModel: mistral-large-latest/ {
        sub("mistral-large-latest", model)
        in_marketing=0
      }
      { print }
    ' "${HERE}/08-quality-gates.yaml" >"${tmp}"
    echo "+ kubectl apply -f ${HERE}/08-quality-gates.yaml (marketing candidate=${THIRD_QUALITY_DEPLOYMENT})"
    ${K} apply -f "${tmp}" >/dev/null
    rm -f "${tmp}"
  else
    apply_file "${HERE}/08-quality-gates.yaml"
  fi

  jobs=""
  for i in $(seq 1 24); do
    ${K} -n default annotate aiqualitygate --all demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
    jobs="$(${K} -n default get jobs -l aiops.imperium.io/quality-evaluator=true --no-headers 2>/dev/null || true)"
    if [ -n "${jobs}" ]; then
      break
    fi
    sleep 5
  done
  if [ -z "${jobs}" ]; then
    echo "AIQualityGate did not create any quality evaluation Job." >&2
    ${K} -n default get aiqualitygate -o yaml >&2 || true
    return 1
  fi

  ${K} -n default wait --for=condition=complete --timeout=300s \
    jobs -l aiops.imperium.io/quality-evaluator=true || {
      echo "Quality evaluation Jobs did not complete." >&2
      ${K} -n default get jobs,pods -l aiops.imperium.io/quality-evaluator=true -o wide >&2 || true
      return 1
    }
  echo "Quality evaluation Jobs completed."
}

build_grafana_image() {
  # Build the custom Grafana image with volkovlabs-echarts-panel pre-installed (radar chart).
  # Required because Kind pods have no internet access on startup.
  local img="grafana-radar:11.2.2-echarts6.6.0"
  local dockerfile="${AUTO}/demo/Dockerfile.grafana"
  if docker image inspect "${img}" >/dev/null 2>&1; then
    echo "Grafana radar image ${img} already built — skipping."
  else
    step "Building Grafana image with volkovlabs-echarts-panel (radar)"
    DOCKER_BUILDKIT=0 docker build -t "${img}" -f "${dockerfile}" "$(dirname "${dockerfile}")"
  fi
  echo "Loading ${img} into kind cluster '${CLUSTER}'..."
  kind load docker-image "${img}" --name "${CLUSTER}"
}

deploy_observability() {
  step "5/5 Prometheus + Grafana (dashboard + radar)"
  build_grafana_image
  ${K} -n "${NS_OP}" create configmap demo-grafana-dashboard \
    --from-file=ai-finops-overview.json="${OPERATOR}/dashboards/ai-finops-overview.json" \
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
  local available i score latencies gateway_metrics operator_metrics metric
  step "6d/6 validate real latency telemetry"
  if [ "${REQUIRE_LATENCY_TELEMETRY}" != "true" ]; then
    echo "Latency telemetry validation skipped (REQUIRE_LATENCY_TELEMETRY=false)."
    return 0
  fi

  gateway_metrics=""
  for i in $(seq 1 12); do
    gateway_metrics="$(scrape_gateway_metrics || true)"
    if grep -q '^gen_ai_server_request_duration_seconds_' <<<"${gateway_metrics}"; then
      break
    fi
    echo "gateway duration metric not visible yet (${i}/12); retrying in 5s"
    sleep 5
  done
  if ! grep -q '^gen_ai_server_request_duration_seconds_' <<<"${gateway_metrics}"; then
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
  if ! grep -Eq '(^| )[1-9][0-9]*(\.[0-9]+)?($| )' <<<"${latencies}"; then
    echo "No positive observedLatencyMillis found in routingScores: ${latencies:-<empty>}." >&2
    return 1
  fi

  operator_metrics="$(scrape_operator_metrics || true)"
  for metric in ai_finops_latency_score ai_finops_measured_latency_millis ai_finops_latency_telemetry_available ai_finops_routing_score; do
    if ! grep -q "^${metric}" <<<"${operator_metrics}"; then
      echo "Operator /metrics does not expose ${metric}; dashboard latency panel would have no real source." >&2
      return 1
    fi
  done
  echo "Latency telemetry OK: gateway duration histogram observed, report status true, routing score=${score}."
}

validate_quality_score() {
  local i statuses operator_metrics providers dimensions min_providers min_provider_default
  step "6e/6 validate real AI Quality Score"
  min_provider_default=2
  if [ "${REQUIRE_THIRD_QUALITY_PROVIDER}" = "true" ]; then
    min_provider_default=3
  fi
  min_providers="${QUALITY_MIN_PROVIDERS:-${min_provider_default}}"

  statuses=""
  for i in $(seq 1 36); do
    ${K} -n default annotate aiqualitygate --all demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
    sleep 5
    statuses="$(${K} -n default get aiqualitygate -o jsonpath='{range .items[*]}{.metadata.name}{"="}{.status.verdict}{":"}{.status.qualityScore}{"\n"}{end}' 2>/dev/null || true)"
    if [ -n "${statuses}" ] && printf '%s\n' "${statuses}" | awk -F'[=:]' '
      NF < 3 { bad=1; next }
      $2 != "candidate-safe" && $2 != "candidate-risk" { bad=1 }
      ($3 + 0) <= 0 { bad=1 }
      END { exit bad }
    '; then
      break
    fi
  done

  if [ -z "${statuses}" ] || ! printf '%s\n' "${statuses}" | awk -F'[=:]' '
    NF < 3 { bad=1; next }
    $2 != "candidate-safe" && $2 != "candidate-risk" { bad=1 }
    ($3 + 0) <= 0 { bad=1 }
    END { exit bad }
  '; then
    echo "AIQualityGate qualityScore is not ready for all demo gates." >&2
    printf '%s\n' "${statuses:-<empty>}" >&2
    ${K} -n default get aiqualitygate -o yaml >&2 || true
    return 1
  fi
  printf '%s\n' "${statuses}"

  operator_metrics="$(scrape_operator_metrics || true)"
  if ! grep -q '^ai_finops_quality_score' <<<"${operator_metrics}"; then
    echo "Operator /metrics does not expose ai_finops_quality_score after quality gates completed." >&2
    return 1
  fi
  providers="$(printf '%s\n' "${operator_metrics}" | awk '/^ai_finops_quality_score/ {
    if (match($0, /provider="[^"]+"/)) {
      print substr($0, RSTART + 10, RLENGTH - 11)
    }
  }' | sort -u | tr '\n' ' ')"
  dimensions="$(printf '%s\n' "${operator_metrics}" | awk '/^ai_finops_quality_score/ {
    if (match($0, /dimension="[^"]+"/)) {
      print substr($0, RSTART + 11, RLENGTH - 12)
    }
  }' | sort -u | tr '\n' ' ')"
  if [ "$(printf '%s\n' "${providers}" | wc -w | tr -d ' ')" -lt "${min_providers}" ]; then
    echo "Quality score radar needs at least ${min_providers} sovereignty-compliant scored providers; got: ${providers:-<none>}" >&2
    return 1
  fi
  for dimension in correctness reliability latency semantic overall; do
    if ! grep -qw "${dimension}" <<<"${dimensions}"; then
      echo "Quality score metric is missing dimension=${dimension}; dimensions=${dimensions:-<none>}" >&2
      return 1
    fi
  done
  echo "Quality score metrics OK: providers=${providers}; dimensions=${dimensions}"
}

up() {
  preflight; ensure_cluster; isolate_existing_gitops; ensure_operator_image; ensure_operator; cleanup_legacy_aiops_state; ensure_envoy
  deploy_gateway_and_apps; deploy_openai_fr; deploy_openai_us; deploy_mistral_eu; deploy_third_quality_provider; wait_consumer_apps; deploy_quality_gates; deploy_observability; ensure_tetragon; deploy_shadow_ai; refresh_operator_status; validate_latency_telemetry; validate_quality_score
  step "Done — real demo is live"
  echo "Apps in ns rh/legal call Azure OpenAI France; finance calls Azure OpenAI US; marketing calls Mistral EU."
  echo "QualityScore jobs evaluate only sovereignty-compliant candidates and require 3 scored providers by default."
  echo "The operator reads real token usage and computes cost/sovereignty/budget per app."
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
  ${K} delete -f "${HERE}/08-quality-gates.yaml" --ignore-not-found
  ${K} -n default delete aiqualitygate finance-risk-assistant-mistral-small-quality --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete jobs -l aiops.imperium.io/quality-evaluator=true --ignore-not-found >/dev/null 2>&1 || true
  ${K} -n default delete configmap \
    finance-quality-evidence finance-mistral-small-quality-evidence rh-quality-evidence legal-quality-evidence marketing-quality-evidence \
    --ignore-not-found >/dev/null 2>&1 || true
  ${K} delete -f "${THIRD_QUALITY_PROVIDER_FILE}" --ignore-not-found
  ${K} delete -f "${HERE}/05b-mistral-small-eu.yaml" --ignore-not-found >/dev/null 2>&1 || true
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
  ${K} -n default delete secret greenops-foundry-apikey greenops-openai-apikey greenops-openai-fr-apikey greenops-openai-us-apikey greenops-mistral-apikey --ignore-not-found
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
