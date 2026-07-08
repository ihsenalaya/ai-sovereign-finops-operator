#!/usr/bin/env bash
# Run non-paid kind scalability for Article 1.
set -euo pipefail

export ENV_NAME="${ENV_NAME:-kind-live-simulated}"
export OUT_DIR="${OUT_DIR:-article1/results/raw/kind}"
export CSV="${CSV:-${OUT_DIR}/scalability_kind.csv}"
export SIZES="${SIZES:-10 50 100}"
export N_RUNS_SCALABILITY="${N_RUNS_SCALABILITY:-10}"
export REQUIRED_TEE="${REQUIRED_TEE:-TDX}"
export RUNTIME_CLASS="${RUNTIME_CLASS:-simulated-kata-qemu-tdx}"
export BENCH_RUNTIME_CLASS="${BENCH_RUNTIME_CLASS:-${RUNTIME_CLASS}}"
export NODE="${NODE:-ai-platform-control-plane}"

bash article1/experiments/harness/measure_scalability.sh
