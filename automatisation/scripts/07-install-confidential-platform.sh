#!/usr/bin/env bash
# Deploy the AI Confidential Governance Platform via Helm into the kind cluster.
# Requires: kind cluster running, images already loaded (run 02-build-load-image.sh first).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require helm
require kubectl

CHART_DIR="${REPO_ROOT}/charts/ai-confidential-governance-platform"
VALUES_KIND="${CHART_DIR}/values-kind.yaml"
RELEASE="${RELEASE_NAME:-ai-platform}"
NAMESPACE="${PLATFORM_NAMESPACE:-ai-platform}"
KUBE_CONTEXT="kind-${CLUSTER_NAME}"

log "applying CRDs..."
kubectl apply -f "${CHART_DIR}/crds/" --context "${KUBE_CONTEXT}"

log "creating namespace ${NAMESPACE}..."
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml \
  | kubectl apply -f - --context "${KUBE_CONTEXT}"

log "deploying ${RELEASE} from ${CHART_DIR}..."
helm upgrade --install "${RELEASE}" "${CHART_DIR}" \
  -f "${VALUES_KIND}" \
  --set images.tag="${IMAGE_TAG}" \
  --namespace "${NAMESPACE}" \
  --kube-context "${KUBE_CONTEXT}" \
  --timeout 3m \
  --wait

log "deployment complete — checking pods..."
kubectl get pods -n "${NAMESPACE}" --context "${KUBE_CONTEXT}"

log ""
log "Access the platform (run each in a separate terminal):"
log "  Platform UI:           kubectl port-forward svc/platform-ui 8090:80 -n ${NAMESPACE} --context ${KUBE_CONTEXT}"
log "  Platform API:          kubectl port-forward svc/platform-api 8083:8083 -n ${NAMESPACE} --context ${KUBE_CONTEXT}"
log "  Key-Release Gateway:   kubectl port-forward svc/key-release-gateway 8082:8082 -n ${NAMESPACE} --context ${KUBE_CONTEXT}"
log "  Operator metrics:      kubectl port-forward svc/governance-operator 8080:8080 -n ${NAMESPACE} --context ${KUBE_CONTEXT}"
log ""
log "  Platform UI: http://localhost:8090"
