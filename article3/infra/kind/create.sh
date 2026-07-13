#!/usr/bin/env bash
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${KIND_DIR}/common.sh"
resolve_profile
require_tools kind kubectl docker python3 sha256sum curl sed timeout
prepare_state_dir

created=false
KUBECONFIG_FILE=""
calico_raw=""
calico_pinned=""
cleanup() {
  local rc=$?
  cleanup_kubeconfig
  rm -f -- "${calico_raw:-}" "${calico_pinned:-}"
  if ((rc != 0)) && [[ "${created}" == true ]]; then
    echo "creation failed; deleting only newly-created ${CLUSTER_NAME}" >&2
    if timeout 90s kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 && ! cluster_exists; then
      rm -f -- "${STATE_FILE}"
    else
      echo "failed cluster cleanup did not complete; retaining ownership record ${STATE_FILE}" >&2
    fi
  fi
  exit "${rc}"
}
trap cleanup EXIT

if cluster_exists; then
  verify_owned
  echo "owned cluster ${CLUSTER_NAME} already exists; creation is idempotent"
  make_kubeconfig
  if [[ "$(profile_cni)" == calico ]]; then
    kubectl -n kube-system get daemonset/calico-node >/dev/null 2>&1 && \
      kubectl -n kube-system get deployment/calico-kube-controllers >/dev/null 2>&1 || {
        echo "owned cluster bootstrap is incomplete; diagnose and reset ${CLUSTER_NAME}" >&2
        exit 2
      }
  fi
else
  [[ ! -e "${STATE_FILE}" && ! -L "${STATE_FILE}" ]] || {
    echo "refusing stale ownership record for absent cluster ${CLUSTER_NAME}" >&2
    exit 2
  }
  [[ -f "${CONFIG_PATH}" ]] || { echo "missing profile manifest: ${CONFIG_PATH}" >&2; exit 1; }
  require_profile_capacity
  KUBECONFIG_FILE="$(mktemp "${TMPDIR:-/tmp}/article3-kind-kubeconfig.XXXXXX")"
  chmod 600 "${KUBECONFIG_FILE}"
  export KUBECONFIG="${KUBECONFIG_FILE}"
  kind create cluster --name "${CLUSTER_NAME}" --image "${NODE_IMAGE}" \
    --config "${CONFIG_PATH}" --kubeconfig "${KUBECONFIG_FILE}"
  created=true

  cni="$(profile_cni)"
  # Establish the destructive-operation capability record immediately after
  # Kind returns the exact node set. If later bootstrap or cleanup fails, the
  # incomplete cluster remains attributable and can be removed safely.
  python3 "${KIND_DIR}/ownership.py" create \
    --state "${STATE_FILE}" --cluster "${CLUSTER_NAME}" --profile "${PROFILE}" \
    --node-image "${NODE_IMAGE}" --config "${CONFIG_PATH}" --cni "${cni}" \
    --calico-version "${CALICO_VERSION}" --calico-sha256 "${CALICO_PINNED_SHA256}"
  verify_owned
  if [[ "${cni}" == calico ]]; then
    calico_raw="$(mktemp "${TMPDIR:-/tmp}/article3-calico-raw.XXXXXX")"
    calico_pinned="$(mktemp "${TMPDIR:-/tmp}/article3-calico-pinned.XXXXXX")"
    curl --fail --location --proto '=https' --tlsv1.2 --retry 4 --retry-all-errors \
      --output "${calico_raw}" "${CALICO_URL}"
    printf '%s  %s\n' "${CALICO_RAW_SHA256}" "${calico_raw}" | sha256sum --check --strict
    sed \
      -e "s#quay.io/calico/cni:v${CALICO_VERSION}#quay.io/calico/cni@${CALICO_CNI_DIGEST}#g" \
      -e "s#quay.io/calico/node:v${CALICO_VERSION}#quay.io/calico/node@${CALICO_NODE_DIGEST}#g" \
      -e "s#quay.io/calico/kube-controllers:v${CALICO_VERSION}#quay.io/calico/kube-controllers@${CALICO_CONTROLLERS_DIGEST}#g" \
      "${calico_raw}" >"${calico_pinned}"
    printf '%s  %s\n' "${CALICO_PINNED_SHA256}" "${calico_pinned}" | sha256sum --check --strict
    # Pre-pull the exact digest-pinned images through CRI before applying the
    # manifest. CRI-aware loading avoids containerd's stale checkpoint-image
    # metadata for aliases created by raw `ctr images import` and is especially
    # important for the four-node performance profile.
    calico_images=(
      "quay.io/calico/cni@${CALICO_CNI_DIGEST}"
      "quay.io/calico/node@${CALICO_NODE_DIGEST}"
      "quay.io/calico/kube-controllers@${CALICO_CONTROLLERS_DIGEST}"
    )
    for image in "${calico_images[@]}"; do
      while IFS= read -r node; do
        pulled=false
        for attempt in 1 2 3 4; do
          if docker exec "${node}" crictl pull "${image}"; then pulled=true; break; fi
          sleep "$((1 << (attempt - 1)))"
        done
        [[ "${pulled}" == true ]] || {
          echo "bounded CRI pre-pull failed on ${node}: ${image}" >&2; exit 1;
        }
        docker exec "${node}" crictl inspecti "${image}" | python3 -c '
import json, sys
expected = sys.argv[1]
data = json.load(sys.stdin)
digests = data["status"]["repoDigests"]
if expected not in digests or any("/import-" in item for item in digests):
    raise SystemExit(f"CRI digest binding mismatch: expected={expected} actual={digests}")
' "${image}"
      done < <(kind get nodes --name "${CLUSTER_NAME}")
    done
    kubectl apply --server-side=true --field-manager=article3-kind-bootstrap -f "${calico_pinned}"
    kubectl -n kube-system rollout status daemonset/calico-node --timeout=8m
    kubectl -n kube-system rollout status deployment/calico-kube-controllers --timeout=8m
  fi

  verify_owned
fi

kubectl wait --for=condition=Ready nodes --all --timeout=5m
kubectl cluster-info
kubectl get nodes -o wide
