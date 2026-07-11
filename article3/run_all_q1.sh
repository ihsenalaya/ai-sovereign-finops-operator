#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

STATE_DIR="${ARTICLE3_STATE_DIR:-$ROOT/.codex_article3_recovery}"
RUN_ID="${ARTICLE3_RUN_ID:-q1-20260711}"
RUN_DIR="$STATE_DIR/$RUN_ID"
STATE_FILE="$RUN_DIR/state.tsv"
mkdir -p "$RUN_DIR"
touch "$STATE_FILE"

usage() {
  echo "usage: $0 [--resume] [--from PHASE] [--status]"
}

resume=false
from=""
status_only=false
while (($#)); do
  case "$1" in
    --resume) resume=true ;;
    --from) shift; from="${1:?phase required}" ;;
    --status) status_only=true ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
  shift
done

if $status_only; then
  column -t -s $'\t' "$STATE_FILE" 2>/dev/null || sed -n '1,240p' "$STATE_FILE"
  exit 0
fi

record() {
  local phase="$1" state="$2" detail="$3" now tmp
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  tmp="$(mktemp)"
  awk -F '\t' -v p="$phase" '$1 != p' "$STATE_FILE" > "$tmp"
  printf '%s\t%s\t%s\t%s\n' "$phase" "$state" "$now" "$detail" >> "$tmp"
  sort -t $'\t' -k1,1 "$tmp" > "$STATE_FILE"
  rm -f "$tmp"
}

is_done() {
  awk -F '\t' -v p="$1" '$1 == p && $2 == "passed" {found=1} END {exit !found}' "$STATE_FILE"
}

run_step() {
  local phase="$1"; shift
  local log="$RUN_DIR/${phase}.log"
  if $resume && is_done "$phase"; then
    echo "SKIP $phase (verified checkpoint)"
    return
  fi
  echo "RUN  $phase"
  record "$phase" running "$log"
  if "$@" > >(tee "$log") 2>&1; then
    sha256sum "$log" > "$log.sha256"
    record "$phase" passed "$log"
  else
    rc=$?
    sha256sum "$log" > "$log.sha256"
    record "$phase" failed "exit=$rc log=$log"
    python3 article3/tools/q1_gate.py --write-status >/dev/null 2>&1 || true
    return "$rc"
  fi
}

phase_a() {
  python3 article3/tools/recompute_prior_audit.py
  python3 article3/tools/operator_inventory.py
}
phase_b() { python3 article3/tools/validate_literature.py; }
phase_c() { bash article3/formal/check.sh; }
phase_d() { bash article3/tools/run_test_matrix.sh; }
phase_e() { bash article3/infra/kind/validate_release.sh; }
phase_f() { python3 article3/datasets/prepare.py --verify; }
phase_g() { bash article3/baselines/validate.sh; }
phase_h() { python3 article3/experiments/pilot_and_freeze.py; }
phase_i() { bash article3/experiments/orchestrator/run_frozen.sh --resume; }
phase_j() { python3 article3/analysis/run_final.py --from-raw; }
phase_k() { bash article3/manuscript/build_release.sh; }
phase_release() {
  python3 article3/tools/q1_gate.py --strict --write-status
  bash article3/reproduction/clean_checkout.sh
  python3 article3/tools/q1_gate.py --strict --write-status
}

phases=(phase_a phase_b phase_c phase_d phase_e phase_f phase_g phase_h phase_i phase_j phase_k phase_release)
started=false
[[ -z "$from" ]] && started=true
for phase in "${phases[@]}"; do
  if ! $started && [[ "$phase" == "$from" ]]; then started=true; fi
  $started || continue
  run_step "$phase" "$phase"
done

python3 article3/tools/q1_gate.py --strict --write-status
