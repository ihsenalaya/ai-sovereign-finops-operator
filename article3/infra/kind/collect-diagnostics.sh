#!/usr/bin/env bash
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${KIND_DIR}/common.sh"
resolve_profile
require_tools kind kubectl docker python3 sha256sum
prepare_state_dir
cluster_exists || { echo "cluster does not exist: ${CLUSTER_NAME}" >&2; exit 1; }
verify_owned
make_kubeconfig
trap cleanup_kubeconfig EXIT

timestamp="$(date -u +%Y%m%dT%H%M%S.%NZ)"
output_root="${OUTPUT_DIR:-${ARTICLE3_DIR}/experiments/logs/kind-diagnostics}"
run_dir="${output_root}/${CLUSTER_NAME}/${timestamp}"
mkdir -p "${run_dir}/pod-logs"
chmod 700 "${run_dir}"

cp "${STATE_FILE}" "${run_dir}/ownership.json"
chmod 600 "${run_dir}/ownership.json"
kind version >"${run_dir}/kind-version.txt"
kubectl version -o yaml >"${run_dir}/kubernetes-version.yaml"
docker version --format '{{json .}}' >"${run_dir}/docker-version.json"
kubectl cluster-info dump --namespaces='kube-system' --output-directory="${run_dir}/cluster-info" >/dev/null 2>&1 || true
kubectl get nodes -o wide >"${run_dir}/nodes.txt"
kubectl get nodes -o yaml >"${run_dir}/nodes.yaml"
kubectl -n kube-system get deployments,statefulsets,daemonsets,replicasets,pods,services,endpoints,networkpolicies,jobs,cronjobs -o yaml \
  >"${run_dir}/objects-no-secrets.yaml"
kubectl -n kube-system get events --sort-by=.metadata.creationTimestamp >"${run_dir}/events.txt" || true
kubectl get crds -o name >"${run_dir}/crds.txt" || true
kubectl api-resources >"${run_dir}/api-resources.txt"
kubectl get --raw='/readyz?verbose' >"${run_dir}/readyz.txt" || true
kubectl get --raw='/livez?verbose' >"${run_dir}/livez.txt" || true

while IFS= read -r pod; do
  [[ -z "${pod}" ]] && continue
  safe="kube-system__${pod}"
  kubectl -n kube-system logs "${pod}" --all-containers=true --prefix=true \
    >"${run_dir}/pod-logs/${safe}.log" 2>&1 || true
  kubectl -n kube-system logs "${pod}" --all-containers=true --prefix=true --previous \
    >"${run_dir}/pod-logs/${safe}.previous.log" 2>&1 || true
done < <(kubectl -n kube-system get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')

(
  cd "${run_dir}"
  find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum >SHA256SUMS
)
echo "${run_dir}"
