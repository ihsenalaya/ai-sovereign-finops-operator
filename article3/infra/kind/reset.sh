#!/usr/bin/env bash
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE="${PROFILE:-validation}"
export PROFILE
bash "${KIND_DIR}/destroy.sh"
bash "${KIND_DIR}/create.sh"
