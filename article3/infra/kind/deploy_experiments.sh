#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE_REPO="${IMAGE_REPO:-gov-ar-experiment}"
IMAGE_TAG="${IMAGE_TAG:-local}"
CLUSTER_NAME="${CLUSTER_NAME:-gov-ar}"
EXPERIMENT_COMMAND="${EXPERIMENT_COMMAND:-e0}"
RELEASE_NAME="${RELEASE_NAME:-gov-ar-experiment}"
NAMESPACE="${NAMESPACE:-gov-ar}"

cd "${ROOT_DIR}/.."

docker build -f article3/Dockerfile.experiment -t "${IMAGE_REPO}:${IMAGE_TAG}" .
kind load docker-image "${IMAGE_REPO}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

helm upgrade --install "${RELEASE_NAME}" "${ROOT_DIR}/infra/helm/gov-ar-experiment" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set image.repository="${IMAGE_REPO}" \
  --set image.tag="${IMAGE_TAG}" \
  --set experiment.command="${EXPERIMENT_COMMAND}"

kubectl -n "${NAMESPACE}" get jobs,pods
