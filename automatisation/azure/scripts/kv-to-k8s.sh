#!/usr/bin/env bash
# Pull a secret from Key Vault and create/update a Kubernetes Secret. Reliable,
# CSI-driver-independent way to inject secrets (e.g. HF token into the vllm ns).
# Usage: kv-to-k8s.sh <kv-secret-name> <k8s-namespace> <k8s-secret-name> <key>
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
require az
require kubectl

KVSECRET="${1:?kv secret name}"; NS="${2:?namespace}"; K8SSECRET="${3:?k8s secret name}"; KEY="${4:?data key}"

tmp="$(mktemp)"; chmod 600 "${tmp}"
trap 'shred -u "${tmp}" 2>/dev/null || rm -f "${tmp}"' EXIT
az keyvault secret download --vault-name "${KEYVAULT}" --name "${KVSECRET}" --file "${tmp}"

kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic "${K8SSECRET}" -n "${NS}" \
  --from-file="${KEY}=${tmp}" --dry-run=client -o yaml | kubectl apply -f -
log "k8s secret ${NS}/${K8SSECRET} (key ${KEY}) synced from KV ${KEYVAULT}/${KVSECRET}."
