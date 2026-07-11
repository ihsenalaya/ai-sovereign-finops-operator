#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

required_files=(
  "experiments/raw/E0_smoke.json"
  "experiments/raw/E1_mean_std.json"
  "experiments/raw/E1_quantile.json"
  "experiments/raw/E2_mean_std.json"
  "experiments/raw/E2_quantile.json"
  "experiments/raw/E3_drift.json"
  "experiments/raw/E4_faults.json"
  "experiments/raw/E5_scalability.json"
  "experiments/raw/E7_ablation.json"
  "experiments/processed/E0_smoke_summary.json"
  "experiments/processed/E1_mean_std_summary.json"
  "experiments/processed/E1_quantile_summary.json"
  "experiments/processed/E2_mean_std_summary.json"
  "experiments/processed/E2_quantile_summary.json"
  "experiments/processed/E3_drift_summary.json"
  "experiments/processed/E4_faults_summary.json"
  "experiments/processed/E5_scalability_summary.json"
  "experiments/processed/E7_ablation_summary.json"
  "experiments/processed/E1_mean_std_campaign.json"
  "experiments/processed/E1_quantile_campaign.json"
  "experiments/processed/E2_mean_std_campaign.json"
  "experiments/processed/E2_quantile_campaign.json"
  "experiments/processed/E1_matrix.json"
  "experiments/processed/E2_matrix.json"
  "experiments/processed/E1_comparison.json"
  "experiments/processed/E2_comparison.json"
  "reports/E0_SMOKE_RESULTS.md"
  "reports/E1_E2_INITIAL_RESULTS.md"
  "reports/E1_CAMPAIGN_RESULTS.md"
  "reports/E2_CAMPAIGN_RESULTS.md"
  "reports/E1_MATRIX_RESULTS.md"
  "reports/E2_MATRIX_RESULTS.md"
  "reports/E1_COMPARISON.md"
  "reports/E2_COMPARISON.md"
  "reports/E3_DRIFT_RESULTS.md"
  "reports/E4_FAULT_RESULTS.md"
  "reports/E5_SCALABILITY_RESULTS.md"
  "reports/E7_ABLATION_RESULTS.md"
)

for path in "${required_files[@]}"; do
  if [[ ! -f "${path}" ]]; then
    echo "[verify_outputs] missing required artifact: ${path}" >&2
    exit 1
  fi
done

python3 analysis/validation/verify_experiment_outputs.py \
  experiments/raw \
  experiments/processed

echo "[verify_outputs] artifact verification passed"
