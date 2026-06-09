#!/usr/bin/env bash
# Bridge Tetragon eBPF egress events -> the `shadow-egress` ConfigMap the greenops
# operator reads. It samples per-pod TCP-connect events, maps each destination IP to
# a known LLM hostname (by resolving the providers' own hostnames), aggregates per
# (namespace, app, host) and writes the ConfigMap. The operator's shadowengine then
# classifies each by sovereignty zone — REAL eBPF data, no fabrication.
#
# A native Tetragon gRPC backend in the operator is the longer-term path; this script
# is the pragmatic, dependency-light bridge to demo the plane on AKS.
#
# Requirements: kubectl, jq, getent (glibc). Tune TETRAGON_NS / jq paths to your
# Tetragon version if its event schema differs.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
NS="${NS:-default}"                  # namespace holding the shadow-egress ConfigMap (the policy's ns)
TETRAGON_NS="${TETRAGON_NS:-kube-system}"
WINDOW="${WINDOW:-60}"               # seconds of events to sample per run
CM="shadow-egress"

# Known LLM hostnames — keep in sync with internal/catalog/defaults.go knownHosts.
HOSTS=(api.openai.com api.anthropic.com api.mistral.ai api.cohere.com api.cohere.ai
       api.groq.com generativelanguage.googleapis.com)

echo "==> Resolving known LLM hostnames to IPs"
declare -A IP2HOST
for h in "${HOSTS[@]}"; do
  while read -r ip; do [ -n "$ip" ] && IP2HOST["$ip"]="$h"; done \
    < <(getent ahostsv4 "$h" 2>/dev/null | awk '{print $1}' | sort -u)
done
echo "    mapped ${#IP2HOST[@]} IPs"

echo "==> Sampling Tetragon egress events for ${WINDOW}s"
POD="$(kubectl -n "${TETRAGON_NS}" get pods -l app.kubernetes.io/name=tetragon -o name | head -1)"
[ -n "${POD}" ] || { echo "no tetragon pod found in ${TETRAGON_NS}" >&2; exit 1; }
EVENTS="$(kubectl -n "${TETRAGON_NS}" exec "${POD}" -c tetragon -- \
  timeout "${WINDOW}" tetra getevents -o json 2>/dev/null || true)"

# Aggregate (namespace \t app \t destIP) -> count from tcp_connect kprobe events.
# jq paths follow Tetragon's process_kprobe schema; the sock arg carries the daddr.
declare -A AGG
while IFS=$'\t' read -r ns app ip; do
  [ -z "${ip:-}" ] && continue
  host="${IP2HOST[$ip]:-}"
  [ -z "$host" ] && continue                      # not a known LLM endpoint — ignore
  app="${app:-unknown}"
  AGG["${ns}|${app}|${host}"]=$(( ${AGG["${ns}|${app}|${host}"]:-0} + 1 ))
done < <(printf '%s\n' "${EVENTS}" | jq -rc '
  select(.process_kprobe.function_name=="tcp_connect")
  | [ (.process_kprobe.process.pod.namespace // "")
    , (.process_kprobe.process.pod.name // "")
    , ( [ .process_kprobe.args[]?.sock_arg.daddr // empty ] | first // "" )
    ] | @tsv' 2>/dev/null)

echo "==> Building ${CM} (${#AGG[@]} distinct AI egress flows)"
records="[]"
for key in "${!AGG[@]}"; do
  IFS='|' read -r ns app host <<<"$key"
  records="$(jq -c --arg ns "$ns" --arg app "$app" --arg host "$host" --argjson c "${AGG[$key]}" \
    '. += [{namespace:$ns, application:$app, host:$host, connections:$c}]' <<<"$records")"
done
printf '%s' "$records" | jq .

kubectl create configmap "${CM}" -n "${NS}" \
  --from-literal=egress.json="${records}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Done. The operator will classify these on its next AISovereigntyPolicy reconcile."
