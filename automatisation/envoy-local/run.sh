#!/usr/bin/env bash
# Local Envoy data-path overhead benchmark (no cloud cost). Starts a real Envoy
# proxy in front of the LLM provider and runs gpubench direct vs through Envoy.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${HERE}/../.." && pwd)"
KEY="${KEY_PATH:-${REPO}/operateur/docs/openaikey.txt}"
MODEL="${MODEL:-gpt-4o-mini}"
CONC="${CONC:-8}"
REQ="${REQ:-48}"

docker rm -f greenops-envoy >/dev/null 2>&1 || true
docker run -d --name greenops-envoy -p 10000:10000 -p 9901:9901 \
  -v "${HERE}/envoy.yaml:/etc/envoy/envoy.yaml:ro" \
  envoyproxy/envoy:v1.31-latest -c /etc/envoy/envoy.yaml
trap 'docker rm -f greenops-envoy >/dev/null 2>&1 || true' EXIT
sleep 6

cd "${REPO}/experimentation"
rm -f results-bench/gpubench.csv
go run ./cmd/gpubench -base https://api.openai.com/v1 -key "${KEY}" -model "${MODEL}" \
  -concurrency "${CONC}" -requests "${REQ}" -warmup 4 -max-tokens 64 -results results-bench -label direct
go run ./cmd/gpubench -base http://localhost:10000/v1 -key "${KEY}" -model "${MODEL}" \
  -concurrency "${CONC}" -requests "${REQ}" -warmup 4 -max-tokens 64 -results results-bench -label via-envoy
echo "=== overhead ===" ; column -s, -t < results-bench/gpubench.csv
