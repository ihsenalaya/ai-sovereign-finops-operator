#!/usr/bin/env bash
# Snapshot results + figures into results/sample/ for reproducible archival.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$HERE"
mkdir -p results/sample
cp -f results/*.csv results/summary.md results/TEST_STATUS.md results/journal.jsonl results/sample/ 2>/dev/null || true
cp -f figures/*.png results/sample/ 2>/dev/null || true
echo "exported $(ls results/sample | wc -l) files to results/sample/"
