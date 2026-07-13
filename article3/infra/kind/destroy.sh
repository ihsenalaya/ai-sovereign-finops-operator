#!/usr/bin/env bash
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${KIND_DIR}/common.sh"
resolve_profile
require_tools kind docker python3
prepare_state_dir

if cluster_exists; then
  verify_owned
  if kind delete cluster --name "${CLUSTER_NAME}" && ! cluster_exists; then
    rm -f -- "${STATE_FILE}"
    echo "destroyed owned cluster ${CLUSTER_NAME}"
  else
    echo "cluster deletion did not produce a verified-absent state; retaining ownership record ${STATE_FILE}" >&2
    exit 2
  fi
else
  [[ ! -e "${STATE_FILE}" && ! -L "${STATE_FILE}" ]] || {
    echo "cluster absent but stale ownership state exists; refusing implicit cleanup: ${STATE_FILE}" >&2
    exit 2
  }
  echo "owned cluster ${CLUSTER_NAME} is already absent"
fi
