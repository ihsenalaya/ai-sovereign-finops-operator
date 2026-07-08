#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-$ROOT_DIR/article2/experiments/evidence/aks-live/aks-kubeconfig}"
OUT_DIR="${1:-}"

if [[ -z "$OUT_DIR" ]]; then
  echo "usage: $0 <output-dir>" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"/{yaml,logs,metrics}

K="kubectl --kubeconfig $KUBECONFIG_PATH"

capture_cmd() {
  local target="$1"
  shift
  "$@" >"$target"
}

pick_pod() {
  local namespace="$1"
  local selector="$2"
  $K get pod -n "$namespace" -l "$selector" -o jsonpath='{.items[0].metadata.name}'
}

FINANCE_POD="$(pick_pod finance app=risk-assistant)"
RH_POD="$(pick_pod rh app=chatbot-rh)"

capture_cmd "$OUT_DIR/yaml/nodes.txt" bash -lc "$K get nodes -o wide"
capture_cmd "$OUT_DIR/yaml/pods.txt" bash -lc "$K get pods -A -o wide"
capture_cmd "$OUT_DIR/yaml/deployments.txt" bash -lc "$K get deploy -A -o wide"
capture_cmd "$OUT_DIR/yaml/services.txt" bash -lc "$K get svc -A -o wide"
capture_cmd "$OUT_DIR/yaml/crds.txt" bash -lc "$K get crds | grep -E 'aiops|gateway|envoy' || true"
capture_cmd "$OUT_DIR/yaml/aigateway.yaml" bash -lc "$K get aigw,aireport,aiqgate,aibudget,aisov -A -o yaml"
capture_cmd "$OUT_DIR/yaml/providers_models.yaml" bash -lc "$K get aiproviders,aimodels -A -o yaml"
capture_cmd "$OUT_DIR/yaml/quality_evidence_configmaps.yaml" bash -lc "$K get configmap -n default -l aiops.imperium.io/evidence-source=quality-eval-job -o yaml"
capture_cmd "$OUT_DIR/yaml/shadow_egress.yaml" bash -lc "$K get configmap -n default shadow-egress -o yaml"
capture_cmd "$OUT_DIR/yaml/tetragon.txt" bash -lc "$K get pods,ds -n kube-system -l app.kubernetes.io/name=tetragon -o wide"
capture_cmd "$OUT_DIR/yaml/prometheus.txt" bash -lc "$K get deploy,svc,configmap -n greenops-system -l app=demo-prometheus -o wide"
capture_cmd "$OUT_DIR/yaml/events.txt" bash -lc "$K get events -A --sort-by=.lastTimestamp"

capture_cmd "$OUT_DIR/logs/operator.log" bash -lc "$K logs -n greenops-system deploy/greenops-ai-sovereign-finops-operator --tail=400"
capture_cmd "$OUT_DIR/logs/chatbot-rh.log" bash -lc "$K logs -n rh deploy/chatbot-rh --all-containers --tail=200"
capture_cmd "$OUT_DIR/logs/risk-assistant.log" bash -lc "$K logs -n finance deploy/risk-assistant --all-containers --tail=400"
capture_cmd "$OUT_DIR/logs/contract-review.log" bash -lc "$K logs -n legal deploy/contract-review --all-containers --tail=200"
capture_cmd "$OUT_DIR/logs/content-writer.log" bash -lc "$K logs -n marketing deploy/content-writer --all-containers --tail=200"
capture_cmd "$OUT_DIR/logs/quality-jobs.log" bash -lc "$K logs -n default -l aiops.imperium.io/quality-evaluator=true --all-containers --tail=200 --prefix"
capture_cmd "$OUT_DIR/logs/shadow-rogue.log" bash -lc "$K logs -n finance deploy/shadow-ai-rogue --tail=200"
capture_cmd "$OUT_DIR/logs/tetragon-export.log" bash -lc "$K logs -n kube-system -l app.kubernetes.io/name=tetragon -c export-stdout --tail=400 --prefix"
capture_cmd "$OUT_DIR/logs/prometheus.log" bash -lc "$K logs -n greenops-system deploy/demo-prometheus --tail=200"

capture_cmd "$OUT_DIR/metrics/gateway_metrics.prom" bash -lc "$K exec -n finance $FINANCE_POD -c client -- sh -lc 'curl -s http://greenops-aigw-metrics.envoy-gateway-system.svc.cluster.local:1064/metrics'"
capture_cmd "$OUT_DIR/metrics/gateway_probe.json" bash -lc "$K exec -n rh $RH_POD -c client -- sh -lc 'curl -s -H \"Content-Type: application/json\" -d '\''{\"model\":\"gpt-france-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"Confirm the gateway path is healthy in one sentence.\"}],\"max_tokens\":32}'\'' http://greenops-aigw.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions'"
capture_cmd "$OUT_DIR/metrics/operator_metrics.prom" bash -lc "$K exec -n greenops-system deploy/demo-prometheus -- sh -lc 'wget -qO- http://greenops-ai-sovereign-finops-operator-metrics.greenops-system.svc.cluster.local:8080/metrics'"
capture_cmd "$OUT_DIR/metrics/prometheus_targets.txt" bash -lc "$K exec -n greenops-system deploy/demo-prometheus -- sh -lc 'wget -qO- http://127.0.0.1:9090/api/v1/targets'"

sha256sum \
  "$OUT_DIR/metrics/gateway_metrics.prom" \
  "$OUT_DIR/metrics/gateway_probe.json" \
  "$OUT_DIR/metrics/operator_metrics.prom" \
  "$OUT_DIR/metrics/prometheus_targets.txt" \
  "$OUT_DIR/yaml/aigateway.yaml" \
  "$OUT_DIR/yaml/providers_models.yaml" \
  "$OUT_DIR/yaml/shadow_egress.yaml" \
  >"$OUT_DIR/sha256.txt"
