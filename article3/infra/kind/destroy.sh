#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-article3-validation}"

if [[ "${CLUSTER_NAME}" == "article3-validation" ]] && ! kind get clusters | grep -Fxq "${CLUSTER_NAME}" && kind get clusters | grep -Fxq "gov-ar"; then
  CLUSTER_NAME="gov-ar"
fi

if kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  kind delete cluster --name "${CLUSTER_NAME}"
else
  echo "kind cluster ${CLUSTER_NAME} not present"
fi
