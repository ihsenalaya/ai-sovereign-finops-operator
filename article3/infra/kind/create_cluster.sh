#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-gov-ar}"

kind create cluster --name "${CLUSTER_NAME}" --config "${ROOT_DIR}/infra/kind/cluster.yaml"
kubectl cluster-info --context "kind-${CLUSTER_NAME}"
