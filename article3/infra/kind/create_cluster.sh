#!/usr/bin/env bash
set -euo pipefail
KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${KIND_DIR}/create.sh"
