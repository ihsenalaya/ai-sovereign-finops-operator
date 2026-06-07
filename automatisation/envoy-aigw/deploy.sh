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
OPERATOR_IMG="${OPERATOR_IMG:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.2.7}"
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

ensure_operator() {
  step "2/5 operator (Helm, ${OPERATOR_IMG})"
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
  preflight; ensure_cluster; ensure_operator; ensure_envoy; deploy_gateway_and_apps; deploy_observability
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
  ${K} delete -f "${HERE}/03-consumer-apps.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/02-metrics-and-catalog.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/01-gateway-openai.yaml" --ignore-not-found
  ${K} -n default delete secret greenops-openai-apikey --ignore-not-found
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
