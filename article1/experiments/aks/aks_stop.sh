#!/usr/bin/env bash
# aks_stop.sh — scale the confidential pool to 0 to stop billing the expensive
# nodes, without destroying the cluster (keeps kubeconfig/state for a later run).
set -euo pipefail
RG="${RG:-rg-article1-confidential}"
CLUSTER="${CLUSTER:-aks-article1}"
POOL="${POOL:-conf}"
OUT_DIR="${OUT_DIR:-article1/results/raw/aks}"
mkdir -p "${OUT_DIR}"

if az aks nodepool show -g "${RG}" --cluster-name "${CLUSTER}" -n "${POOL}" >/dev/null 2>&1; then
  echo "== scaling confidential pool ${POOL} to 0"
  autoscaler="$(az aks nodepool show -g "${RG}" --cluster-name "${CLUSTER}" -n "${POOL}" --query enableAutoScaling -o tsv)"
  if [ "${autoscaler}" = "true" ] || [ "${autoscaler}" = "True" ]; then
    az aks nodepool update -g "${RG}" --cluster-name "${CLUSTER}" -n "${POOL}" \
      --disable-cluster-autoscaler | tee "${OUT_DIR}/aks-stop.log"
  fi
  az aks nodepool scale -g "${RG}" --cluster-name "${CLUSTER}" -n "${POOL}" --node-count 0 \
    | tee -a "${OUT_DIR}/aks-stop.log"
else
  echo "== no confidential pool '${POOL}' to stop (system-only or not created)" \
    | tee "${OUT_DIR}/aks-stop.log"
fi
echo "== aks_stop done"
