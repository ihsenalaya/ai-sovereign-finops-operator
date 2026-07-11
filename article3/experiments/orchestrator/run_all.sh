#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

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

for cmd in "${commands[@]}"; do
  echo "[run_all] ${cmd}"
  eval "${cmd}"
done

echo "[run_all] validating generated outputs"
bash experiments/orchestrator/verify_outputs.sh

echo "[run_all] completed successfully"
