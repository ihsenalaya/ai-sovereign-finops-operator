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
  log "creating AKS ${CLUSTER} (Free tier) in ${LOCATION}..."
  az aks create -g "${RG}" -n "${CLUSTER}" \
    --tier free \
    --node-count "${SYS_COUNT}" \
    --node-vm-size "${SYS_VMSIZE}" \
    --nodepool-name system \
    --generate-ssh-keys \
    --network-plugin azure \
    "${ver_args[@]}" -o table
fi

az aks get-credentials -g "${RG}" -n "${CLUSTER}" --overwrite-existing
kubectl get nodes -o wide
log "AKS ready. Next: 02-add-gpu-nodepool.sh"
