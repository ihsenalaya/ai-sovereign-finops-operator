#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OVERLEAF_DIR="${ROOT_DIR}/overleaf"
ARTIFACTS_DIR="${ROOT_DIR}/artifacts"

mkdir -p "${ARTIFACTS_DIR}"
cd "${OVERLEAF_DIR}"

latexmk -pdf -interaction=nonstopmode -halt-on-error main.tex
cp main.pdf "${ARTIFACTS_DIR}/article3-paper.pdf"

rm -f "${ARTIFACTS_DIR}/article3-overleaf.zip"
zip -r "${ARTIFACTS_DIR}/article3-overleaf.zip" . \
  -x "*.aux" "*.bbl" "*.blg" "*.fdb_latexmk" "*.fls" "*.log" "*.out" "*.synctex.gz"
