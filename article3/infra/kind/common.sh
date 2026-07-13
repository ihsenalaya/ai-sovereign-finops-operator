#!/usr/bin/env bash
# Shared safety boundary for Article 3 Kind automation. Source this file.
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTICLE3_DIR="$(cd "${KIND_DIR}/../.." && pwd)"
REPO_ROOT="$(cd "${ARTICLE3_DIR}/.." && pwd)"
# WSL/Windows-backed worktrees cannot reliably preserve POSIX mode 0600. Keep
# the destructive-operation capability record on the Linux state filesystem.
STATE_DIR="${ARTICLE3_KIND_STATE_DIR:-${XDG_STATE_HOME:-${HOME}/.local/state}/article3-q1-recovery/kind}"
NODE_IMAGE="kindest/node:v1.35.0@sha256:452d707d4862f52530247495d180205e029056831160e22870e37e3f6c1ac31f"
CALICO_VERSION="3.32.1"
CALICO_URL="https://raw.githubusercontent.com/projectcalico/calico/v${CALICO_VERSION}/manifests/calico.yaml"
CALICO_RAW_SHA256="a1df919d9721cf667accdc3e72848911b0cb25cfab7d2478ad0c996302c95744"
CALICO_PINNED_SHA256="9af3df3e4f49e50befc3f9fb79778f41b1496c8e7ef5778f413e60d25ddf16f3"
CALICO_CNI_DIGEST="sha256:bb1567e3ed81e2e8414e9a68f186e1f7ffd4067a4871a9ae90896793af0190dd"
CALICO_NODE_DIGEST="sha256:7f874b3f0b540c2b523aea9961ef5e2f43b0af9056a47874c916d6cf348168d3"
CALICO_CONTROLLERS_DIGEST="sha256:18008f781c869376dbbc4dfb1ffe3afb46f7897887d4f20e080c420ac44a6612"

resolve_profile() {
  PROFILE="${PROFILE:-validation}"
  case "${PROFILE}" in
    dev|validation|performance) ;;
    *) echo "invalid profile ${PROFILE}; expected dev, validation, or performance" >&2; return 2 ;;
  esac
  local exact_name="article3-${PROFILE}"
  CLUSTER_NAME="${CLUSTER_NAME:-${exact_name}}"
  [[ "${CLUSTER_NAME}" == "${exact_name}" ]] || {
    echo "profile ${PROFILE} is bound to exact cluster name ${exact_name}; refusing ${CLUSTER_NAME}" >&2
    return 2
  }
  CONFIG_PATH="${KIND_DIR}/cluster-${PROFILE}.yaml"
  STATE_FILE="${STATE_DIR}/${CLUSTER_NAME}.json"
}

prepare_state_dir() {
  [[ ! -L "${STATE_DIR}" ]] || {
    echo "refusing symlinked Kind ownership state directory: ${STATE_DIR}" >&2
    return 2
  }
  mkdir -p -- "${STATE_DIR}"
  chmod 700 -- "${STATE_DIR}"
  local probe
  probe="$(mktemp "${STATE_DIR}/.posix-mode-check.XXXXXX")"
  chmod 600 -- "${probe}"
  if [[ "$(stat -c '%a' -- "${probe}")" != 600 ]]; then
    rm -f -- "${probe}"
    echo "Kind ownership state filesystem does not preserve POSIX mode 0600: ${STATE_DIR}" >&2
    return 2
  fi
  rm -f -- "${probe}"
}

require_tools() {
  local tool
  for tool in "$@"; do
    command -v "${tool}" >/dev/null || { echo "required tool not found: ${tool}" >&2; return 1; }
  done
}

cluster_exists() {
  kind get clusters 2>/dev/null | grep -Fxq -- "${CLUSTER_NAME}"
}

verify_owned() {
  [[ -f "${STATE_FILE}" && ! -L "${STATE_FILE}" ]] || {
    echo "refusing unowned cluster ${CLUSTER_NAME}: valid Article 3 state record absent" >&2
    return 2
  }
  python3 "${KIND_DIR}/ownership.py" verify \
    --state "${STATE_FILE}" --cluster "${CLUSTER_NAME}" --profile "${PROFILE}" \
    --node-image "${NODE_IMAGE}" --config "${CONFIG_PATH}" --cni "$(profile_cni)" \
    --calico-version "${CALICO_VERSION}" --calico-sha256 "${CALICO_PINNED_SHA256}"
}

make_kubeconfig() {
  KUBECONFIG_FILE="$(mktemp "${TMPDIR:-/tmp}/article3-kind-kubeconfig.XXXXXX")"
  chmod 600 "${KUBECONFIG_FILE}"
  kind get kubeconfig --name "${CLUSTER_NAME}" >"${KUBECONFIG_FILE}"
  export KUBECONFIG="${KUBECONFIG_FILE}"
}

cleanup_kubeconfig() {
  if [[ -n "${KUBECONFIG_FILE:-}" ]]; then rm -f -- "${KUBECONFIG_FILE}"; fi
}

profile_cni() {
  if [[ "${PROFILE}" == dev ]]; then printf '%s\n' kindnet; else printf '%s\n' calico; fi
}

require_profile_capacity() {
  [[ "${PROFILE}" == performance ]] || return 0
  local available_kib minimum_kib
  available_kib="${ARTICLE3_KIND_AVAILABLE_KIB_OVERRIDE:-$(
    awk '/^MemAvailable:/ {mem=$2} /^SwapFree:/ {swap=$2} END {print mem+swap}' /proc/meminfo
  )}"
  minimum_kib="${ARTICLE3_KIND_MIN_AVAILABLE_KIB:-2097152}"
  [[ "${available_kib}" =~ ^[0-9]+$ && "${minimum_kib}" =~ ^[0-9]+$ ]] || {
    echo "invalid performance capacity preflight values" >&2; return 2;
  }
  ((available_kib >= minimum_kib)) || {
    echo "refusing performance cluster: available memory plus free swap ${available_kib} KiB is below ${minimum_kib} KiB safety floor" >&2
    return 2
  }
}
