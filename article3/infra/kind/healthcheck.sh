#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-article3-validation}"
NAMESPACE="${NAMESPACE:-gov-ar}"
RELEASE_NAME="${RELEASE_NAME:-gov-ar-experiment}"
KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"

if [[ "${CLUSTER_NAME}" == "article3-validation" ]] && ! kind get clusters | grep -Fxq "${CLUSTER_NAME}" && kind get clusters | grep -Fxq "gov-ar"; then
  CLUSTER_NAME="gov-ar"
  KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"
fi

if ! kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  echo "kind cluster ${CLUSTER_NAME} does not exist" >&2
  exit 1
fi

kubectl --context "${KUBECONFIG_CONTEXT}" get nodes
kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" get all
kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" describe job "${RELEASE_NAME}" || true
kubectl --context "${KUBECONFIG_CONTEXT}" get --raw='/readyz?verbose' >/dev/null
kubectl --context "${KUBECONFIG_CONTEXT}" get --raw='/livez?verbose' >/dev/null
