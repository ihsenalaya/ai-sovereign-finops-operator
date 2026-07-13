#!/usr/bin/env bash
set -euo pipefail
echo "refusing experiment deployment: Phase D application release is not yet independently approved; use the frozen experiment orchestrator after that gate" >&2
exit 2
