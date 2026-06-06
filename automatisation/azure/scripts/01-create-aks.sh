#!/usr/bin/env bash
# Create the AKS cluster (Free control-plane tier) with a small CPU system pool.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az

az group create -n "${RG}" -l "${LOCATION}" -o table

ver_args=()
[ -n "${K8S_VERSION}" ] && ver_args=(--kubernetes-version "${K8S_VERSION}")

if az aks show -g "${RG}" -n "${CLUSTER}" >/dev/null 2>&1; then
  log "cluster ${CLUSTER} already exists; reusing."
else
  log "creating AKS ${CLUSTER} (Free tier) in ${LOCATION} with Key Vault CSI + workload identity..."
  az aks create -g "${RG}" -n "${CLUSTER}" \
    --tier free \
    --node-count "${SYS_COUNT}" \
    --node-vm-size "${SYS_VMSIZE}" \
    --nodepool-name system \
    --generate-ssh-keys \
    --network-plugin azure \
    --enable-managed-identity \
    --enable-addons azure-keyvault-secrets-provider \
    --enable-oidc-issuer \
    --enable-workload-identity \
    "${ver_args[@]}" -o table
fi

# Grant the cluster's Key Vault CSI identity read access to the vault (if KV exists).
if az keyvault show -n "${KEYVAULT}" -g "${RG}" >/dev/null 2>&1; then
  kvcsi="$(az aks show -g "${RG}" -n "${CLUSTER}" \
    --query "addonProfiles.azureKeyvaultSecretsProvider.identity.objectId" -o tsv 2>/dev/null || true)"
  kvid="$(az keyvault show -n "${KEYVAULT}" -g "${RG}" --query id -o tsv)"
  if [ -n "${kvcsi}" ] && [ -n "${kvid}" ]; then
    az role assignment create --role "Key Vault Secrets User" \
      --assignee-object-id "${kvcsi}" --assignee-principal-type ServicePrincipal \
      --scope "${kvid}" -o none 2>/dev/null || true
    log "granted Key Vault Secrets User to the AKS CSI identity."
  fi
fi

az aks get-credentials -g "${RG}" -n "${CLUSTER}" --overwrite-existing
kubectl get nodes -o wide
log "AKS ready. Next: 02-add-gpu-nodepool.sh"
