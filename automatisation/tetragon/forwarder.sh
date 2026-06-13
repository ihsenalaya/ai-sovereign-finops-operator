#!/usr/bin/env bash
# Bridge Tetragon eBPF egress events -> the `shadow-egress` ConfigMap the greenops
# operator reads. It reads Tetragon's JSON export log (reliable across versions — the
# live gRPC stream is not always served), prefers tcp_connect events and maps each
# destination IP to a known LLM hostname (by resolving the providers' own hostnames).
# If a platform does not export those kprobe events (common on kind/WSL), it falls
# back to Tetragon process_exec events that contain direct LLM URLs such as
# `https://api.openai.com/...`. The operator's shadowengine then classifies each by
# sovereignty zone — real Tetragon data, no manual seeding.
#
# Deps: kubectl + python3 (no jq required). Tune TETRAGON_NS / EXPORT_LOG if needed.
set -euo pipefail
NS="${NS:-default}"                  # namespace holding the shadow-egress ConfigMap (the policy's ns)
TETRAGON_NS="${TETRAGON_NS:-kube-system}"
EXPORT_LOG="${EXPORT_LOG:-/var/run/cilium/tetragon/tetragon.log}"
TAIL="${TAIL:-2000}"                 # lines of export log to scan
CM="shadow-egress"
KCTX="${KCTX:-}"

# Known LLM hostnames — keep in sync with internal/catalog/defaults.go knownHosts.
HOSTS="api.openai.com api.anthropic.com api.mistral.ai api.cohere.com api.cohere.ai api.groq.com generativelanguage.googleapis.com"

K=(kubectl)
if [ -n "${KCTX}" ]; then
  K+=(--context "${KCTX}")
fi

POD="$("${K[@]}" -n "${TETRAGON_NS}" get pods -l app.kubernetes.io/name=tetragon -o name | head -1)"
[ -n "${POD}" ] || { echo "no tetragon pod found in ${TETRAGON_NS}" >&2; exit 1; }

echo "==> Reading Tetragon export log (${EXPORT_LOG}, last ${TAIL} lines)"
TMP="$(mktemp)"
trap 'rm -f "${TMP}"' EXIT
"${K[@]}" -n "${TETRAGON_NS}" exec "${POD}" -c tetragon -- sh -c "tail -n ${TAIL} ${EXPORT_LOG}" >"${TMP}" 2>/dev/null || true
"${K[@]}" -n "${TETRAGON_NS}" logs "${POD}" -c export-stdout --tail="${TAIL}" >>"${TMP}" 2>/dev/null || true
"${K[@]}" -n "${TETRAGON_NS}" logs "${POD}" -c tetragon --tail="${TAIL}" >>"${TMP}" 2>/dev/null || true

echo "==> Building ${CM} from observed egress (resolving known LLM hosts)"
EGRESS_JSON="$(HOSTS="${HOSTS}" python3 /dev/stdin "${TMP}" <<'PY'
import collections
import json
import os
import re
import socket
import sys

log = open(sys.argv[1]).read() if len(sys.argv) > 1 else ""
hosts = os.environ["HOSTS"].split()

# Resolve known LLM hostnames to their current IPs -> ip2host.
ip2host = {}
for h in hosts:
    try:
        for fam, _, _, _, sa in socket.getaddrinfo(h, 443, socket.AF_INET, socket.SOCK_STREAM):
            ip2host[sa[0]] = h
    except OSError:
        pass

def pod_identity(pod):
    labels = pod.get("pod_labels") or {}
    app = (
        labels.get("app")
        or labels.get("app.kubernetes.io/name")
        or labels.get("aiops.imperium.io/application")
        or pod.get("workload")
        or pod.get("name")
        or "unknown"
    )
    ns = pod.get("namespace") or ""
    return ns, app

def find_exec_host(arguments):
    text = (arguments or "").lower()
    for match in re.findall(r'https?://([^/\s]+)', text):
        host = match.split("@", 1)[-1].split(":", 1)[0]
        for known in hosts:
            if host == known or host.endswith("." + known):
                return known
    for known in hosts:
        if known in text:
            return known
    return ""

kprobe_agg = collections.Counter()
exec_agg = collections.Counter()
for line in log.splitlines():
    line = line.strip()
    if not line:
        continue
    try:
        e = json.loads(line)
    except ValueError:
        continue
    kp = e.get("process_kprobe")
    if not kp or kp.get("function_name") != "tcp_connect":
        pe = e.get("process_exec")
        if not pe:
            continue
        process = pe.get("process") or {}
        pod = process.get("pod") or {}
        ns, app = pod_identity(pod)
        if not ns or not app:
            continue
        host = find_exec_host(process.get("arguments") or "")
        if not host:
            continue
        exec_agg[(ns, app, host)] += 1
        continue

    pod = (kp.get("process") or {}).get("pod") or {}
    ns, app = pod_identity(pod)
    if not ns or not app:
        continue
    daddr = ""
    for a in kp.get("args") or []:
        sa = a.get("sock_arg")
        if sa and sa.get("daddr"):
            daddr = sa["daddr"]
            break
    host = ip2host.get(daddr)
    if not host:
        continue
    kprobe_agg[(ns, app, host)] += 1

agg = kprobe_agg if kprobe_agg else exec_agg
records = [
    {"namespace": ns, "application": app, "host": host, "connections": n}
    for (ns, app, host), n in sorted(agg.items())
]
print(json.dumps(records))
PY
)"

echo "${EGRESS_JSON}"
"${K[@]}" create configmap "${CM}" -n "${NS}" \
  --from-literal=egress.json="${EGRESS_JSON}" \
  --dry-run=client -o yaml | "${K[@]}" apply -f -

echo "==> Done. The operator classifies these on its next AISovereigntyPolicy reconcile."
