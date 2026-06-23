#!/usr/bin/env bash
# Run the full RQ1-RQ6 + ablation experiment with real OpenAI calls.
# Responses/judge scores are cached to results/cache.json (reproducible, re-runs free).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$HERE"

KEY="${KEY_PATH:-../operateur/docs/openaikey.txt}"
JUDGE="${JUDGE_MODEL:-gpt-4o}"
RESULTS="${RESULTS_DIR:-results}"

echo "[experiment] judge=$JUDGE results=$RESULTS"
go run ./cmd/experiment -key "$KEY" -judge "$JUDGE" -results "$RESULTS" -datasets datasets -timeout-min 40
echo "[experiment] done — see $RESULTS/TEST_STATUS.md and $RESULTS/*.csv"
