#!/usr/bin/env bash
set -euo pipefail
KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "${KIND_DIR}/test_kind_source.py"
