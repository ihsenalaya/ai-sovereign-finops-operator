#!/usr/bin/env bash
# Article 1 — live kind campaign: positive e2e + attacks A1–A9.
#
# All results produced by this script are:
#   SIMULATED — KIND ONLY — NOT REAL TEE/GPU
#
# Every scenario writes one raw JSON file into $OUT_DIR. Nothing is inferred:
# observed state comes from kubectl reads of the live cluster.
set -uo pipefail

MARKING="SIMULATED — KIND ONLY — NOT REAL TEE/GPU"
OUT_DIR="${1:-results/article1/raw/kind}"
NODE="ai-platform-control-plane"
PLATFORM_NS="ai-platform"
PERMIT_WAIT=40   # scheduler permit timeout is 15s; leave margin
VERIFY_PLACEMENT="${VERIFY_PLACEMENT:-verify-placement}"
PUBKEY_HEX="${PUBKEY_HEX:-}"

mkdir -p "${OUT_DIR}"

ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }

json_escape() { python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))'; }

# write_result <file> <id> <name> <expected> <observed> <pass> <detail>
write_result() {
  local file="$1" id="$2" name="$3" expected="$4" observed="$5" passed="$6" detail="$7"
  python3 - "$OUT_DIR/$file" "$id" "$name" "$expected" "$observed" "$passed" "$detail" <<'PYEOF'
import json, sys, datetime
path, sid, name, expected, observed, passed, detail = sys.argv[1:8]
json.dump({
    "scenario_id": sid,
    "name": name,
    "environment": "kind",
    "marking": "SIMULATED — KIND ONLY — NOT REAL TEE/GPU",
    "expected": expected,
    "observed": observed,
    "pass": passed == "true",
    "detail": detail,
    "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
}, open(path, "w"), ensure_ascii=False, indent=2)
PYEOF
  echo "[$(ts)] ${id} ${name}: expected=${expected} observed=${observed} pass=${passed}"
}

# wait_pod_state <ns> <pod> — echoes "Running:<node>" | "Pending:" | "Absent:"
wait_pod_bound() {
  local ns="$1" pod="$2" deadline=$(( $(date +%s) + PERMIT_WAIT ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    local phase node
    phase=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || echo Absent)
    node=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)
    if [ -n "$node" ]; then echo "${phase}:${node}"; return; fi
    sleep 2
  done
  phase=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || echo Absent)
  echo "${phase}:"
}

make_ns() {
  local ns="$1" label="$2"
  kubectl delete namespace "$ns" --ignore-not-found --wait=true >/dev/null 2>&1
  kubectl create namespace "$ns" >/dev/null
  kubectl label namespace "$ns" "article1=${label}" >/dev/null
}

# apply_policy <ns> <label> [tee] [maxAge] [runtime]
apply_policy() {
  local ns="$1" label="$2" tee="${3:-SEV-SNP}" maxage="${4:-300}" runtime="${5:-kata-qemu-snp}"
  kubectl apply -f - <<EOF >/dev/null
apiVersion: aiops.imperium.io/v1alpha1
kind: ConfidentialInferencePolicy
metadata:
  name: article1-policy
  namespace: ${ns}
spec:
  target:
    namespaceSelector:
      matchLabels: {article1: "${label}"}
    workloadSelector:
      matchLabels: {app: confidential-inference}
  requiredTEE: ["${tee}"]
  requireConfidentialContainers: true
  allowedRuntimeClasses: ["${runtime}"]
  maxEvidenceAgeSeconds: ${maxage}
  requireModelDigest: true
  enforcementMode: enforce
EOF
}

# apply_evidence <ns> <name> [tee]
apply_evidence() {
  local ns="$1" name="$2" tee="${3:-SEV-SNP}"
  kubectl apply -f - <<EOF >/dev/null
apiVersion: aiops.imperium.io/v1alpha1
kind: AttestationEvidence
metadata:
  name: ${name}
  namespace: ${ns}
spec:
  subjectRef: {name: ${NODE}}
  evidenceType: cpu
  tee: ${tee}
  simulated: true
  runtime: {runtimeClassName: simulated-kata-qemu-snp, simulated: true}
  freshness: {maxAgeSeconds: 300, simulated: true}
EOF
}

wait_evidence_verified() {
  local ns="$1" name="$2" deadline=$(( $(date +%s) + 30 ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    v=$(kubectl get attestationevidence "$name" -n "$ns" -o jsonpath='{.status.verified}' 2>/dev/null)
    [ "$v" = "true" ] && return 0
    sleep 1
  done
  return 1
}

# make_pod <ns> <name> <evidence-annotation> [runtimeClass]
pod_yaml() {
  local ns="$1" name="$2" evref="$3" rtc="${4:-}"
  cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${name}
  namespace: ${ns}
  labels: {app: confidential-inference}
  annotations:
    ai.sovereign.io/model-digest: "sha256:e2e-model-digest"
$( [ -n "$evref" ] && echo "    ai.sovereign.io/attestation-evidence: \"${evref}\"" )
spec:
$( [ -n "$rtc" ] && echo "  runtimeClassName: ${rtc}" )
  restartPolicy: Never
  containers:
  - name: app
    image: platform-ui:0.5.9
    imagePullPolicy: Never
EOF
}

echo "== Article 1 kind campaign — $(ts) =="
echo "== Marking: ${MARKING} =="
kubectl get nodes -o wide > "${OUT_DIR}/cluster_nodes.txt"
kubectl version > "${OUT_DIR}/cluster_version.txt" 2>&1

# ─── P1: positive end-to-end ─────────────────────────────────────────────────
NS=article1-e2e
make_ns "$NS" "e2e"
apply_policy "$NS" "e2e"
apply_evidence "$NS" ev-control-plane
wait_evidence_verified "$NS" ev-control-plane || echo "WARN: evidence not verified in time"
pod_yaml "$NS" inference-positive ev-control-plane | kubectl apply -f - >/dev/null
STATE=$(wait_pod_bound "$NS" inference-positive)
kubectl get pod inference-positive -n "$NS" -o yaml > "${OUT_DIR}/p1_pod.yaml" 2>/dev/null
kubectl get aiplacementdecision -n "$NS" -o yaml > "${OUT_DIR}/p1_decision.yaml" 2>/dev/null

P1_PASS=false; P1_DETAIL="pod state: ${STATE}"
TOKEN=$(kubectl get aiplacementdecision "inference-positive-${NS}" -n "$NS" -o jsonpath="{.metadata.annotations.ai\.sovereign\.io/placement-token}" 2>/dev/null)
if [[ "$STATE" == *":${NODE}" ]] && [ -n "$TOKEN" ]; then
  echo "$TOKEN" > "${OUT_DIR}/p1_token.json"
  if [ -n "$PUBKEY_HEX" ] && command -v "$VERIFY_PLACEMENT" >/dev/null 2>&1; then
    POD_UID=$(kubectl get pod inference-positive -n "$NS" -o jsonpath='{.metadata.uid}')
    if "$VERIFY_PLACEMENT" -token-file "${OUT_DIR}/p1_token.json" -pubkey-hex "$PUBKEY_HEX" -pod-uid "$POD_UID" -node "$NODE" > "${OUT_DIR}/p1_verify.txt" 2>&1; then
      P1_PASS=true; P1_DETAIL="pod bound to ${NODE}; AIPlacementDecision present; verify-placement PASS (pod-uid + node checked)"
    else
      P1_DETAIL="pod bound but verify-placement FAILED: $(cat "${OUT_DIR}/p1_verify.txt")"
    fi
  else
    P1_DETAIL="pod bound, token present, but verify-placement/pubkey unavailable"
  fi
fi
write_result p1_positive_e2e.json P1 "Positive e2e: policy+evidence+pod → bound + verifiable token" \
  "POD_BOUND_AND_TOKEN_VERIFIES" "$( [ "$P1_PASS" = true ] && echo POD_BOUND_AND_TOKEN_VERIFIES || echo "$STATE" )" "$P1_PASS" "$P1_DETAIL"

# Scheduler metrics snapshot
( kubectl port-forward -n "$PLATFORM_NS" deploy/attestation-scheduler 18088:8088 >/dev/null 2>&1 &
  PF=$!; sleep 3
  curl -s http://127.0.0.1:18088/metrics | grep -E '^(controller_runtime_reconcile_total|workqueue_)' | head -20 > "${OUT_DIR}/p1_scheduler_metrics.txt"
  kill $PF 2>/dev/null )

# ─── LA1: fake node label without evidence ───────────────────────────────────
NS=article1-a1
make_ns "$NS" "a1"
apply_policy "$NS" "a1"
kubectl label node "$NODE" ai.sovereign.io/attested=true --overwrite >/dev/null
kubectl label node "$NODE" ai.sovereign.io/simulated-evidence=true --overwrite >/dev/null
pod_yaml "$NS" attack1-fake-label "fake-evidence-name" | kubectl apply -f - >/dev/null
STATE=$(wait_pod_bound "$NS" attack1-fake-label)
kubectl label node "$NODE" ai.sovereign.io/attested- ai.sovereign.io/simulated-evidence- >/dev/null 2>&1
PASS=false; [[ "$STATE" == Pending:* || "$STATE" == Absent:* ]] && PASS=true
write_result a1_fake_label.json A1 "Fake attested labels on node, no AttestationEvidence" \
  "BLOCKED_PENDING" "$STATE" "$PASS" "scheduler must ignore node labels; only AttestationEvidence counts"

# ─── LA2: expired evidence ───────────────────────────────────────────────────
NS=article1-a2
make_ns "$NS" "a2"
apply_policy "$NS" "a2" "SEV-SNP" 5
apply_evidence "$NS" ev-old
wait_evidence_verified "$NS" ev-old || true
echo "  waiting 12s for evidence to exceed maxEvidenceAgeSeconds=5..."
sleep 12
pod_yaml "$NS" attack2-expired ev-old | kubectl apply -f - >/dev/null
STATE=$(wait_pod_bound "$NS" attack2-expired)
PASS=false; [[ "$STATE" == Pending:* ]] && PASS=true
write_result a2_expired_evidence.json A2 "Evidence older than policy maxEvidenceAgeSeconds" \
  "BLOCKED_PENDING" "$STATE" "$PASS" "evidence age exceeded 5s freshness limit before pod creation"

# ─── LA3: revoked evidence ───────────────────────────────────────────────────
NS=article1-a3
make_ns "$NS" "a3"
apply_policy "$NS" "a3"
apply_evidence "$NS" ev-revoked
wait_evidence_verified "$NS" ev-revoked || true
kubectl apply -f - <<EOF >/dev/null
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRevocationPolicy
metadata: {name: revoke-ev, namespace: ${NS}}
spec:
  target:
    namespaceSelector:
      matchLabels: {article1: "a3"}
  evidenceRef: {name: ev-revoked}
  reasons: ["article1 attack A3 — deliberate revocation"]
  ttlSeconds: 3600
EOF
# wait until operator marks evidence revoked
DEADLINE=$(( $(date +%s) + 30 ))
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  R=$(kubectl get attestationevidence ev-revoked -n "$NS" -o jsonpath='{.status.revoked}' 2>/dev/null)
  [ "$R" = "true" ] && break
  # nudge a reconcile
  kubectl annotate attestationevidence ev-revoked -n "$NS" nudge="$(date +%s)" --overwrite >/dev/null 2>&1
  sleep 2
done
REVOKED_STATE=$(kubectl get attestationevidence ev-revoked -n "$NS" -o jsonpath='{.status.revoked}' 2>/dev/null)
pod_yaml "$NS" attack3-revoked ev-revoked | kubectl apply -f - >/dev/null
STATE=$(wait_pod_bound "$NS" attack3-revoked)
PASS=false; [[ "$STATE" == Pending:* && "$REVOKED_STATE" == "true" ]] && PASS=true
write_result a3_revoked_evidence.json A3 "Revoked evidence via AIRevocationPolicy" \
  "BLOCKED_PENDING" "$STATE" "$PASS" "evidence status.revoked=${REVOKED_STATE}; scheduler must refuse revoked evidence"

# ─── LA4: wrong TEE ──────────────────────────────────────────────────────────
NS=article1-a4
make_ns "$NS" "a4"
apply_policy "$NS" "a4" "TDX" 300 "kata-qemu-tdx"
apply_evidence "$NS" ev-wrong-tee "SEV-SNP"
wait_evidence_verified "$NS" ev-wrong-tee || true
pod_yaml "$NS" attack4-wrong-tee ev-wrong-tee | kubectl apply -f - >/dev/null
STATE=$(wait_pod_bound "$NS" attack4-wrong-tee)
PASS=false; [[ "$STATE" == Pending:* ]] && PASS=true
write_result a4_wrong_tee.json A4 "Evidence TEE=SEV-SNP but policy requires TDX" \
  "BLOCKED_PENDING" "$STATE" "$PASS" "TEE mismatch must exclude the node"

# ─── LA5: disallowed RuntimeClass (admission denial) ─────────────────────────
NS=article1-a5
make_ns "$NS" "a5"
apply_policy "$NS" "a5"
apply_evidence "$NS" ev-a5
wait_evidence_verified "$NS" ev-a5 || true
DENY_MSG=$(pod_yaml "$NS" attack5-bad-runtime ev-a5 "runc" | kubectl apply -f - 2>&1)
CREATED=$(kubectl get pod attack5-bad-runtime -n "$NS" --no-headers 2>/dev/null | wc -l)
PASS=false; [ "$CREATED" = "0" ] && PASS=true
write_result a5_bad_runtimeclass.json A5 "Pod requests runtimeClass runc not allowed by policy" \
  "ADMISSION_DENIED" "$( [ "$CREATED" = "0" ] && echo ADMISSION_DENIED || echo POD_CREATED )" "$PASS" \
  "validating webhook response: $(echo "$DENY_MSG" | head -c 300)"

# ─── LA6: policy modified between admission and bind ─────────────────────────
NS=article1-a6
make_ns "$NS" "a6"
apply_policy "$NS" "a6"
# No evidence yet: pod admitted (policy-hash annotation frozen), scheduler polls in permit
pod_yaml "$NS" attack6-policy-race ev-a6 | kubectl apply -f - >/dev/null
sleep 2
# Mutate the policy während permit window → hash changes
kubectl patch confidentialinferencepolicy article1-policy -n "$NS" --type=merge \
  -p '{"spec":{"maxEvidenceAgeSeconds":600}}' >/dev/null
# Now provide valid evidence so Filter passes; PreBind must detect hash mismatch
apply_evidence "$NS" ev-a6
STATE=$(wait_pod_bound "$NS" attack6-policy-race)
SCHED_LOG=$(kubectl logs -n "$PLATFORM_NS" deploy/attestation-scheduler --since=2m 2>/dev/null | grep -c "policy hash mismatch" || true)
PASS=false; [[ "$STATE" == Pending:* ]] && PASS=true
write_result a6_policy_modified_race.json A6 "Policy mutated between admission and bind" \
  "BLOCKED_PENDING_PREBIND" "$STATE" "$PASS" \
  "prebind must fail-close on policy hash mismatch; scheduler log matches: ${SCHED_LOG}"

# ─── LA7: evidence revoked during scheduling window ──────────────────────────
NS=article1-a7
make_ns "$NS" "a7"
apply_policy "$NS" "a7"
pod_yaml "$NS" attack7-revoke-race ev-a7 | kubectl apply -f - >/dev/null
sleep 1
apply_evidence "$NS" ev-a7
kubectl apply -f - <<EOF >/dev/null
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRevocationPolicy
metadata: {name: revoke-race, namespace: ${NS}}
spec:
  target:
    namespaceSelector:
      matchLabels: {article1: "a7"}
  evidenceRef: {name: ev-a7}
  reasons: ["article1 attack A7 — revocation during scheduling"]
  ttlSeconds: 3600
EOF
kubectl annotate attestationevidence ev-a7 -n "$NS" nudge=1 --overwrite >/dev/null 2>&1
STATE=$(wait_pod_bound "$NS" attack7-revoke-race)
PASS=false; [[ "$STATE" == Pending:* ]] && PASS=true
write_result a7_revoked_during_scheduling.json A7 "Evidence revoked while pod is in scheduling window" \
  "BLOCKED_PENDING" "$STATE" "$PASS" \
  "revocation raced against permit loop; fail-closed behavior required (filter or prebind)"

# ─── LA8: tampered placement token ───────────────────────────────────────────
PASS=false; OBSERVED="NO_TOKEN"; DETAIL="requires P1 token"
if [ -s "${OUT_DIR}/p1_token.json" ] && [ -n "$PUBKEY_HEX" ]; then
  python3 - "${OUT_DIR}/p1_token.json" "${OUT_DIR}/a8_tampered_token.json.tmp" <<'PYEOF'
import json, sys
tok = json.load(open(sys.argv[1]))
sig = tok["signature"]
tok["signature"] = ("00" if not sig.startswith("00") else "ff") + sig[2:]
json.dump(tok, open(sys.argv[2], "w"))
PYEOF
  if "$VERIFY_PLACEMENT" -token-file "${OUT_DIR}/a8_tampered_token.json.tmp" -pubkey-hex "$PUBKEY_HEX" > "${OUT_DIR}/a8_verify.txt" 2>&1; then
    OBSERVED="ACCEPTED"; DETAIL="tampered token was accepted — FAILURE"
  else
    OBSERVED="REJECTED"; PASS=true; DETAIL="verify-placement rejected tampered signature: $(cat "${OUT_DIR}/a8_verify.txt")"
  fi
  rm -f "${OUT_DIR}/a8_tampered_token.json.tmp"
fi
write_result a8_tampered_token.json A8 "Placement token signature tampered" \
  "REJECTED" "$OBSERVED" "$PASS" "$DETAIL"

# ─── LA9: simulated RuntimeClass refused in production mode ──────
