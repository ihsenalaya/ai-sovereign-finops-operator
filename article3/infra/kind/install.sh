#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE_REPO="${IMAGE_REPO:-gov-ar-experiment}"
IMAGE_TAG="${IMAGE_TAG:-local}"
CLUSTER_NAME="${CLUSTER_NAME:-article3-validation}"
EXPERIMENT_COMMAND="${EXPERIMENT_COMMAND:-e0}"
RELEASE_NAME="${RELEASE_NAME:-gov-ar-experiment}"
NAMESPACE="${NAMESPACE:-gov-ar}"
KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"

if [[ "${CLUSTER_NAME}" == "article3-validation" ]] && ! kind get clusters | grep -Fxq "${CLUSTER_NAME}" && kind get clusters | grep -Fxq "gov-ar"; then
  CLUSTER_NAME="gov-ar"
  KUBECONFIG_CONTEXT="kind-${CLUSTER_NAME}"
fi

if ! kind get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  echo "kind cluster ${CLUSTER_NAME} does not exist" >&2
  exit 1
fi

cd "${ROOT_DIR}/.."

docker build -f article3/Dockerfile.experiment -t "${IMAGE_REPO}:${IMAGE_TAG}" .
kind load docker-image "${IMAGE_REPO}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

helm upgrade --install "${RELEASE_NAME}" "${ROOT_DIR}/infra/helm/gov-ar-experiment" \
  --kube-context "${KUBECONFIG_CONTEXT}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set image.repository="${IMAGE_REPO}" \
  --set image.tag="${IMAGE_TAG}" \
  --set experiment.command="${EXPERIMENT_COMMAND}"

kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" get jobs,pods -o wide
