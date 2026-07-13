#!/usr/bin/env bash
set -euo pipefail

# Phase E infrastructure installation only. GOV-AR application deployment is
# intentionally gated on the independently approved Phase D implementation.
KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "${KIND_DIR}/create.sh"
bash "${KIND_DIR}/validate.sh"
