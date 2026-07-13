#!/usr/bin/env bash
set -euo pipefail
KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "${KIND_DIR}/test_source.sh"
for profile in dev validation performance; do
  PROFILE="${profile}" bash "${KIND_DIR}/create.sh"
  PROFILE="${profile}" bash "${KIND_DIR}/validate.sh"
done
