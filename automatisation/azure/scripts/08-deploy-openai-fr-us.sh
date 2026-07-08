#!/usr/bin/env bash
# Provision the two Azure OpenAI accounts used by the real Envoy AI Gateway demo:
#   - FR account (francecentral) with deployment gpt-france-mini -> zone FR (compliant)
#   - US account (eastus)        with deployment gpt-us-mini     -> zone US (sovereignty violation)
#
# Account names are derived deterministically from the subscription id so the
# script is idempotent and the demo manifests stay in sync. It:
#   1. creates both Azure OpenAI (kind=OpenAI) accounts + a gpt-4.1-mini deployment each,
#   2. writes each key to the gitignored operateur/docs/openai-{fr,us}-key.txt (no echo),
#   3. rewrites the account hostnames into envoy-aigw/06-openai-fr.yaml and 07-openai-us.yaml
#      and the account names into envoy-aigw/deploy.sh, so `deploy.sh up` finds them.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

MODEL="${OPENAI_FR_US_MODEL:-gpt-4.1-mini}"
MODEL_VERSION="${OPENAI_FR_US_MODEL_VERSION:-2025-04-14}"
MODEL_FORMAT="OpenAI"
SKU="${OPENAI_FR_US_SKU:-Standard}"          # regional Standard keeps data in-region (residency)
CAP="${OPENAI_FR_US_CAP:-10}"

FR_LOCATION="${FR_LOCATION:-francecentral}"
US_LOCATION="${US_LOCATION:-eastus}"
FR_DEPLOY="${FR_DEPLOY:-gpt-france-mini}"
US_DEPLOY="${US_DEPLOY:-gpt-us-mini}"

_sub6="$(az account show --query id -o tsv | tr -d '-' | cut -c1-6)"
FR_ACCOUNT="${FR_ACCOUNT:-greenops-fr-${_sub6}}"
US_ACCOUNT="${US_ACCOUNT:-greenops-us-${_sub6}}"

ENVOY_DIR="${REPO_ROOT}/automatisation/envoy-aigw"

ensure_account() {
  local name="$1" loc="$2"
  if az cognitiveservices account show -n "${name}" -g "${RG}" >/dev/null 2>&1; then
    log "Azure OpenAI account ${name} already exists (${loc})."
    return 0
  fi
  log "creating Azure OpenAI account ${name} in ${loc}..."
  az cognitiveservices account create -n "${name}" -g "${RG}" -l "${loc}" \
    --kind OpenAI --sku S0 --custom-domain "${name}" --yes -o none
}

ensure_deployment() {
  local account="$1" deploy="$2"
  local state
  state="$(az cognitiveservices account deployment show -g "${RG}" -n "${account}" \
    --deployment-name "${deploy}" --query properties.provisioningState -o tsv 2>/dev/null || true)"
  if [ "${state}" = "Succeeded" ]; then
    log "deployment ${account}/${deploy} already Succeeded."
    return 0
  fi
  log "deploying ${MODEL} (${MODEL_VERSION}) as ${account}/${deploy} [${SKU}]..."
  if ! az cognitiveservices account deployment create -g "${RG}" -n "${account}" \
      --deployment-name "${deploy}" --model-name "${MODEL}" --model-version "${MODEL_VERSION}" \
      --model-format "${MODEL_FORMAT}" --sku-name "${SKU}" --sku-capacity "${CAP}" -o none; then
    warn "${SKU} failed for ${account}/${deploy}; retrying GlobalStandard (residency claim weakened)."
    az cognitiveservices account deployment create -g "${RG}" -n "${account}" \
      --deployment-name "${deploy}" --model-name "${MODEL}" --model-version "${MODEL_VERSION}" \
      --model-format "${MODEL_FORMAT}" --sku-name GlobalStandard --sku-capacity "${CAP}" -o none
  fi
}

save_key() {
  local account="$1" out="$2"
  local key; key="$(az cognitiveservices account keys list -n "${account}" -g "${RG}" --query key1 -o tsv)"
  umask 077; printf '%s' "${key}" > "${out}"
  log "wrote key for ${account} -> ${out} (gitignored, not printed)."
}

# Rewrite the demo manifests + deploy.sh so the hardcoded old names point at the new accounts.
sync_manifests() {
  local fr_yaml="${ENVOY_DIR}/06-openai-fr.yaml"
  local us_yaml="${ENVOY_DIR}/07-openai-us.yaml"
  local deploy="${ENVOY_DIR}/deploy.sh"
  # Replace any greenops-fr-* / greenops-us-* token (old or new) with the current names.
  sed -i -E "s/greenops-fr-[a-z0-9-]+/${FR_ACCOUNT}/g" "${fr_yaml}" "${deploy}"
  sed -i -E "s/greenops-us-[a-z0-9-]+/${US_ACCOUNT}/g" "${us_yaml}" "${deploy}"
  log "synced hostnames/account names into 06-openai-fr.yaml, 07-openai-us.yaml, deploy.sh."
}

az group create -n "${RG}" -l "${LOCATION}" -o none
[ "$(az provider show -n Microsoft.CognitiveServices --query registrationState -o tsv 2>/dev/null)" = "Registered" ] \
  || az provider register -n Microsoft.CognitiveServices -o none

ensure_account "${FR_ACCOUNT}" "${FR_LOCATION}"
ensure_account "${US_ACCOUNT}" "${US_LOCATION}"
ensure_deployment "${FR_ACCOUNT}" "${FR_DEPLOY}"
ensure_deployment "${US_ACCOUNT}" "${US_DEPLOY}"
save_key "${FR_ACCOUNT}" "${OPERATOR_DIR}/docs/openai-fr-key.txt"
save_key "${US_ACCOUNT}" "${OPERATOR_DIR}/docs/openai-us-key.txt"
sync_manifests

cat <<EOF
[azure] Azure OpenAI FR/US ready.
  FR: ${FR_ACCOUNT}.openai.azure.com (${FR_LOCATION}) deployment ${FR_DEPLOY} -> ${MODEL}
  US: ${US_ACCOUNT}.openai.azure.com (${US_LOCATION}) deployment ${US_DEPLOY} -> ${MODEL}
  keys -> operateur/docs/openai-fr-key.txt, operateur/docs/openai-us-key.txt (gitignored)
Next: cd ${ENVOY_DIR} && ./deploy.sh up
EOF
