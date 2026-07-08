#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTICLE2_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${ARTICLE2_DIR}/.." && pwd)"
source "${ARTICLE2_DIR}/infra/scripts/common.sh"

require kubectl
require helm
require curl

OUTPUTS="${EVIDENCE_DIR}/terraform_outputs.json"
[ -f "${OUTPUTS}" ] || die "missing ${OUTPUTS}; run article2 infra apply first"

RG="$(json_value "${OUTPUTS}" resource_group_name)"
CLUSTER="$(json_value "${OUTPUTS}" cluster_name)"
FOUNDRY="$(json_value "${OUTPUTS}" foundry_account_name)"
KEYVAULT="$(json_value "${OUTPUTS}" keyvault_name)"
LOCATION="$(json_value "${OUTPUTS}" location)"
FOUNDRY_ENDPOINT="$(json_value "${OUTPUTS}" foundry_chat_endpoint)"
FOUNDRY_HOST="${FOUNDRY_ENDPOINT#https://}"
FOUNDRY_HOST="${FOUNDRY_HOST%/}"
AKS_EVIDENCE_DIR="${ARTICLE2_DIR}/experiments/evidence/aks-live"
OPERATOR_NS="${OPERATOR_NS:-greenops-system}"
OPERATOR_CHART_VERSION="${OPERATOR_CHART_VERSION:-0.5.13}"
OPERATOR_CHART="${OPERATOR_CHART:-${REPO_ROOT}/operateur/charts/ai-sovereign-finops-operator}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator/controller}"
OPERATOR_TAG="${OPERATOR_TAG:-0.5.13}"
CTX="${CTX:-${CLUSTER}}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-${AKS_EVIDENCE_DIR}/aks-kubeconfig}"
ENABLE_THIRD_QUALITY_PROVIDER="${ENABLE_THIRD_QUALITY_PROVIDER:-false}"
ENVOY_DIR="${REPO_ROOT}/automatisation/envoy-aigw"

mkdir -p "${AKS_EVIDENCE_DIR}/logs" "${AKS_EVIDENCE_DIR}/yaml" "${AKS_EVIDENCE_DIR}/metrics"

log "fetching AKS credentials into ${KUBECONFIG_PATH} (context=${CTX})"
mkdir -p "$(dirname "${KUBECONFIG_PATH}")"
az aks get-credentials -g "${RG}" -n "${CLUSTER}" --file - --context "${CTX}" > "${KUBECONFIG_PATH}"
export KUBECONFIG="${KUBECONFIG_PATH}"
kubectl config use-context "${CTX}" >/dev/null
kubectl cluster-info > "${AKS_EVIDENCE_DIR}/cluster_info.txt"

log "ensuring Foundry deployments and syncing key material"
(
  export RG="${RG}"
  export LOCATION="${LOCATION}"
  export FOUNDRY="${FOUNDRY}"
  export KEYVAULT="${KEYVAULT}"
  export THIRD_MODEL="DeepSeek-V4-Flash"
  export THIRD_VERSION="2026-04-23"
  export THIRD_DEPLOY="judge-a"
  export THIRD_FORMAT="DeepSeek"
  export THIRD_SKU="GlobalStandard"
  export THIRD_CAP="10"
  bash "${REPO_ROOT}/automatisation/azure/scripts/07-deploy-mistral-foundry.sh"
) | tee "${AKS_EVIDENCE_DIR}/logs/foundry_prepare.log"
az cognitiveservices account keys list -n "${FOUNDRY}" -g "${RG}" --query key1 -o tsv | tr -d '\r\n' > "${REPO_ROOT}/operateur/docs/foundrykey.txt"

log "ensuring Azure OpenAI FR/US accounts for residency experiments"
(
  export RG="${RG}"
  export LOCATION="${LOCATION}"
  export FR_ACCOUNT="a2fr${AZURE_SUBSCRIPTION_SHORT}${ARTICLE2_DATE_CODE}"
  export US_ACCOUNT="a2us${AZURE_SUBSCRIPTION_SHORT}${ARTICLE2_DATE_CODE}"
  bash "${REPO_ROOT}/automatisation/azure/scripts/08-deploy-openai-fr-us.sh"
) | tee "${AKS_EVIDENCE_DIR}/logs/openai_fr_us_prepare.log"

log "installing operator CRDs and Helm chart from GHCR"
kubectl apply -f "${REPO_ROOT}/operateur/config/crd/bases" > "${AKS_EVIDENCE_DIR}/logs/operator_crds_apply.log"
PULL_ARGS=()
case "${OPERATOR_IMAGE}" in
  ghcr.io/*)
    GHCR_USER="${GHCR_USER:-$(printf '%s' "${OPERATOR_IMAGE}" | cut -d/ -f2)}"
    GHCR_TOKEN="${GHCR_TOKEN:-$(grep -E 'oauth_token:' "${HOME}/.config/gh/hosts.yml" 2>/dev/null | head -1 | awk '{print $2}')}"
    if [ -n "${GHCR_TOKEN}" ]; then
      kubectl create namespace "${OPERATOR_NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
      kubectl -n "${OPERATOR_NS}" create secret docker-registry ghcr-pull \
        --docker-server=ghcr.io --docker-username="${GHCR_USER}" --docker-password="${GHCR_TOKEN}" \
        --docker-email="ci@article2.local" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
      PULL_ARGS=(--set imagePullSecrets[0].name=ghcr-pull)
    fi
    ;;
esac
helm upgrade --install greenops "${OPERATOR_CHART}" \
  --namespace "${OPERATOR_NS}" \
  --create-namespace \
  --set image.repository="${OPERATOR_IMAGE}" \
  --set image.tag="${OPERATOR_TAG}" \
  "${PULL_ARGS[@]}" \
  --wait --timeout 10m | tee "${AKS_EVIDENCE_DIR}/logs/operator_helm_install.log"
kubectl -n "${OPERATOR_NS}" rollout status deploy/greenops-ai-sovereign-finops-operator --timeout=300s | tee "${AKS_EVIDENCE_DIR}/logs/operator_rollout.txt"

log "installing Envoy Gateway and Envoy AI Gateway"
curl -fsSL -o /tmp/article2-eg-values.yaml \
  "https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.7.0/manifests/envoy-gateway-values.yaml"
helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm --version v1.5.0 \
  -f /tmp/article2-eg-values.yaml -n envoy-gateway-system --create-namespace \
  --wait --timeout 10m | tee "${AKS_EVIDENCE_DIR}/logs/envoy_gateway_install.log"
helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm --version v0.7.0 \
  -n envoy-ai-gateway-system --create-namespace \
  --wait --timeout 10m | tee "${AKS_EVIDENCE_DIR}/logs/aieg_crd_install.log"
helm upgrade -i aieg oci://docker.io/envoyproxy/ai-gateway-helm --version v0.7.0 \
  -n envoy-ai-gateway-system --create-namespace \
  --set "controller.metricsRequestHeaderAttributes=x-greenops-namespace:k8s.namespace\,x-greenops-app:k8s.app" \
  --wait --timeout 10m | tee "${AKS_EVIDENCE_DIR}/logs/aieg_install.log"
kubectl -n envoy-gateway-system rollout status deploy/envoy-gateway --timeout=300s | tee "${AKS_EVIDENCE_DIR}/logs/envoy_gateway_rollout.txt"
kubectl -n envoy-ai-gateway-system rollout status deploy/ai-gateway-controller --timeout=300s | tee "${AKS_EVIDENCE_DIR}/logs/aieg_rollout.txt"

log "creating provider secrets"
kubectl -n default create secret generic greenops-foundry-apikey \
  --from-file=apiKey="${REPO_ROOT}/operateur/docs/foundrykey.txt" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n default create secret generic greenops-mistral-apikey \
  --from-file=apiKey="${REPO_ROOT}/operateur/docs/foundrykey.txt" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n default create secret generic greenops-openai-fr-apikey \
  --from-file=apiKey="${REPO_ROOT}/operateur/docs/openai-fr-key.txt" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n default create secret generic greenops-openai-us-apikey \
  --from-file=apiKey="${REPO_ROOT}/operateur/docs/openai-us-key.txt" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

log "patching Foundry hostnames in demo manifests to current account"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
for file in 01-gateway-cohere.yaml 05-mistral-eu.yaml 05b-openai-foundry-eu.yaml; do
  sed "s/greenops-foundry.services.ai.azure.com/${FOUNDRY_HOST}/g" \
    "${ENVOY_DIR}/${file}" > "${TMP_DIR}/${file}"
done
cp "${ENVOY_DIR}/02-metrics-and-catalog.yaml" "${TMP_DIR}/02-metrics-and-catalog.yaml"
cp "${ENVOY_DIR}/06-openai-fr.yaml" "${TMP_DIR}/06-openai-fr.yaml"
cp "${ENVOY_DIR}/07-openai-us.yaml" "${TMP_DIR}/07-openai-us.yaml"
cp "${ENVOY_DIR}/08-quality-gates.yaml" "${TMP_DIR}/08-quality-gates.yaml"

log "applying real gateway + catalog + workloads"
kubectl apply -f "${TMP_DIR}/01-gateway-cohere.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/01_gateway_apply.log"
kubectl apply -f "${TMP_DIR}/02-metrics-and-catalog.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/02_catalog_apply.log"
kubectl apply -f "${TMP_DIR}/06-openai-fr.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/06_openai_fr_apply.log"
kubectl apply -f "${TMP_DIR}/07-openai-us.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/07_openai_us_apply.log"
kubectl apply -f "${TMP_DIR}/05-mistral-eu.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/05_mistral_apply.log"
if [ "${ENABLE_THIRD_QUALITY_PROVIDER}" = "true" ]; then
  kubectl apply -f "${TMP_DIR}/05b-openai-foundry-eu.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/05b_foundry_openai_apply.log"
else
  log "skipping optional third quality provider on this subscription"
fi
kubectl apply -f "${TMP_DIR}/08-quality-gates.yaml" | tee "${AKS_EVIDENCE_DIR}/logs/08_quality_gates_apply.log"

log "waiting for application workloads"
for ns_app in "rh/chatbot-rh" "finance/risk-assistant" "legal/contract-review" "marketing/content-writer"; do
  ns="${ns_app%%/*}"
  app="${ns_app##*/}"
  kubectl -n "${ns}" rollout status "deploy/${app}" --timeout=300s | tee -a "${AKS_EVIDENCE_DIR}/logs/app_rollouts.txt"
done

log "capturing initial AKS evidence"
kubectl get pods -A -o wide > "${AKS_EVIDENCE_DIR}/kubectl_pods.txt"
kubectl get deploy -A > "${AKS_EVIDENCE_DIR}/kubectl_deployments.txt"
kubectl get svc -A > "${AKS_EVIDENCE_DIR}/kubectl_services.txt"
kubectl get aigw,aiprov,aimodel,aiqgate,aisov,aibudget,aireport -A > "${AKS_EVIDENCE_DIR}/kubectl_aiops.txt"
kubectl get aigw,aiprov,aimodel,aiqgate,aisov,aibudget,aireport -A -o yaml > "${AKS_EVIDENCE_DIR}/yaml/aiops.yaml"
kubectl get gateway,gatewayclass,aigatewayroute,aiservicebackend,backend,backendtlspolicy -A -o yaml > "${AKS_EVIDENCE_DIR}/yaml/gateway.yaml"

log "stack ready on AKS; let workloads generate traffic before collecting measured results"
