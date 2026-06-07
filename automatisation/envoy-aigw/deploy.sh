#!/usr/bin/env bash
# Reproducible end-to-end demo: REAL apps consuming tokens through Envoy AI Gateway,
# with the greenops operator reading the gateway's real per-app token usage and
# computing cost / sovereignty, surfaced in Grafana.
#
# Pinned, working versions (IMPORTANT):
#   - Envoy Gateway  v1.5.0   (v1.3.0 is too old: extproc not injected)
#   - Envoy AI Gateway v0.7.0
#
# Usage: ./deploy.sh up      # install + configure + deploy apps
#        ./deploy.sh test    # one real call through the gateway (sanity)
#        ./deploy.sh down     # remove the Envoy AI Gateway demo
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${HERE}/../.." && pwd)"
CTX="${KCTX:-kind-greenops}"
K="kubectl --context ${CTX}"
EG_VERSION="${EG_VERSION:-v1.5.0}"
AIGW_VERSION="${AIGW_VERSION:-v0.7.0}"
# helm OCI pulls fail if the WSL docker credential helper is used; force anonymous.
export DOCKER_CONFIG="${DOCKER_CONFIG:-/tmp/emptydockercfg}"
mkdir -p "${DOCKER_CONFIG}" && [ -f "${DOCKER_CONFIG}/config.json" ] || echo '{}' > "${DOCKER_CONFIG}/config.json"

step() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }

up() {
  step "Envoy Gateway ${EG_VERSION} (with AI Gateway values)"
  curl -fsSL -o /tmp/eg-values.yaml \
    "https://raw.githubusercontent.com/envoyproxy/ai-gateway/${AIGW_VERSION}/manifests/envoy-gateway-values.yaml"
  helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm --version "${EG_VERSION}" \
    -f /tmp/eg-values.yaml -n envoy-gateway-system --create-namespace
  ${K} -n envoy-gateway-system rollout status deploy/envoy-gateway --timeout=180s

  step "Envoy AI Gateway ${AIGW_VERSION}"
  helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm --version "${AIGW_VERSION}" \
    -n envoy-ai-gateway-system --create-namespace
  helm upgrade -i aieg oci://docker.io/envoyproxy/ai-gateway-helm --version "${AIGW_VERSION}" \
    -n envoy-ai-gateway-system --create-namespace
  ${K} wait --timeout=180s -n envoy-ai-gateway-system deployment/ai-gateway-controller --for=condition=Available

  step "Gateway + OpenAI route (+ secret from docs/openaikey.txt)"
  local oaikey
  oaikey="$(grep -oE 'sk-[A-Za-z0-9_-]+' "${REPO}/docs/openaikey.txt" | head -1)"
  ${K} -n default create secret generic greenops-openai-apikey \
    --from-literal=apiKey="${oaikey}" --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} apply -f "${HERE}/01-gateway-openai.yaml"
  ${K} wait pods --timeout=180s -l gateway.envoyproxy.io/owning-gateway-name=greenops-aigw \
    -n envoy-gateway-system --for=condition=Ready

  step "Operator catalog (models->apps, prices, sovereignty) + consumer apps"
  ${K} apply -f "${HERE}/02-metrics-and-catalog.yaml"
  ${K} apply -f "${HERE}/03-consumer-apps.yaml"

  step "Done"
  echo "Real apps now call gpt-4o/gpt-4o-mini through Envoy AI Gateway."
  echo "The operator reads the gateway's gen_ai token metrics (telemetry mode: aigw)."
  echo "Watch cost build up:  ${K} -n default get aifinopsreport ai-report-all -o yaml | grep -A3 status"
}

test_call() {
  step "One real call through Envoy AI Gateway"
  local svc ep
  svc="$(${K} get svc -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-name=greenops-aigw -o jsonpath='{.items[0].metadata.name}')"
  ep="http://${svc}.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions"
  ${K} -n default run aigw-test --rm -i --restart=Never --image=curlimages/curl:8.10.1 --quiet -- \
    sh -c "curl -s -m 60 -H 'Content-Type: application/json' -d '{\"model\":\"gpt-4o\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"max_tokens\":20}' ${ep}"
}

down() {
  step "Removing the Envoy AI Gateway demo"
  ${K} delete -f "${HERE}/03-consumer-apps.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/02-metrics-and-catalog.yaml" --ignore-not-found
  ${K} delete -f "${HERE}/01-gateway-openai.yaml" --ignore-not-found
  ${K} -n default delete secret greenops-openai-apikey --ignore-not-found
  echo "Envoy Gateway / AI Gateway control planes left installed (helm uninstall eg / aieg to remove)."
}

case "${1:-up}" in
  up) up ;;
  test) test_call ;;
  down) down ;;
  *) echo "usage: $0 [up|test|down]" >&2; exit 1 ;;
esac
