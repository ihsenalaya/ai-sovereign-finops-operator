#!/usr/bin/env bash
# Create the Key Vault (RBAC mode) and store the OpenAI API key (and optional HF
# token) as secrets. The key is never printed; it is extracted to a 0600 temp
# file and uploaded via --file, then shredded.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

az group create -n "${RG}" -l "${LOCATION}" -o none

if az keyvault show -n "${KEYVAULT}" -g "${RG}" >/dev/null 2>&1; then
  log "Key Vault ${KEYVAULT} exists; reusing."
else
  log "creating Key Vault ${KEYVAULT} (RBAC) in ${LOCATION}..."
  az keyvault create -n "${KEYVAULT}" -g "${RG}" -l "${LOCATION}" \
    --enable-rbac-authorization true -o none
fi

# Grant the current user secret read/write (RBAC).
oid="$(az ad signed-in-user show --query id -o tsv 2>/dev/null || true)"
kvid="$(az keyvault show -n "${KEYVAULT}" -g "${RG}" --query id -o tsv)"
if [ -n "${oid}" ]; then
  az role assignment create --role "Key Vault Secrets Officer" \
    --assignee-object-id "${oid}" --assignee-principal-type User \
    --scope "${kvid}" -o none 2>/dev/null || true
  log "granted Key Vault Secrets Officer to current user (propagation ~1-2 min)."
fi

# Upload the OpenAI key without echoing it.
if [ -f "${OPENAI_KEY_FILE}" ]; then
  tmp="$(mktemp)"; chmod 600 "${tmp}"
  sed -n 's/.*\(sk-[A-Za-z0-9_-]*\).*/\1/p' "${OPENAI_KEY_FILE}" | head -1 > "${tmp}"
  [ -s "${tmp}" ] || tr -d '\r\n' < "${OPENAI_KEY_FILE}" > "${tmp}"
  for i in 1 2 3 4 5; do
    if az keyvault secret set --vault-name "${KEYVAULT}" --name "${KV_OPENAI_SECRET}" \
        --file "${tmp}" -o none 2>/dev/null; then
      log "stored secret '${KV_OPENAI_SECRET}' in ${KEYVAULT}."
      break
    fi
    warn "secret set failed (RBAC may still be propagating); retry ${i}/5..."; sleep 20
  done
  shred -u "${tmp}" 2>/dev/null || rm -f "${tmp}"
else
  warn "OpenAI key file not found at ${OPENAI_KEY_FILE}; skipping openai secret."
fi

# Optional HF token (for gated vLLM models).
if [ -n "${HF_TOKEN:-}" ]; then
  az keyvault secret set --vault-name "${KEYVAULT}" --name "${KV_HF_SECRET}" --value "${HF_TOKEN}" -o none \
    && log "stored secret '${KV_HF_SECRET}'."
fi

log "Key Vault ready: ${KEYVAULT}. Secrets: $(az keyvault secret list --vault-name "${KEYVAULT}" --query "[].name" -o tsv 2>/dev/null | tr '\n' ' ' || echo '(pending RBAC)')"
