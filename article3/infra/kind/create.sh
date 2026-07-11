#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-article3-validation}"
CONFIG_PATH="${CONFIG_PATH:-${ROOT_DIR}/infra/kind/cluster.yaml}"
KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"

if [[ "${CLUSTER_NAME}" == "article3-validation" ]] && kind get clusters | grep -Fxq "gov-ar"; then
  CLUSTER_NAME="gov-ar"
  KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"
fi

if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  echo "kind cluster ${CLUSTER_NAME} already exists"
else
  kind create cluster --name "${CLUSTER_NAME}" --config "${CONFIG_PATH}"
fi

kubectl cluster-info --context "${KUBECONFIG_CONTEXT}"
kubectl --context "${KUBECONFIG_CONTEXT}" get nodes -o wide
