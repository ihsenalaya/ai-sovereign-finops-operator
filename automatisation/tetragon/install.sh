#!/usr/bin/env bash
# Install Tetragon (standalone eBPF DaemonSet — no CNI change, any cluster) and apply
# the egress-connect TracingPolicy. This is the gateway-INDEPENDENT sovereignty plane
# for greenops: it lets the operator catch shadow-AI (pods calling LLM endpoints
# directly, bypassing the AI gateway). Tested target: AKS (standard Linux kernel with
# BTF). kind/WSL is also supported for the demo: if tcp_connect kprobe events are not
# exported there, the forwarder falls back to Tetragon process_exec events.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
TETRAGON_NS="${TETRAGON_NS:-kube-system}"
KCTX="${KCTX:-}"

K=(kubectl)
HELM_CTX=()
if [ -n "${KCTX}" ]; then
  K+=(--context "${KCTX}")
  HELM_CTX+=(--kube-context "${KCTX}")
fi

echo "==> Installing Tetragon (Helm)"
helm repo add cilium https://helm.cilium.io >/dev/null 2>&1 || true
helm repo update >/dev/null
helm upgrade --install tetragon cilium/tetragon \
  "${HELM_CTX[@]}" \
  -n "${TETRAGON_NS}" \
  --set tetragon.enableProcessCred=true \
  --set tetragon.enableProcessNs=true \
  --set tetragon.exportRateLimit=-1 \
  --set tetragon.exportFilename=tetragon.log \
  --set export.mode=stdout

echo "==> Waiting for Tetragon DaemonSet to be ready"
"${K[@]}" -n "${TETRAGON_NS}" rollout status ds/tetragon --timeout=180s

echo "==> Applying egress TracingPolicy"
"${K[@]}" apply -f "${HERE}/tracingpolicy.yaml"

cat <<EOF

Tetragon installed. The real-demo automation now deploys the shadow-AI workload,
runs the forwarder, and refreshes the operator status automatically.
EOF
