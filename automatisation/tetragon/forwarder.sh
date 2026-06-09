#!/usr/bin/env bash
# Bridge Tetragon eBPF egress events -> the `shadow-egress` ConfigMap the greenops
# operator reads. It reads Tetragon's JSON export log (reliable across versions — the
# live gRPC stream is not always served), keeps tcp_connect events, maps each
# destination IP to a known LLM hostname (by resolving the providers' own hostnames),
# aggregates per (namespace, workload, host) and writes the ConfigMap. The operator's
# shadowengine then classifies each by sovereignty zone — REAL eBPF data, no fabrication.
#
# Deps: kubectl + python3 (no jq required). Tune TETRAGON_NS / EXPORT_LOG if needed.
set -euo pipefail
NS="${NS:-default}"                  # namespace holding the shadow-egress ConfigMap (the policy's ns)
TETRAGON_NS="${TETRAGON_NS:-kube-system}"
EXPORT_LOG="${EXPORT_LOG:-/var/run/cilium/tetragon/tetragon.log}"
TAIL="${TAIL:-2000}"                 # lines of export log to scan
CM="shadow-egress"

# Known LLM hostnames — keep in sync with internal/catalog/defaults.go knownHosts.
HOSTS="api.openai.com api.anthropic.com api.mistral.ai api.cohere.com api.cohere.ai api.groq.com generativelanguage.googleapis.com"

POD="$(kubectl -n "${TETRAGON_NS}" get pods -l app.kubernetes.io/name=tetragon -o name | head -1)"
[ -n "${POD}" ] || { echo "no tetragon pod found in ${TETRAGON_NS}" >&2; exit 1; }

echo "==> Reading Tetragon export log (${EXPORT_LOG}, last ${TAIL} lines)"
TMP="$(mktemp)"
trap 'rm -f "${TMP}"' EXIT
kubectl -n "${TETRAGON_NS}" exec "${POD}" -c tetragon -- sh -c "tail -n ${TAIL} ${EXPORT_LOG}" >"${TMP}" 2>/dev/null || true

echo "==> Building ${CM} from observed egress (resolving known LLM hosts)"
EGRESS_JSON="$(HOSTS="${HOSTS}" python3 /dev/stdin "${TMP}" <<'PY'
import json, os, socket, sys, collections

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

def workload(podname):
    # strip the ReplicaSet/pod suffix: foo-5dbc575cfc-v9plm -> foo
    parts = podname.rsplit("-", 2)
    return parts[0] if len(parts) == 3 else podname

agg = collections.Counter()
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
        continue
    pod = (kp.get("process") or {}).get("pod") or {}
    ns = pod.get("namespace") or ""
    name = pod.get("name") or ""
    if not ns or not name:
        continue
    daddr = ""
    for a in kp.get("args") or []:
        sa = a.get("sock_arg")
        if sa and sa.get("daddr"):
            daddr = sa["daddr"]
            break
    host = ip2host.get(daddr)
    if not host:
        continue  # not a known LLM endpoint
    agg[(ns, workload(name), host)] += 1

records = [{"namespace": ns, "application": app, "host": host, "connections": n}
           for (ns, app, host), n in agg.items()]
print(json.dumps(records))
PY
)"

echo "${EGRESS_JSON}"
kubectl create configmap "${CM}" -n "${NS}" \
  --from-literal=egress.json="${EGRESS_JSON}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Done. The operator classifies these on its next AISovereigntyPolicy reconcile."
