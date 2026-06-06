#!/usr/bin/env bash
# Generic bounded-concurrency job runner. Runs labeled jobs in parallel, logs each
# to results/parallel/<label>.log, and prints a status summary with durations and
# exit codes. Used to overlap independent work (e.g. local OpenAI experiments while
# an Azure vLLM benchmark runs) so no time is wasted serially.
#
# Usage:
#   scripts/parallel.sh "label1::command one" "label2::command two" ...
#   MAX_PAR=4 scripts/parallel.sh ...        # cap concurrency (default: nproc)
#   scripts/parallel.sh                      # no args -> built-in demo job set
set -uo pipefail

# Run from the Go module root so `go ./...` paths resolve; keep logs under experimentation/results.
MODULE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${MODULE_ROOT}"
OUTDIR="experimentation/results/parallel"
mkdir -p "${OUTDIR}"
MAX_PAR="${MAX_PAR:-$( (nproc 2>/dev/null) || echo 4 )}"

# Default demo set if no jobs given: two quick, independent jobs (proves the runner).
if [ "$#" -eq 0 ]; then
  set -- \
    "build::go build ./..." \
    "router-tests::go test ./experimentation/internal/router/..." \
    "engine-tests::go test ./internal/costengine/... ./internal/budgetengine/... ./internal/sovereigntyengine/... ./internal/breakevenengine/..."
fi

declare -a LABELS PIDS
running=0

start_job() {
  local spec="$1" label cmd
  label="${spec%%::*}"; cmd="${spec#*::}"
  echo "[parallel] start  ${label}"
  ( /usr/bin/env bash -c "${cmd}" >"${OUTDIR}/${label}.log" 2>&1; echo $? >"${OUTDIR}/${label}.code" ) &
  LABELS+=("${label}"); PIDS+=("$!")
}

# Launch with bounded concurrency.
for spec in "$@"; do
  start_job "${spec}"
  running=$((running+1))
  if [ "${running}" -ge "${MAX_PAR}" ]; then
    wait -n 2>/dev/null || true
    running=$((running-1))
  fi
done
wait

echo
echo "[parallel] summary (logs in ${OUTDIR}/):"
fail=0
printf "  %-16s %-6s %s\n" "JOB" "EXIT" "LOG"
for label in "${LABELS[@]}"; do
  code="$(cat "${OUTDIR}/${label}.code" 2>/dev/null || echo '?')"
  [ "${code}" = "0" ] || fail=$((fail+1))
  printf "  %-16s %-6s %s\n" "${label}" "${code}" "${OUTDIR}/${label}.log"
done
echo
if [ "${fail}" -eq 0 ]; then echo "[parallel] ALL OK"; else echo "[parallel] ${fail} job(s) FAILED"; fi
exit "${fail}"
