#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-$ROOT_DIR/article2/experiments/evidence/aks-live/aks-kubeconfig}"
K="kubectl --kubeconfig $KUBECONFIG_PATH"
RUN_ID="${RUN_ID:-aks-live-$(date -u +%Y%m%dT%H%M%SZ)}"
OUT_DIR="$ROOT_DIR/article2/experiments/runs/E3_quality_gate/$RUN_ID"
REPETITIONS="${REPETITIONS:-5}"
MIN_SAMPLES="${MIN_SAMPLES:-20}"
RUN_LABEL_KEY="article2-run-id"
MAX_WAIT_SECONDS="${MAX_WAIT_SECONDS:-1800}"

mkdir -p "$OUT_DIR"/{evidence,manifests,processed}

log() {
  printf '[E3] %s\n' "$*"
}

capture() {
  local target="$1"
  shift
  "$@" >"$target"
}

ensure_ghcr_pull_secret() {
  local namespace="$1"
  local token
  token="$(grep -E 'oauth_token:' "${HOME}/.config/gh/hosts.yml" 2>/dev/null | head -1 | awk '{print $2}')"
  if [[ -z "$token" ]]; then
    log "No GHCR token available; skipping imagePullSecret setup for $namespace"
    return 0
  fi
  kubectl --kubeconfig "$KUBECONFIG_PATH" create namespace "$namespace" --dry-run=client -o yaml | $K apply -f - >/dev/null
  kubectl --kubeconfig "$KUBECONFIG_PATH" -n "$namespace" create secret docker-registry ghcr-pull \
    --docker-server=ghcr.io \
    --docker-username=ihsenalaya \
    --docker-password="$token" \
    --docker-email=ci@article2.local \
    --dry-run=client -o yaml | $K apply -f - >/dev/null
  $K -n "$namespace" patch serviceaccount default --type=merge -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}' >/dev/null
}

wait_rollout() {
  local namespace="$1"
  local deploy="$2"
  $K -n "$namespace" rollout status "deploy/$deploy" --timeout=240s >/dev/null
}

set_workload_model() {
  local namespace="$1"
  local deploy="$2"
  local model="$3"
  local attempts="${4:-24}"
  local replicas="${5:-2}"

  log "Patch $namespace/$deploy -> model=$model attempts=$attempts replicas=$replicas"
  $K -n "$namespace" set env "deploy/$deploy" \
    MODEL="$model" \
    ATTEMPT_LIMIT="$attempts" \
    ATTEMPT_SLEEP_SECONDS="1" \
    START_DELAY_SECONDS="1" \
    SUCCESS_TARGET="999" >/dev/null
  $K -n "$namespace" scale "deploy/$deploy" --replicas="$replicas" >/dev/null
  $K -n "$namespace" rollout restart "deploy/$deploy" >/dev/null
  wait_rollout "$namespace" "$deploy"
}

drive_and_settle() {
  local seconds="${1:-45}"
  log "Waiting ${seconds}s for requests and metrics to settle"
  sleep "$seconds"
}

delete_if_exists() {
  local kind="$1"
  local name="$2"
  $K -n default delete "$kind" "$name" --ignore-not-found >/dev/null
}

render_dataset_manifest() {
  python3 "$ROOT_DIR/article2/scripts/render_quality_golden_configmaps.py" >"$OUT_DIR/manifests/quality-golden-configmaps.yaml"
}

render_gate_manifest() {
  local name="$1"
  local app_ns="$2"
  local app_name="$3"
  local team="$4"
  local source_model="$5"
  local candidate_model="$6"
  local golden_name="$7"
  local evidence_name="$8"
  local latency_ms="$9"
  local max_tokens="${10}"
  local output_file="${11:-$OUT_DIR/manifests/e3-gates.yaml}"
  cat >>"$output_file" <<EOF
---
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: $name
  namespace: default
  labels:
    app.kubernetes.io/name: ai-sovereign-finops-operator
    ${RUN_LABEL_KEY}: "$RUN_ID"
    article2-gate-base: "${name%-r*}"
    article2-gate-kind: repetition
spec:
  target:
    namespace: $app_ns
    team: $team
    application: $app_name
  sourceModel: $source_model
  candidateModel: $candidate_model
  goldenDatasetRef:
    name: $golden_name
  evidenceRef:
    name: $evidence_name
  gatewayRef: main-gateway
  latencyThresholdMs: $latency_ms
  minSamples: $MIN_SAMPLES
  tolerancePoints: 3
  evaluation:
    endpoint: http://greenops-aigw.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions
    maxTokens: $max_tokens
    timeoutSeconds: 90
  weights:
    judged: 0
EOF
}

render_aux_manifests() {
  cat >"$OUT_DIR/manifests/e3-aux-gates.yaml" <<EOF
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: finance-quality-missing-evidence
  namespace: default
  labels:
    ${RUN_LABEL_KEY}: "$RUN_ID"
    article2-gate-kind: auxiliary
spec:
  target:
    namespace: finance
    team: finance
    application: risk-assistant
  sourceModel: gpt-france-mini
  candidateModel: mistral-large-latest
  goldenDatasetRef:
    name: finance-quality-golden
  gatewayRef: main-gateway
  latencyThresholdMs: 3000
  minSamples: $MIN_SAMPLES
  tolerancePoints: 3
  evaluation:
    endpoint: http://greenops-aigw.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions
    maxTokens: 80
    timeoutSeconds: 90
  weights:
    judged: 0
---
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: finance-quality-missing-telemetry
  namespace: default
  labels:
    ${RUN_LABEL_KEY}: "$RUN_ID"
    article2-gate-kind: auxiliary
spec:
  target:
    namespace: finance
    team: finance
    application: risk-assistant
  sourceModel: gpt-france-mini
  candidateModel: cohere-command-a-latest
  goldenDatasetRef:
    name: finance-quality-golden
  evidenceRef:
    name: finance-quality-missing-telemetry-evidence
  gatewayRef: main-gateway
  latencyThresholdMs: 3000
  minSamples: $MIN_SAMPLES
  tolerancePoints: 3
  evaluation:
    endpoint: http://greenops-aigw.envoy-gateway-system.svc.cluster.local:80/v1/chat/completions
    maxTokens: 80
    timeoutSeconds: 90
  weights:
    judged: 0
EOF
}

wait_for_gates() {
  local deadline=$(( $(date +%s) + MAX_WAIT_SECONDS ))
  while true; do
    local all_done=1
    while IFS= read -r phase; do
      if [[ "$phase" != "Passed" && "$phase" != "Failed" ]]; then
        all_done=0
        break
      fi
    done < <($K -n default get aiqualitygate -l "${RUN_LABEL_KEY}=${RUN_ID},article2-gate-kind=repetition" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}')

    if [[ "$all_done" -eq 1 ]]; then
      log "All repetition gates reached terminal phases"
      return 0
    fi
    if [[ "$(date +%s)" -ge "$deadline" ]]; then
      log "Timed out waiting for E3 gates"
      return 1
    fi
    sleep 15
  done
}

wait_for_gate() {
  local gate_name="$1"
  local deadline=$(( $(date +%s) + MAX_WAIT_SECONDS ))
  while true; do
    local phase
    phase="$($K -n default get aiqualitygate "$gate_name" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    if [[ "$phase" == "Passed" || "$phase" == "Failed" ]]; then
      log "Gate $gate_name reached terminal phase: $phase"
      return 0
    fi
    if [[ "$(date +%s)" -ge "$deadline" ]]; then
      log "Timed out waiting for gate $gate_name"
      return 1
    fi
    sleep 15
  done
}

log "Rendering and applying 20-item golden datasets"
render_dataset_manifest
$K apply -f "$OUT_DIR/manifests/quality-golden-configmaps.yaml" >/dev/null

log "Deleting previous auxiliary gates if present"
delete_if_exists aiqualitygate finance-quality-missing-evidence
delete_if_exists aiqualitygate finance-quality-missing-telemetry
delete_if_exists configmap finance-quality-missing-telemetry-evidence

log "Deleting baseline demo quality gates to isolate E3"
delete_if_exists aiqualitygate finance-risk-assistant-quality
delete_if_exists aiqualitygate legal-contract-quality
delete_if_exists aiqualitygate marketing-content-quality
delete_if_exists aiqualitygate rh-chatbot-quality

log "Ensuring GHCR pull secrets for injected sidecars"
for ns in finance legal marketing rh; do
  ensure_ghcr_pull_secret "$ns"
done

log "Generating fresh source/candidate telemetry"
set_workload_model finance risk-assistant gpt-france-mini 24 2
drive_and_settle 45
set_workload_model finance risk-assistant mistral-large-latest 24 2
drive_and_settle 45
set_workload_model legal contract-review gpt-france-mini 24 2
drive_and_settle 45
set_workload_model legal contract-review mistral-large-latest 24 2
drive_and_settle 45
set_workload_model marketing content-writer gpt-france-mini 24 2
drive_and_settle 45
set_workload_model marketing content-writer mistral-large-latest 24 2
drive_and_settle 45
set_workload_model rh chatbot-rh mistral-large-latest 24 2
drive_and_settle 45
set_workload_model rh chatbot-rh gpt-france-mini 24 2
drive_and_settle 45

log "Rendering repetition gates"
: >"$OUT_DIR/manifests/e3-gates.yaml"
for rep in $(seq 1 "$REPETITIONS"); do
  suffix="$(printf 'r%02d' "$rep")"
  gate_dir="$OUT_DIR/manifests/gates/$suffix"
  mkdir -p "$gate_dir"
  : >"$gate_dir/finance-risk-assistant-quality-$suffix.yaml"
  : >"$gate_dir/legal-contract-quality-$suffix.yaml"
  : >"$gate_dir/marketing-content-quality-$suffix.yaml"
  : >"$gate_dir/rh-chatbot-quality-$suffix.yaml"
  render_gate_manifest "finance-risk-assistant-quality-$suffix" finance risk-assistant finance gpt-france-mini mistral-large-latest finance-quality-golden "finance-quality-evidence-$suffix" 3000 80 "$gate_dir/finance-risk-assistant-quality-$suffix.yaml"
  render_gate_manifest "legal-contract-quality-$suffix" legal contract-review legal gpt-france-mini mistral-large-latest legal-quality-golden "legal-quality-evidence-$suffix" 3500 90 "$gate_dir/legal-contract-quality-$suffix.yaml"
  render_gate_manifest "marketing-content-quality-$suffix" marketing content-writer marketing gpt-france-mini mistral-large-latest marketing-quality-golden "marketing-quality-evidence-$suffix" 3500 90 "$gate_dir/marketing-content-quality-$suffix.yaml"
  render_gate_manifest "rh-chatbot-quality-$suffix" rh chatbot-rh rh mistral-large-latest gpt-france-mini rh-quality-golden "rh-quality-evidence-$suffix" 3000 70 "$gate_dir/rh-chatbot-quality-$suffix.yaml"
  cat "$gate_dir/"*.yaml >>"$OUT_DIR/manifests/e3-gates.yaml"
done
render_aux_manifests

log "Deleting previous repetition gates if present"
for rep in $(seq 1 "$REPETITIONS"); do
  suffix="$(printf 'r%02d' "$rep")"
  delete_if_exists aiqualitygate "finance-risk-assistant-quality-$suffix"
  delete_if_exists aiqualitygate "legal-contract-quality-$suffix"
  delete_if_exists aiqualitygate "marketing-content-quality-$suffix"
  delete_if_exists aiqualitygate "rh-chatbot-quality-$suffix"
  delete_if_exists configmap "finance-quality-evidence-$suffix"
  delete_if_exists configmap "legal-quality-evidence-$suffix"
  delete_if_exists configmap "marketing-quality-evidence-$suffix"
  delete_if_exists configmap "rh-quality-evidence-$suffix"
done

log "Applying repetition gates sequentially to stay within real quota"
for rep in $(seq 1 "$REPETITIONS"); do
  suffix="$(printf 'r%02d' "$rep")"
  for gate in \
    "finance-risk-assistant-quality-$suffix" \
    "legal-contract-quality-$suffix" \
    "marketing-content-quality-$suffix" \
    "rh-chatbot-quality-$suffix"; do
    log "Applying gate $gate"
    $K apply -f "$OUT_DIR/manifests/gates/$suffix/$gate.yaml" >/dev/null
    wait_for_gate "$gate"
    sleep 20
  done
done

log "Applying auxiliary gates"
$K apply -f "$OUT_DIR/manifests/e3-aux-gates.yaml" >/dev/null

log "Waiting for repetition gates to finish"
wait_for_gates
sleep 20

log "Capturing evidence"
"$ROOT_DIR/article2/scripts/collect_evidence.sh" "$OUT_DIR/common"
evidence_names="finance-quality-missing-telemetry-evidence"
for rep in $(seq 1 "$REPETITIONS"); do
  suffix="$(printf 'r%02d' "$rep")"
  evidence_names+=" finance-quality-evidence-$suffix"
  evidence_names+=" legal-quality-evidence-$suffix"
  evidence_names+=" marketing-quality-evidence-$suffix"
  evidence_names+=" rh-quality-evidence-$suffix"
done
capture "$OUT_DIR/evidence/aiqualitygates.after.json" \
  bash -lc "$K get aiqualitygate -n default -l ${RUN_LABEL_KEY}=${RUN_ID} -o json"
capture "$OUT_DIR/evidence/aiqualitygates.after.yaml" \
  bash -lc "$K get aiqualitygate -n default -l ${RUN_LABEL_KEY}=${RUN_ID} -o yaml"
capture "$OUT_DIR/evidence/evidence-configmaps.after.yaml" \
  bash -lc "$K get configmap -n default $evidence_names --ignore-not-found -o yaml"
capture "$OUT_DIR/evidence/quality-jobs.after.txt" \
  bash -lc "$K get jobs -n default -o wide | grep quality || true"
capture "$OUT_DIR/evidence/operator.after.log" \
  bash -lc "$K logs -n greenops-system deploy/greenops-ai-sovereign-finops-operator --tail=800"

log "Summarizing repetition stability"
python3 "$ROOT_DIR/article2/scripts/summarize_e3_repetitions.py" \
  --input-json "$OUT_DIR/evidence/aiqualitygates.after.json" \
  --output-json "$OUT_DIR/processed/summary.json" \
  --output-md "$OUT_DIR/SUMMARY.md" \
  --output-tex "$OUT_DIR/processed/e3_qualitygate_detailed.tex"

log "E3 campaign complete: $OUT_DIR"
