#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/experiments/logs/kind-diagnostics}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
CLUSTER_NAME="${CLUSTER_NAME:-article3-validation}"
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

mkdir -p "${OUTPUT_DIR}/${TIMESTAMP}"

kubectl --context "${KUBECONFIG_CONTEXT}" get nodes -o wide > "${OUTPUT_DIR}/${TIMESTAMP}/nodes.txt"
kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" get all -o wide > "${OUTPUT_DIR}/${TIMESTAMP}/workloads.txt"
kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" describe jobs,pods > "${OUTPUT_DIR}/${TIMESTAMP}/describe.txt"
job_names="$(
  kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" get jobs -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true
)"
if [[ -n "${job_names}" ]]; then
  : > "${OUTPUT_DIR}/${TIMESTAMP}/job-logs.txt"
  while IFS= read -r job_name; do
    [[ -z "${job_name}" ]] && continue
    {
      echo "## ${job_name}"
      kubectl --context "${KUBECONFIG_CONTEXT}" -n "${NAMESPACE}" logs "job/${job_name}" --all-containers=true
      echo
    } >> "${OUTPUT_DIR}/${TIMESTAMP}/job-logs.txt" || true
  done <<< "${job_names}"
else
  echo "No jobs found in namespace ${NAMESPACE}" > "${OUTPUT_DIR}/${TIMESTAMP}/job-logs.txt"
fi
helm list -n "${NAMESPACE}" > "${OUTPUT_DIR}/${TIMESTAMP}/helm-list.txt" || true

echo "${OUTPUT_DIR}/${TIMESTAMP}"
