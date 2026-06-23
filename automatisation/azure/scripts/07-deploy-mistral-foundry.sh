#!/usr/bin/env bash
# Provision the Azure AI Foundry deployments used by the real Envoy AI Gateway
# demo. Despite the historical filename, this script now manages the three
# Foundry deployments required by the demo:
#   - cohere-command-a-latest  -> Cohere, GlobalStandard
#   - mistral-large-latest     -> Mistral-Large-3, DataZoneStandard
#   - gpt-foundry-eu-mini      -> gpt-4.1-mini, DataZoneStandard
#
# It stores the Foundry account key in Key Vault without printing it. Idempotent.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

FOUNDRY="${FOUNDRY:-greenops-foundry}"
FOUNDRY_API_VERSION="${FOUNDRY_API_VERSION:-2024-05-01-preview}"

COHERE_MODEL="${COHERE_MODEL:-cohere-command-a}"
COHERE_VERSION="${COHERE_VERSION:-1}"
COHERE_DEPLOY="${COHERE_DEPLOY:-cohere-command-a-latest}"
COHERE_FORMAT="${COHERE_FORMAT:-Cohere}"
COHERE_SKU="${COHERE_SKU:-GlobalStandard}"
COHERE_CAP="${COHERE_CAP:-1}"

MISTRAL_MODEL="${MISTRAL_MODEL:-Mistral-Large-3}"
MISTRAL_VERSION="${MISTRAL_VERSION:-1}"
MISTRAL_DEPLOY="${MISTRAL_DEPLOY:-mistral-large-latest}"
MISTRAL_FORMAT="${MISTRAL_FORMAT:-Mistral AI}"
MISTRAL_SKU="${MISTRAL_SKU:-DataZoneStandard}"
MISTRAL_CAP="${MISTRAL_CAP:-20}"

THIRD_MODEL="${THIRD_MODEL:-gpt-4.1-mini}"
THIRD_VERSION="${THIRD_VERSION:-2025-04-14}"
THIRD_DEPLOY="${THIRD_DEPLOY:-gpt-foundry-eu-mini}"
THIRD_FORMAT="${THIRD_FORMAT:-OpenAI}"
THIRD_SKU="${THIRD_SKU:-DataZoneStandard}"
THIRD_CAP="${THIRD_CAP:-10}"

ENABLE_MISTRAL_SMALL="${ENABLE_MISTRAL_SMALL:-false}"
MISTRAL_SMALL_MODEL="${MISTRAL_SMALL_MODEL:-mistral-small-2503}"
MISTRAL_SMALL_VERSION="${MISTRAL_SMALL_VERSION:-1}"
MISTRAL_SMALL_DEPLOY="${MISTRAL_SMALL_DEPLOY:-mistral-small-latest}"
MISTRAL_SMALL_FORMAT="${MISTRAL_SMALL_FORMAT:-Mistral AI}"
MISTRAL_SMALL_SKU="${MISTRAL_SMALL_SKU:-DataZoneStandard}"
MISTRAL_SMALL_CAP="${MISTRAL_SMALL_CAP:-20}"
ALLOW_FOUNDRY_GLOBAL_FALLBACK="${ALLOW_FOUNDRY_GLOBAL_FALLBACK:-false}"

deployment_state() {
  az cognitiveservices account deployment show \
    -g "${RG}" -n "${FOUNDRY}" --deployment-name "$1" \
    --query properties.provisioningState -o tsv 2>/dev/null || true
}

deploy_foundry_model() {
  local model="$1" version="$2" format="$3" deploy="$4" sku="$5" cap="$6"
  local state

  state="$(deployment_state "${deploy}")"
  if [ "${state}" = "Succeeded" ]; then
    log "deployment '${deploy}' already Succeeded."
    return 0
  fi

  log "deploying ${model} as '${deploy}' (${format}, ${sku}, capacity=${cap})..."
  if az cognitiveservices account deployment create -g "${RG}" -n "${FOUNDRY}" \
      --deployment-name "${deploy}" --model-name "${model}" --model-version "${version}" \
      --model-format "${format}" --sku-name "${sku}" --sku-capacity "${cap}" -o none; then
    return 0
  fi

  if [ "${ALLOW_FOUNDRY_GLOBAL_FALLBACK}" = "true" ]; then
    warn "${sku} failed; retrying GlobalStandard for ${deploy}."
    az cognitiveservices account deployment create -g "${RG}" -n "${FOUNDRY}" \
      --deployment-name "${deploy}" --model-name "${model}" --model-version "${version}" \
      --model-format "${format}" --sku-name GlobalStandard --sku-capacity "${cap}" -o none
    return 0
  fi

  die "${sku} deployment failed for ${deploy}; refusing GlobalStandard fallback because the demo requires explicit residency for compliant providers."
}

az group create -n "${RG}" -l "${LOCATION}" -o none

for p in Microsoft.CognitiveServices Microsoft.MachineLearningServices; do
  [ "$(az provider show -n "$p" --query registrationState -o tsv 2>/dev/null)" = "Registered" ] || az provider register -n "$p" -o none
done

if ! az cognitiveservices account show -n "${FOUNDRY}" -g "${RG}" >/dev/null 2>&1; then
  log "creating Azure AI Foundry (AIServices) account ${FOUNDRY}..."
  az cognitiveservices account create -n "${FOUNDRY}" -g "${RG}" -l "${LOCATION}" \
    --kind AIServices --sku S0 --custom-domain "${FOUNDRY}" --yes -o none
fi

deploy_foundry_model "${COHERE_MODEL}" "${COHERE_VERSION}" "${COHERE_FORMAT}" "${COHERE_DEPLOY}" "${COHERE_SKU}" "${COHERE_CAP}"
deploy_foundry_model "${MISTRAL_MODEL}" "${MISTRAL_VERSION}" "${MISTRAL_FORMAT}" "${MISTRAL_DEPLOY}" "${MISTRAL_SKU}" "${MISTRAL_CAP}"
deploy_foundry_model "${THIRD_MODEL}" "${THIRD_VERSION}" "${THIRD_FORMAT}" "${THIRD_DEPLOY}" "${THIRD_SKU}" "${THIRD_CAP}"

if [ "${ENABLE_MISTRAL_SMALL}" = "true" ]; then
  deploy_foundry_model "${MISTRAL_SMALL_MODEL}" "${MISTRAL_SMALL_VERSION}" "${MISTRAL_SMALL_FORMAT}" "${MISTRAL_SMALL_DEPLOY}" "${MISTRAL_SMALL_SKU}" "${MISTRAL_SMALL_CAP}"
fi

ENDPOINT="https://${FOUNDRY}.services.ai.azure.com/models"
CHAT_ENDPOINT="https://${FOUNDRY}.services.ai.azure.com"
KEY="$(az cognitiveservices account keys list -n "${FOUNDRY}" -g "${RG}" --query key1 -o tsv)"

# Store the key in Key Vault (no echo). Keep the old secret name for existing automation.
if az keyvault show -n "${KEYVAULT}" -g "${RG}" >/dev/null 2>&1; then
  tmp="$(mktemp)"; chmod 600 "${tmp}"; printf '%s' "${KEY}" > "${tmp}"
  az keyvault secret set --vault-name "${KEYVAULT}" --name foundry-api-key --file "${tmp}" -o none || true
  az keyvault secret set --vault-name "${KEYVAULT}" --name foundry-mistral-key --file "${tmp}" -o none || true
  shred -u "${tmp}" 2>/dev/null || rm -f "${tmp}"
  log "stored foundry-api-key and foundry-mistral-key in Key Vault ${KEYVAULT}."
fi

cat <<EOF
[azure] Azure AI Foundry deployments ready.
  account:      ${FOUNDRY} (${RG}, ${LOCATION})
  endpoint:     ${ENDPOINT}
  chat base:    ${CHAT_ENDPOINT}
  api-version:  ${FOUNDRY_API_VERSION}

  deployments:
    ${COHERE_DEPLOY}  -> ${COHERE_MODEL} (${COHERE_FORMAT}, ${COHERE_SKU})
    ${MISTRAL_DEPLOY} -> ${MISTRAL_MODEL} (${MISTRAL_FORMAT}, ${MISTRAL_SKU})
    ${THIRD_DEPLOY}   -> ${THIRD_MODEL} (${THIRD_FORMAT}, ${THIRD_SKU})

Sync the local ignored key for the Kind demo, without printing it:
  az cognitiveservices account keys list -n ${FOUNDRY} -g ${RG} --query key1 -o tsv > ${OPERATOR_DIR}/docs/foundrykey.txt

Run the strict Kind proof:
  cd ${REPO_ROOT}/automatisation/envoy-aigw
  ./deploy.sh verify
EOF

az cognitiveservices account deployment list -g "${RG}" -n "${FOUNDRY}" \
  --query '[].{name:name,model:properties.model.name,format:properties.model.format,version:properties.model.version,sku:sku.name,capacity:sku.capacity,state:properties.provisioningState}' \
  -o table
