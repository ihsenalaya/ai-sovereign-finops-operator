#!/usr/bin/env bash
# Provision the SECOND LLM provider for the experiments: Mistral on Azure AI
# Foundry (serverless, no GPU/quota). Deploys Mistral-Large-3 in DataZoneStandard
# (EU data zone) — a real EU-hosted provider that makes RQ4 sovereignty real.
# Stores the key in Key Vault. Idempotent.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

FOUNDRY="${FOUNDRY:-greenops-foundry}"
MISTRAL_MODEL="${MISTRAL_MODEL:-Mistral-Large-3}"
MISTRAL_VERSION="${MISTRAL_VERSION:-1}"
MISTRAL_DEPLOY="${MISTRAL_DEPLOY:-mistral-large-latest}"
MISTRAL_SKU="${MISTRAL_SKU:-DataZoneStandard}"   # EU data zone; no global fallback unless explicitly allowed
MISTRAL_CAP="${MISTRAL_CAP:-20}"
ENABLE_MISTRAL_SMALL="${ENABLE_MISTRAL_SMALL:-true}"
MISTRAL_SMALL_MODEL="${MISTRAL_SMALL_MODEL:-Mistral-small-2503}"
MISTRAL_SMALL_VERSION="${MISTRAL_SMALL_VERSION:-1}"
MISTRAL_SMALL_DEPLOY="${MISTRAL_SMALL_DEPLOY:-mistral-small-latest}"
MISTRAL_SMALL_SKU="${MISTRAL_SMALL_SKU:-DataZoneStandard}"
MISTRAL_SMALL_CAP="${MISTRAL_SMALL_CAP:-20}"
ALLOW_FOUNDRY_GLOBAL_FALLBACK="${ALLOW_FOUNDRY_GLOBAL_FALLBACK:-false}"

deploy_foundry_model() {
  local model="$1" version="$2" deploy="$3" sku="$4" cap="$5"
  log "deploying ${model} as '${deploy}' (${sku})..."
  if az cognitiveservices account deployment create -g "${RG}" -n "${FOUNDRY}" \
      --deployment-name "${deploy}" --model-name "${model}" --model-version "${version}" \
      --model-format "Mistral AI" --sku-name "${sku}" --sku-capacity "${cap}" -o none 2>/dev/null; then
    return 0
  fi
  if [ "${ALLOW_FOUNDRY_GLOBAL_FALLBACK}" = "true" ]; then
    warn "${sku} failed; retrying GlobalStandard for ${deploy}."
    az cognitiveservices account deployment create -g "${RG}" -n "${FOUNDRY}" \
      --deployment-name "${deploy}" --model-name "${model}" --model-version "${version}" \
      --model-format "Mistral AI" --sku-name GlobalStandard --sku-capacity "${cap}" -o none
    return 0
  fi
  die "${sku} deployment failed for ${deploy}; refusing GlobalStandard fallback because the demo requires EU data residency."
}

az group create -n "${RG}" -l "${LOCATION}" -o none

for p in Microsoft.CognitiveServices Microsoft.MachineLearningServices; do
  [ "$(az provider show -n $p --query registrationState -o tsv 2>/dev/null)" = "Registered" ] || az provider register -n $p -o none
done

if ! az cognitiveservices account show -n "${FOUNDRY}" -g "${RG}" >/dev/null 2>&1; then
  log "creating Azure AI Foundry (AIServices) account ${FOUNDRY}..."
  az cognitiveservices account create -n "${FOUNDRY}" -g "${RG}" -l "${LOCATION}" \
    --kind AIServices --sku S0 --custom-domain "${FOUNDRY}" --yes -o none
fi

deploy_foundry_model "${MISTRAL_MODEL}" "${MISTRAL_VERSION}" "${MISTRAL_DEPLOY}" "${MISTRAL_SKU}" "${MISTRAL_CAP}"
if [ "${ENABLE_MISTRAL_SMALL}" = "true" ]; then
  deploy_foundry_model "${MISTRAL_SMALL_MODEL}" "${MISTRAL_SMALL_VERSION}" "${MISTRAL_SMALL_DEPLOY}" "${MISTRAL_SMALL_SKU}" "${MISTRAL_SMALL_CAP}"
fi

ENDPOINT="https://${FOUNDRY}.services.ai.azure.com/models"
KEY="$(az cognitiveservices account keys list -n "${FOUNDRY}" -g "${RG}" --query key1 -o tsv)"

# Store the key in Key Vault (no echo).
if az keyvault show -n "${KEYVAULT}" -g "${RG}" >/dev/null 2>&1; then
  tmp="$(mktemp)"; chmod 600 "${tmp}"; printf '%s' "${KEY}" > "${tmp}"
  az keyvault secret set --vault-name "${KEYVAULT}" --name foundry-mistral-key --file "${tmp}" -o none || true
  shred -u "${tmp}" 2>/dev/null || rm -f "${tmp}"
  log "stored foundry-mistral-key in Key Vault ${KEYVAULT}."
fi

cat <<EOF
[azure] Mistral (EU) ready on Azure AI Foundry.
  endpoint:    ${ENDPOINT}
  deployment:  ${MISTRAL_DEPLOY}   (model ${MISTRAL_MODEL}, ${MISTRAL_SKU})
  small:       ${MISTRAL_SMALL_DEPLOY}   (model ${MISTRAL_SMALL_MODEL}, ${MISTRAL_SMALL_SKU}, enabled=${ENABLE_MISTRAL_SMALL})
  api-version: 2024-05-01-preview   (auth: api-key header)

Run the experiment with the second provider:
  cd ${REPO_ROOT}/experimentation
  MISTRAL_API_KEY="\$(az cognitiveservices account keys list -n ${FOUNDRY} -g ${RG} --query key1 -o tsv)" \\
  go run ./cmd/experiment -key ../operateur/docs/openaikey.txt -results results -datasets datasets -judge gpt-4o \\
    -mistral-base ${ENDPOINT} -mistral-auth api-key -mistral-api-version 2024-05-01-preview
EOF
