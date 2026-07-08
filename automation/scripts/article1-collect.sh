#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-results/article1/raw/manual-$(date +%Y%m%d-%H%M%S)}"
mkdir -p "${OUT_DIR}"

if command -v kubectl >/dev/null 2>&1; then
  kubectl get pods -A -o wide > "${OUT_DIR}/kubectl_pods.txt"
  kubectl get events -A --sort-by=.metadata.creationTimestamp > "${OUT_DIR}/kubectl_events.txt"
fi

echo "environment=manual" > "${OUT_DIR}/run.env"
echo "Results collected into ${OUT_DIR}"
