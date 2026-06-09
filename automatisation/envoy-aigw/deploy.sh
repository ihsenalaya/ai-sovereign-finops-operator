#!/usr/bin/env bash
# ============================================================================
# greenops — full reproducible REAL demo, from scratch, no manual steps.
#
# Real apps consume tokens through Envoy AI Gateway; the operator reads the
# gateway's real per-app token usage and computes cost / sovereignty / budget;
# Prometheus + Grafana show truthful values.
#
# Brings up, in order and idempotently:
#   1. kind cluster (created if missing)
#   2. the operator (Helm chart + CRDs, image OPERATOR_IMG)
#   3. Envoy Gateway v1.5.0 (with AI Gateway values) + Envoy AI Gateway v0.7.0
#   4. Gateway + OpenAI route (+ key Secret) + catalog + 2 consumer apps
#   5. Prometheus + Grafana with the AI-FinOps dashboard
#
# Pinned versions are deliberate (see EG_VERSION / AIGW_VERSION below).
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

# helm OCI pulls fail when the WSL docker credential helper is used -> anonymous.
export DOCKER_CONFIG="${DOCKER_CONFIG:-/tmp/emptydockercfg}"
mkdir -p "${DOCKER_CONFIG}"; [ -f "${DOCKER_CONFIG}/config.json" ] || echo '{}' > "${DOCKER_CONFIG}/config.json"

step() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
need() { command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 1; }; }

preflight() { for t in kubectl helm kind docker; do need "$t"; done; }

ensure_cluster() {
  step "1/5 kind cluster '${CLUSTER}'"
  if kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"; then
    echo "cluster exists."
  else
    kind create cluster --name "${CLUSTER}" --config "${AUTO}/kind/kind-config.yaml" 2>/dev/null \
      || kind create cluster --name "${CLUSTER}"
  fi
  ${K} cluster-info >/dev/null
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
  helm --kube-context "${CTX}" upgrade --install greenops "${REPO}/charts/ai-sovereign-finops-operator" \
    --namespace "${NS_OP}" --create-namespace \
    --set image.repository="${OPERATOR_IMG%:*}" --set image.tag="${OPERATOR_IMG##*:}" \
    --set image.pullPolicy=IfNotPresent
  ${K} -n "${NS_OP}" rollout status deploy/greenops-ai-sovereign-finops-operator --timeout=180s
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
  step "4/5 Gateway + OpenAI route + catalog + consumer apps"
  local oaikey
  oaikey="$(grep -oE 'sk-[A-Za-z0-9_-]+' "${REPO}/docs/openaikey.txt" | head -1)"
  [ -n "${oaikey}" ] || { echo "no OpenAI key in docs/openaikey.txt" >&2; exit 1; }
  ${K} -n default create secret generic greenops-openai-apikey \
    --from-literal=apiKey="${oaikey}" --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} apply -f "${HERE}/01-gateway-openai.yaml"
  ${K} wait pods --timeout=180s -l gateway.envoyproxy.io/owning-gateway-name=greenops-aigw \
    -n envoy-gateway-system --for=condition=Ready
  ${K} apply -f "${HERE}/02-metrics-and-catalog.yaml"
  ${K} apply -f "${HERE}/03-consumer-apps.yaml"
}

deploy_mistral_eu() {
  # Optional 2nd provider: Mistral on Azure AI Foundry (EU zone) — proves the
  # sovereignty engine is zone-aware (the EU app yields ZERO violation). Skipped
  # cleanly if no key is present, so the OpenAI-only demo still works.
  step "4b/6 Mistral EU app (sovereignty zone test)"
  if [ ! -f "${REPO}/docs/mistralkey.txt" ]; then
    echo "no docs/mistralkey.txt — skipping the Mistral EU app."
    echo "  get the key: az cognitiveservices account keys list -n greenops-foundry -g greenops-rg --query key1 -o tsv > docs/mistralkey.txt"
    return 0
  fi
  local mkey; mkey="$(tr -d '\r\n' < "${REPO}/docs/mistralkey.txt")"
  [ -n "${mkey}" ] || { echo "docs/mistralkey.txt is empty — skipping Mistral EU app." >&2; return 0; }
  ${K} -n default create secret generic greenops-mistral-apikey \
    --from-literal=apiKey="${mkey}" --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} apply -f "${HERE}/05-mistral-eu.yaml"
}

deploy_observability() {
  step "5/5 Prometheus + Grafana (dashboard)"
  ${K} -n "${NS_OP}" create configmap demo-grafana-dashboard \
    --from-file=ai-finops-overview.json="${REPO}/dashboards/ai-finops-overview.json" \
    --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} label cm demo-grafana-dashboard -n "${NS_OP}" aiops.imperium.io/demo=true --overwrite >/dev/null
  ${K} apply -f "${AUTO}/demo/observability.yaml"
  ${K} -n "${NS_OP}" rollout status deploy/demo-prometheus --timeout=120s
  ${K} -n "${NS_OP}" rollout status deploy/demo-grafana --timeout=120s
}

up() {
  preflight; ensure_cluster; ensure_operator_image; ensure_operator; ensure_envoy
  deploy_gateway_and_apps; deploy_mistral_eu; deploy_observability
  step "Done — real demo is live"
  echo "Apps in ns rh/finance call OpenAI through Envoy AI Gateway; the operator"
  echo "reads real token usage and computes cost/sovereignty/budget per app."
  echo
  echo "Open Grafana:  ./deploy.sh grafana   (then http://localhost:3000)"
  echo "Watch cost:    ${K} -n default get aifinopsreport ai-report-all -o yaml | grep -A4 'status:'"
  echo "Let it run a few minutes so cost/tokens accumulate to meaningful values."
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
    sh -c "curl -s -m 60 -H 'Content-Type: application/json' -d '{\"model\":\"gpt-4o\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"max_tokens\":20}' ${ep}"
}

down() {
  step "Removing the real demo"
  ${K} delete -f "${HERE}/05-mistral-eu.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/03-consumer-apps.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/02-metrics-and-catalog.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/01-gateway-openai.yaml" --ignore-not-found
  ${K} -n default delete secret greenops-openai-apikey greenops-mistral-apikey --ignore-not-found
  ${K} -n "${NS_OP}" delete deploy,svc,configmap -l aiops.imperium.io/demo=true --ignore-not-found
  echo "Operator + Envoy control planes kept (helm uninstall greenops / eg / aieg to remove)."
}

case "${1:-up}" in
  up) up ;;
  grafana) grafana ;;
  test) test_call ;;
  down) down ;;
  *) echo "usage: $0 [up|grafana|test|down]" >&2; exit 1 ;;
esac
