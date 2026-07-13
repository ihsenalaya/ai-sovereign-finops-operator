#!/usr/bin/env bash
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${KIND_DIR}/common.sh"
resolve_profile
require_tools kind kubectl docker python3
prepare_state_dir
cluster_exists || { echo "cluster does not exist: ${CLUSTER_NAME}" >&2; exit 1; }
verify_owned
make_kubeconfig
CHECK_NAMESPACE="article3-cni-check"
cleanup() {
  kubectl delete namespace "${CHECK_NAMESPACE}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  cleanup_kubeconfig
}
trap cleanup EXIT

case "${PROFILE}" in dev) expected_nodes=2 ;; validation) expected_nodes=3 ;; performance) expected_nodes=4 ;; esac
actual_nodes="$(kubectl get nodes --no-headers | wc -l | tr -d ' ')"
[[ "${actual_nodes}" == "${expected_nodes}" ]] || {
  echo "node count mismatch: expected ${expected_nodes}, got ${actual_nodes}" >&2; exit 1;
}
[[ "$(kubectl version -o json | python3 -c 'import json,sys; print(json.load(sys.stdin)["serverVersion"]["gitVersion"])')" == v1.35.0 ]] || {
  echo "Kubernetes server version is not pinned v1.35.0" >&2; exit 1;
}
kubectl wait --for=condition=Ready nodes --all --timeout=3m
kubectl get --raw='/readyz?verbose' >/dev/null
kubectl get --raw='/livez?verbose' >/dev/null

if [[ "$(profile_cni)" == calico ]]; then
  kubectl -n kube-system rollout status daemonset/calico-node --timeout=3m
  kubectl -n kube-system rollout status deployment/calico-kube-controllers --timeout=3m
  mapfile -t images < <(kubectl -n kube-system get daemonset/calico-node deployment/calico-kube-controllers \
    -o jsonpath='{range .items[*].spec.template.spec.initContainers[*]}{.image}{"\n"}{end}{range .items[*].spec.template.spec.containers[*]}{.image}{"\n"}{end}' | sort -u)
  allowed=(
    "quay.io/calico/cni@${CALICO_CNI_DIGEST}"
    "quay.io/calico/node@${CALICO_NODE_DIGEST}"
    "quay.io/calico/kube-controllers@${CALICO_CONTROLLERS_DIGEST}"
  )
  for image in "${images[@]}"; do
    [[ -z "${image}" ]] && continue
    printf '%s\n' "${allowed[@]}" | grep -Fxq -- "${image}" || {
      echo "unpinned or unexpected Calico image: ${image}" >&2; exit 1;
    }
  done
fi

# Functional standard Kubernetes NetworkPolicy test. A non-enforcing CNI fails
# because the second wget remains successful after the egress deny is applied.
kubectl delete namespace "${CHECK_NAMESPACE}" --ignore-not-found --wait=true >/dev/null
kubectl create namespace "${CHECK_NAMESPACE}" >/dev/null
BUSYBOX_IMAGE="docker.io/library/busybox@sha256:9532d8c39891ca2ecde4d30d7710e01fb739c87a8b9299685c63704296b16028"
kubectl -n "${CHECK_NAMESPACE}" run server --image="${BUSYBOX_IMAGE}" --labels=app=server \
  --command -- sh -c 'mkdir -p /www; echo article3-network-policy-ok >/www/index.html; exec httpd -f -p 8080 -h /www'
kubectl -n "${CHECK_NAMESPACE}" expose pod server --name=server --port=8080 --target-port=8080 >/dev/null
kubectl -n "${CHECK_NAMESPACE}" run client --image="${BUSYBOX_IMAGE}" --labels=app=client \
  --command -- sh -c 'exec sleep 3600'
kubectl -n "${CHECK_NAMESPACE}" wait --for=condition=Ready pod/server pod/client --timeout=3m
service_ip="$(kubectl -n "${CHECK_NAMESPACE}" get service server -o jsonpath='{.spec.clusterIP}')"
kubectl -n "${CHECK_NAMESPACE}" exec client -- wget -q -T 3 -O - "http://${service_ip}:8080" | grep -Fxq article3-network-policy-ok
kubectl -n "${CHECK_NAMESPACE}" apply -f - >/dev/null <<'YAML'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-client-egress
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes:
    - Egress
  egress: []
YAML
denied=false
for _ in $(seq 1 15); do
  if ! kubectl -n "${CHECK_NAMESPACE}" exec client -- wget -q -T 1 -O - "http://${service_ip}:8080" >/dev/null 2>&1; then
    denied=true
    break
  fi
  sleep 1
done
[[ "${denied}" == true ]] || { echo "NetworkPolicy deny was not enforced" >&2; exit 1; }
kubectl -n "${CHECK_NAMESPACE}" delete networkpolicy deny-client-egress --wait=true >/dev/null
allowed_again=false
for _ in $(seq 1 15); do
  if kubectl -n "${CHECK_NAMESPACE}" exec client -- wget -q -T 2 -O - "http://${service_ip}:8080" 2>/dev/null | grep -Fxq article3-network-policy-ok; then
    allowed_again=true
    break
  fi
  sleep 1
done
[[ "${allowed_again}" == true ]] || { echo "connectivity did not recover after policy deletion" >&2; exit 1; }

echo "Kind profile validated: profile=${PROFILE} cluster=${CLUSTER_NAME} nodes=${actual_nodes} cni=$(profile_cni) network_policy=PASS"
