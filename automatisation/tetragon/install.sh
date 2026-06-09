#!/usr/bin/env bash
# Install Tetragon (standalone eBPF DaemonSet — no CNI change, any cluster) and apply
# the egress-connect TracingPolicy. This is the gateway-INDEPENDENT sovereignty plane
# for greenops: it lets the operator catch shadow-AI (pods calling LLM endpoints
# directly, bypassing the AI gateway). Tested target: AKS (standard Linux kernel with
# BTF). On a memory-starved kind/WSL it may not have room — prefer AKS.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
TETRAGON_NS="${TETRAGON_NS:-kube-system}"

echo "==> Installing Tetragon (Helm)"
helm repo add cilium https://helm.cilium.io >/dev/null 2>&1 || true
helm repo update >/dev/null
helm upgrade --install tetragon cilium/tetragon \
  -n "${TETRAGON_NS}" \
  --set tetragon.enableProcessCred=true

echo "==> Waiting for Tetragon DaemonSet to be ready"
kubectl -n "${TETRAGON_NS}" rollout status ds/tetragon --timeout=180s

echo "==> Applying egress TracingPolicy"
kubectl apply -f "${HERE}/tracingpolicy.yaml"

cat <<EOF

Tetragon installed. Next:
  1. (optional) deploy the shadow-AI demo workload:  kubectl apply -f ${HERE}/rogue-app.yaml
  2. start the forwarder (writes the shadow-egress ConfigMap the operator reads):
       NS=default ${HERE}/forwarder.sh
  3. watch the operator classify it:  the AISovereigntyPolicy reconcile emits
     ai_finops_shadow_ai_egress and the Grafana "Shadow-AI egress" panels light up.
EOF
