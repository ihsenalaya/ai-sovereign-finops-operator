#!/usr/bin/env bash
# Shared configuration for the ai-govar-operator kind automation.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-govar-operator}"
NAMESPACE="${NAMESPACE:-govar-system}"
DEMO_NAMESPACE="${DEMO_NAMESPACE:-govar-demo}"
MONITORING_NAMESPACE="${MONITORING_NAMESPACE:-monitoring}"
IMAGE_NAME="${IMAGE_NAME:-govar-operator}"
IMAGE_TAG="${IMAGE_TAG:-dev}"
HELM_RELEASE="${HELM_RELEASE:-govar}"
# Published admission image (fail-closed digest pinning is kept even in dev).
ADMISSION_IMAGE="ghcr.io/ihsenalaya/ai-sovereign-finops-operator/gov-ar-admission"

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${HERE}/../../.." && pwd)"
OPERATOR_DIR="${REPO_ROOT}/operateur"
CHART_DIR="${HERE}/../chart/ai-govar-operator"
DOCKERFILE="${OPERATOR_DIR}/Dockerfile.govar-operator"

log()  { printf '\033[36m[govar-kind]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[govar-kind] WARN:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31m[govar-kind] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed"
}
