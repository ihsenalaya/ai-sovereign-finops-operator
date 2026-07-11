#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
STATE_DIR="${ROOT_DIR}/experiments/logs/runs/${RUN_ID}"
mkdir -p "${STATE_DIR}"
STATE_FILE="${STATE_DIR}/state.tsv"
touch "${STATE_FILE}"

set_state() {
  local task="$1" state="$2"
  awk -F '\t' -v t="${task}" '$1 != t' "${STATE_FILE}" > "${STATE_FILE}.tmp"
  printf '%s\t%s\t%s\n' "${task}" "${state}" "$(date -u +%FT%TZ)" >> "${STATE_FILE}.tmp"
  mv "${STATE_FILE}.tmp" "${STATE_FILE}"
}

run_task() {
  local task="$1" cmd="$2" attempts=0 max_attempts="${MAX_ATTEMPTS:-3}"
  if awk -F '\t' -v t="${task}" '$1 == t && ($2 == "PASSED" || $2 == "COMPLETE") {found=1} END {exit !found}' "${STATE_FILE}"; then
    echo "[run_all] ${task}: already complete, resuming"
    return 0
  fi
  while (( attempts < max_attempts )); do
    attempts=$((attempts + 1))
    set_state "${task}" RUNNING
    echo "[run_all] ${task}: attempt ${attempts}/${max_attempts}"
    if bash -lc "${cmd}" >"${STATE_DIR}/${task}.log" 2>&1; then
      set_state "${task}" PASSED
      return 0
    fi
    set_state "${task}" FAILED_RETRYABLE
    (( attempts < max_attempts )) && sleep $((attempts * 2))
  done
  return 1
}

echo "[run_all] running GOV-AR experiment suite from ${ROOT_DIR}"

commands=(
  "go run ./cmd/experiment e0"
  "go run ./cmd/experiment e1"
  "go run ./cmd/experiment e2"
  "go run ./cmd/experiment e3"
  "go run ./cmd/experiment e4"
  "go run ./cmd/experiment e5"
  "go run ./cmd/experiment e7"
  "go run ./cmd/experiment e1-campaign"
  "go run ./cmd/experiment e2-campaign"
  "go run ./cmd/experiment e1-matrix"
  "go run ./cmd/experiment e2-matrix"
  "go run ./cmd/experiment e1-compare"
  "go run ./cmd/experiment e2-compare"
)

for index in "${!commands[@]}"; do
  cmd="${commands[$index]}"
  run_task "$(printf 'step_%02d' "$((index + 1))")" "${cmd}"
done

run_task verify_outputs "bash experiments/orchestrator/verify_outputs.sh"
sha256sum "${STATE_FILE}" > "${STATE_FILE}.sha256"

echo "[run_all] completed successfully"
