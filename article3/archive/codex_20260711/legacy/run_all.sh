#!/usr/bin/env bash
set -euo pipefail

# Reproducible Article 3 entry point. Reuse RUN_ID to resume an interrupted run.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT_DIR}"

echo "[article3] validating prerequisites"
command -v go >/dev/null
command -v sha256sum >/dev/null
go test ./... >/dev/null

echo "[article3] running checkpointed experiment suite"
bash experiments/orchestrator/run_all.sh

echo "[article3] generating figures, tables, statistics, and manuscript"
python3 analysis/scripts/generate_figures_tables.py
python3 analysis/scripts/generate_statistical_summary.py
python3 analysis/validation/prompt_release_gate.py
bash overleaf/build.sh

echo "[article3] writing reproducibility manifest"
date -u +%FT%TZ > artifacts/LAST_RUN_UTC.txt
git rev-parse HEAD > artifacts/LAST_RUN_COMMIT.txt
find experiments/raw experiments/processed figures tables -type f -print0 \
  | sort -z \
  | xargs -0 sha256sum > provenance/generated_artifacts.sha256
echo "[article3] complete"
