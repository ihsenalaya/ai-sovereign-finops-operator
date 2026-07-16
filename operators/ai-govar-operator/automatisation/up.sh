#!/usr/bin/env bash
# One-command kind deployment of the ai-govar-operator with all its
# dependencies: kind cluster, Prometheus + Grafana, the operator chart
# (manager + fail-closed change-approval webhooks), the GOV-AR admission
# service in explicit development in-memory mode, demo test applications
# (catalog + workload bindings), and the governed-admission dashboard.
#
# Usage:  ./up.sh            # full stack
#         SKIP_MONITORING=1 ./up.sh   # operator + test apps only
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require kind
require kubectl
require helm
require docker
require openssl

# 1. Cluster ------------------------------------------------------------------
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  log "kind cluster '${CLUSTER_NAME}' already exists; reusing it."
else
  log "creating kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --name "${CLUSTER_NAME}"
fi
kind export kubeconfig --name "${CLUSTER_NAME}"

# 2. Images ---------------------------------------------------------------------
log "building manager image ${IMAGE_NAME}:${IMAGE_TAG}..."
docker build -f "${DOCKERFILE}" -t "${IMAGE_NAME}:${IMAGE_TAG}" "${OPERATOR_DIR}"
kind load docker-image "${IMAGE_NAME}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

log "pulling and loading the published gov-ar-admission image..."
ADMISSION_TAG="0.5.12-article3.20260713"
docker pull "${ADMISSION_IMAGE}:${ADMISSION_TAG}"
kind load docker-image "${ADMISSION_IMAGE}:${ADMISSION_TAG}" --name "${CLUSTER_NAME}"
ADMISSION_SHA="$(docker inspect --format '{{index .RepoDigests 0}}' "${ADMISSION_IMAGE}:${ADMISSION_TAG}" | cut -d@ -f2)"
ADMISSION_SOFTWARE_SHA="${ADMISSION_SHA#sha256:}"
log "admission image digest: ${ADMISSION_SHA}"

# 3. Monitoring ------------------------------------------------------------------
if [ -z "${SKIP_MONITORING:-}" ]; then
  log "installing kube-prometheus-stack in namespace ${MONITORING_NAMESPACE}..."
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
  helm repo update prometheus-community >/dev/null
  helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    --namespace "${MONITORING_NAMESPACE}" --create-namespace \
    --set grafana.sidecar.dashboards.enabled=true \
    --set grafana.sidecar.dashboards.searchNamespace=ALL \
    --set grafana.adminPassword=admin \
    --wait --timeout 10m
  SM_ARGS=(--set metrics.serviceMonitor.enabled=true --set metrics.serviceMonitor.labels.release=monitoring)
else
  warn "SKIP_MONITORING set — Prometheus/Grafana not installed."
  SM_ARGS=()
fi

# 4. Identity secret (server-only master for the ext_proc loopback) ---------------
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
if ! kubectl -n "${NAMESPACE}" get secret govar-identity-master >/dev/null 2>&1; then
  log "creating the GOV-AR identity master secret..."
  kubectl -n "${NAMESPACE}" create secret generic govar-identity-master \
    --from-literal=GOVAR_IDENTITY_MASTER_SECRET="$(openssl rand -hex 32)"
fi

# 5. Operator chart -----------------------------------------------------------------
# devInMemory is the EXPLICIT development mode: single replica, no PostgreSQL.
# Production requires govArAdmission.postgres.enabled=true and an OTLP endpoint.
log "installing ai-govar-operator chart (release ${HELM_RELEASE})..."
helm upgrade --install "${HELM_RELEASE}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --set image.repository="${IMAGE_NAME}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=Never \
  --set govArAdmission.enabled=true \
  --set govArAdmission.devInMemory=true \
  --set govArAdmission.image.tag="${ADMISSION_TAG}" \
  --set govArAdmission.image.digest="${ADMISSION_SHA}" \
  --set govArAdmission.softwareSHA256="${ADMISSION_SOFTWARE_SHA}" \
  --set govArAdmission.identity.masterExistingSecret=govar-identity-master \
  "${SM_ARGS[@]}" \
  --wait --timeout 5m

# 6. Test applications ------------------------------------------------------------
log "applying test applications in namespace ${DEMO_NAMESPACE}..."
kubectl create namespace "${DEMO_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${DEMO_NAMESPACE}" apply -f "${HERE}/test-apps/"

# 7. Grafana dashboard ---------------------------------------------------------------
if [ -z "${SKIP_MONITORING:-}" ]; then
  log "importing the GOV-AR Grafana dashboard..."
  kubectl -n "${MONITORING_NAMESPACE}" create configmap govar-overview-dashboard \
    --from-file=govar-overview.json="${HERE}/dashboards/govar-overview.json" \
    --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n "${MONITORING_NAMESPACE}" label configmap govar-overview-dashboard \
    grafana_dashboard="1" --overwrite
fi

# 8. Smoke test -----------------------------------------------------------------------
log "running the admission smoke test..."
sleep 5
kubectl -n "${DEMO_NAMESPACE}" wait --for=condition=complete --timeout=120s job/govar-smoke-test \
  && log "smoke test PASSED" \
  || warn "smoke test did not complete; inspect: kubectl -n ${DEMO_NAMESPACE} logs job/govar-smoke-test"

# 9. Summary ----------------------------------------------------------------------------
kubectl -n "${DEMO_NAMESPACE}" get aiwb 2>/dev/null || true

cat <<SUMMARY

============================================================
 ai-govar-operator is up on kind cluster '${CLUSTER_NAME}'.

 Installed: manager (AIWorkloadBinding + fail-closed AIChangeRequest
 approval webhooks) + gov-ar-admission service in EXPLICIT development
 in-memory mode (single replica, no PostgreSQL).

 Verify:
   kubectl -n ${NAMESPACE} get deploy
   kubectl -n ${DEMO_NAMESPACE} get aiwb
   kubectl -n ${DEMO_NAMESPACE} logs job/govar-smoke-test

 Grafana (admin / admin):
   kubectl -n ${MONITORING_NAMESPACE} port-forward svc/monitoring-grafana 3000:80
   -> http://localhost:3000  (dashboard: "AI GOV-AR Operator — Governed Admission")

 Production notes:
   - PostgreSQL ledger: --set govArAdmission.postgres.enabled=true
     --set govArAdmission.postgres.existingSecret=<secret with DATABASE_URL>
   - OTLP tracing: --set govArAdmission.tracing.enabled=true
     --set govArAdmission.tracing.endpoint=http://otel-collector:4318
   - Envoy ext_proc integration: --set govArAdmission.extProc.envoyExample.enabled=true

 Tear down:  ./down.sh
============================================================
SUMMARY
