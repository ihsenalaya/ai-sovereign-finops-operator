#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HELPER="$ROOT/article3/tools/test_matrix_evidence.py"
OPERATOR="$ROOT/operateur"
REQUIRED=(format go_test go_race_govar go_vet lint unit_reservations transactions idempotence envtest postgres_integration gateway_e2e helm_lint helm_install helm_upgrade helm_rollback helm_uninstall sbom vulnerability_scan secret_scan)
POSTGRES_IMAGE="postgres:16@sha256:be01cf82fc7dbba824acf0a82e150b4b360f3ff93c6631d7844af431e841a95c"

internal_kind_env() {
  [[ -n "${KIND_CLUSTER:-}" ]] || { echo "KIND_CLUSTER must name an existing Kind cluster" >&2; return 2; }
  local profile
  case "$KIND_CLUSTER" in
    article3-dev) profile=dev ;;
    article3-validation) profile=validation ;;
    article3-performance) profile=performance ;;
    *) echo "refusing unscoped Kind cluster: $KIND_CLUSTER" >&2; return 2 ;;
  esac
  (
    PROFILE="$profile" CLUSTER_NAME="$KIND_CLUSTER"
    # shellcheck source=../infra/kind/common.sh
    source "$ROOT/article3/infra/kind/common.sh"
    resolve_profile
    prepare_state_dir
    verify_owned
  )
  kind get clusters | grep -Fxq "$KIND_CLUSTER" || { echo "Kind cluster not found: $KIND_CLUSTER" >&2; return 2; }
  local kubeconfig="$1"
  kind get kubeconfig --name "$KIND_CLUSTER" >"$kubeconfig"
  export KUBECONFIG="$kubeconfig"
  kubectl config current-context | grep -Fxq "kind-$KIND_CLUSTER"
  kubectl get --raw=/readyz >/dev/null
}

if [[ "${1:-}" == "--internal-postgres" ]]; then
  name="article3-pg-${ARTICLE3_RUN_ID_SAFE:-manual}-$$"
  cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
  trap cleanup EXIT INT TERM
  docker pull "$POSTGRES_IMAGE"
  docker run -d --rm --name "$name" -e POSTGRES_USER=govar -e POSTGRES_PASSWORD=govar-test-only -e POSTGRES_DB=govar \
    -p 127.0.0.1::5432 --health-cmd='pg_isready -U govar -d govar' --health-interval=1s --health-timeout=3s --health-retries=60 "$POSTGRES_IMAGE"
  for _ in $(seq 1 90); do
    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$name" 2>/dev/null || true)" == healthy ]] && break
    sleep 1
  done
  [[ "$(docker inspect -f '{{.State.Health.Status}}' "$name")" == healthy ]]
  port="$(docker port "$name" 5432/tcp | awk -F: 'NR==1 {print $NF}')"
  [[ "$port" =~ ^[0-9]+$ ]]
  cd "$OPERATOR"
  output="$(GOVAR_TEST_DATABASE_URL="postgres://govar:govar-test-only@127.0.0.1:${port}/govar?sslmode=disable" \
    go test -count=1 -v -run 'TestPostgresEngine(RejectsEmptyURL|RejectsUnreconciledLegacyFloatLedger|Lifecycle)$' ./internal/govar 2>&1)" || { rc=$?; printf '%s\n' "$output"; exit "$rc"; }
  printf '%s\n' "$output"
  grep -Fq -- '--- PASS: TestPostgresEngineRejectsUnreconciledLegacyFloatLedger' <<<"$output"
  grep -Fq -- '--- PASS: TestPostgresEngineLifecycle' <<<"$output"
  exit 0
fi

if [[ "${1:-}" == "--internal-envtest" ]]; then
  cd "$OPERATOR"
  make envtest ENVTEST_VERSION=v0.24.1
  assets="$(./bin/setup-envtest-v0.24.1 use 1.31.0 --bin-dir "$OPERATOR/bin" -p path)" || exit $?
  [[ "$assets" == "$OPERATOR/bin/k8s/1.31.0-"* && -x "$assets/kube-apiserver" && -x "$assets/etcd" ]] || {
    echo "invalid envtest assets: $assets" >&2
    exit 1
  }
  KUBEBUILDER_ASSETS="$assets" go test -count=1 ./internal/controller
  exit $?
fi

if [[ "${1:-}" == "--internal-gateway-e2e" ]]; then
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT INT TERM
  internal_kind_env "$tmp/kubeconfig" || exit $?
  cp -a "$OPERATOR" "$tmp/operateur"
  cd "$tmp/operateur"
  go test -count=1 ./internal/sidecarproxy ./internal/govarextproc ./cmd/gov-ar-admission
  GOVAR_REAL_ENVOY=1 go test -count=1 -v \
    -run '^TestRealEnvoyRoutesOnlyToAdmissionSelectedBackend$' ./internal/govarextproc
  ARTICLE3_RUN_KIND_E2E=1 KIND_CLUSTER="$KIND_CLUSTER" go test -count=1 ./test/e2e/ -v -ginkgo.v
  exit $?
fi

if [[ "${1:-}" == "--internal-helm-lint" ]]; then
  chart="$OPERATOR/charts/ai-sovereign-finops-operator"
  helm lint "$chart"
  helm template phase-d-dev "$chart" \
    --set govArAdmission.enabled=true \
    --set govArAdmission.devInMemory=true \
    --set govArAdmission.replicaCount=1 \
    --set govArAdmission.identity.masterExistingSecret=govar-master >/dev/null
  helm template phase-d-prod "$chart" \
    --set govArAdmission.enabled=true \
    --set govArAdmission.identity.masterExistingSecret=govar-master \
    --set govArAdmission.postgres.enabled=true \
    --set govArAdmission.postgres.existingSecret=govar-db \
    --set govArAdmission.softwareSHA256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
    --set govArAdmission.enforcement.networkPolicy.enabled=true \
    --set govArAdmission.enforcement.networkPolicy.governedNamespace=article3-workloads \
    --set govArAdmission.tracing.enabled=true \
    --set govArAdmission.tracing.endpoint=http://otel-collector.article3-observability.svc:4318 \
    --set govArAdmission.tracing.insecure=true \
    --set govArAdmission.extProc.tls.serverExistingSecret=govar-ext-proc-server \
    --set govArAdmission.extProc.tls.clientCAExistingSecret=govar-ext-proc-client-ca \
    --set govArAdmission.extProc.gatewaySPIFFEID=spiffe://govar.local/gateway/envoy >/dev/null
  if helm template invalid-prod "$chart" --set govArAdmission.enabled=true >/dev/null 2>&1; then echo "invalid production configuration rendered" >&2; exit 1; fi
  if helm template invalid-replicas "$chart" --set govArAdmission.enabled=true --set govArAdmission.devInMemory=true --set govArAdmission.replicaCount=2 >/dev/null 2>&1; then echo "invalid in-memory replica configuration rendered" >&2; exit 1; fi
  exit 0
fi

if [[ "${1:-}" == --internal-helm-* ]]; then
  action="${1#--internal-helm-}"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT INT TERM
  internal_kind_env "$tmp/kubeconfig" || exit $?
  release="article3-phase-d"; namespace="article3-phase-d"; chart="$OPERATOR/charts/ai-sovereign-finops-operator"
  tag="${ARTICLE3_SOURCE_SHA256:0:12}"; repository="article3-phase-d-controller"; image="$repository:$tag"
  case "$action" in
    install)
      helm uninstall "$release" -n "$namespace" --wait >/dev/null 2>&1 || true
      kubectl delete namespace "$namespace" --ignore-not-found --wait=true --timeout=120s >/dev/null
      docker build -t "$image" "$OPERATOR"
      kind load docker-image "$image" --name "$KIND_CLUSTER"
      helm install "$release" "$chart" -n "$namespace" --create-namespace --wait --timeout 5m \
        --set image.repository="$repository" --set image.tag="$tag" --set image.pullPolicy=Never
      kubectl -n "$namespace" rollout status deployment/"$release-ai-sovereign-finops-operator" --timeout=180s
      [[ "$(helm status "$release" -n "$namespace" -o json | python3 -c 'import json,sys; print(json.load(sys.stdin)["info"]["status"])')" == deployed ]]
      ;;
    upgrade)
      helm status "$release" -n "$namespace" >/dev/null
      helm upgrade "$release" "$chart" -n "$namespace" --reuse-values --wait --timeout 5m --set leaderElection.enabled=true
      [[ "$(helm history "$release" -n "$namespace" -o json | python3 -c 'import json,sys; print(json.load(sys.stdin)[-1]["revision"])')" == 2 ]]
      ;;
    rollback)
      helm status "$release" -n "$namespace" >/dev/null
      helm rollback "$release" 1 -n "$namespace" --wait --timeout 5m
      [[ "$(helm history "$release" -n "$namespace" -o json | python3 -c 'import json,sys; print(json.load(sys.stdin)[-1]["status"])')" == deployed ]]
      [[ "$(helm get values "$release" -n "$namespace" -o json | python3 -c 'import json,sys; print(json.load(sys.stdin).get("leaderElection",{}).get("enabled",False))')" == False ]]
      ;;
    uninstall)
      helm uninstall "$release" -n "$namespace" --wait --timeout 5m
      if helm status "$release" -n "$namespace" >/dev/null 2>&1; then echo "release still exists" >&2; exit 1; fi
      kubectl delete namespace "$namespace" --ignore-not-found --wait=true --timeout=120s >/dev/null
      ;;
    *) exit 2 ;;
  esac
  exit 0
fi

if [[ "${1:-}" == "--internal-image-scan" ]]; then
  mode="${2:?scan mode required}"; output="${3:-}"
  image="article3-phase-d-scan:${ARTICLE3_SOURCE_SHA256:0:12}"
  docker image inspect "$image" >/dev/null 2>&1 || docker build -t "$image" "$OPERATOR"
  case "$mode" in
    sbom)
      trivy image --quiet --format cyclonedx --output "$output" "$image"
      python3 - "$output" <<'PY'
import json, pathlib, sys
p=pathlib.Path(sys.argv[1]); d=json.loads(p.read_text())
assert p.stat().st_size > 100 and d.get("bomFormat") == "CycloneDX" and isinstance(d.get("components"), list)
PY
      ;;
    vulnerability)
      trivy image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 "$image"
      ;;
    *) exit 2 ;;
  esac
  exit 0
fi

if [[ "${1:-}" == "--internal-secret-scan" ]]; then
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT INT TERM
  cd "$ROOT"
  while IFS= read -r -d '' file; do
    [[ "$file" == propmt.txt || "$file" == article3/archive/* || "$file" == article3/reports/* || "$file" == operateur/bin/* ]] && continue
    [[ -f "$file" ]] || continue
    mkdir -p "$tmp/$(dirname "$file")"; cp -- "$file" "$tmp/$file"
  done < <(git ls-files -z --cached --others --exclude-standard)
  trivy fs --quiet --scanners secret --format json --output "$tmp/trivy-secret.json" "$tmp"
  python3 - "$tmp/trivy-secret.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); findings=[]
for result in d.get("Results", []):
    for item in result.get("Secrets", []) or []:
        findings.append({"target": result.get("Target"), "rule": item.get("RuleID"),
                         "category": item.get("Category"), "severity": item.get("Severity")})
print(json.dumps({"secret_finding_count": len(findings), "findings": findings}, sort_keys=True))
raise SystemExit(1 if findings else 0)
PY
  exit $?
fi

usage() { echo "usage: $0 [--resume] [--only NAME[,NAME...]] [--run-id ID] [--validate] [--list]"; }
resume=false; only=""; validate=false; list=false; run_id="${ARTICLE3_TEST_RUN_ID:-phase-d-$(date -u +%Y%m%dT%H%M%SZ)}"
while (($#)); do
  case "$1" in
    --resume) resume=true ;;
    --only) shift; only="${1:?check list required}" ;;
    --run-id) shift; run_id="${1:?run id required}" ;;
    --validate) validate=true ;;
    --list) list=true ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
  shift
done
[[ "$run_id" =~ ^[A-Za-z0-9._-]+$ ]] || { echo "unsafe run id" >&2; exit 2; }
$list && { printf '%s\n' "${REQUIRED[@]}"; exit 0; }

if $validate; then
  bash -n "$ROOT/article3/tools/run_test_matrix.sh"
  python3 -m py_compile "$HELPER"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  python3 "$HELPER" fingerprint --root "$ROOT" >"$tmp/fingerprint.json"
  python3 "$HELPER" build-report --root "$ROOT" --run-dir "$tmp" --run-id validation --output "$tmp/report.json" >/dev/null 2>&1 || true
  python3 - "$tmp/report.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); assert d["passed_count"] == 0 and all(x["status"] == "not_run" for x in d["checks"])
PY
  echo "Phase D harness validation passed; no tests were marked passed"
  exit 0
fi

branch="$(git -C "$ROOT" branch --show-current)"
[[ "$branch" == article3-q1-recovery-* ]] || { echo "refusing branch: $branch" >&2; exit 2; }
propmt_hash=""; [[ -f "$ROOT/propmt.txt" ]] && propmt_hash="$(sha256sum "$ROOT/propmt.txt" | awk '{print $1}')"
RUN_DIR="$ROOT/article3/reports/test-matrix/$run_id"; mkdir -p "$RUN_DIR/logs" "$RUN_DIR/results"
fingerprint_json="$(python3 "$HELPER" fingerprint --root "$ROOT")"
source_sha="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["source_sha256"])' <<<"$fingerprint_json")"
printf '%s\n' "$fingerprint_json" >"$RUN_DIR/source_manifest.json"
export ARTICLE3_SOURCE_SHA256="$source_sha" ARTICLE3_RUN_ID_SAFE="${run_id//[^A-Za-z0-9_.-]/-}"

selected() { [[ -z "$only" || ",$only," == *",$1,"* ]]; }
run_check() {
  local name="$1" timeout="$2" cwd="$3"; shift 3
  selected "$name" || return 0
  if $resume && python3 "$HELPER" checkpoint --root "$ROOT" --name "$name" --result "$RUN_DIR/results/$name.json" --source-sha256 "$source_sha"; then
    echo "SKIP $name (verified source/log checkpoint)"; return 0
  fi
  echo "RUN  $name"
  python3 "$HELPER" run --root "$ROOT" --name "$name" --run-id "$run_id" --cwd "$cwd" \
    --log "$RUN_DIR/logs/$name.log" --result "$RUN_DIR/results/$name.json" --source-sha256 "$source_sha" --timeout "$timeout" -- "$@" || true
}

run_check format 300 "$OPERATOR" bash -c 'files=$(gofmt -l $(find . -type f -name "*.go" -not -path "./bin/*")); if [[ -n "$files" ]]; then printf "%s\n" "$files"; exit 1; fi; git -C .. diff --check -- AGENTS.md operateur article3'
run_check envtest 900 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-envtest
run_check go_test 1800 "$OPERATOR" bash -c 'packages=$(go list ./... | grep -vE "/(test/e2e|internal/controller)$"); go test -count=1 $packages'
run_check go_race_govar 1800 "$OPERATOR" go test -count=1 -race ./internal/govar/...
run_check go_vet 900 "$OPERATOR" go vet ./...
run_check lint 1200 "$OPERATOR" make lint GOLANGCI_LINT_VERSION=v2.12.0
run_check unit_reservations 600 "$OPERATOR" go test -count=1 -run 'Test(QuantityToMicros|ExpectedCostMicros|ExecutableReservationPolicies|DriftFallsBack|JointSelection|StrictRoughInput|RoughInputForces|LatencyObjective|HardFeasibility)' ./internal/govar
run_check transactions 900 "$OPERATOR" go test -count=1 -run 'TestEngine(ReserveDispatch|PendingCancellation|RejectsSettlement|FailsClosedOnBudget|AmbiguousCancel|PostFinality|ConcurrentReservations)' ./internal/govar
run_check idempotence 900 "$OPERATOR" go test -count=1 -run 'TestEngine(RejectsCrossPrincipal|RejectsStale|RejectsSameEventID|DuplicateAdmission|DeterministicCandidate)' ./internal/govar
run_check postgres_integration 1200 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-postgres
run_check gateway_e2e 1800 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-gateway-e2e
run_check helm_lint 300 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-helm-lint

helm_requested=false; for n in helm_install helm_upgrade helm_rollback helm_uninstall; do selected "$n" && helm_requested=true; done
if $helm_requested; then
  helm_all_valid=true
  for n in helm_install helm_upgrade helm_rollback helm_uninstall; do
    python3 "$HELPER" checkpoint --root "$ROOT" --name "$n" --result "$RUN_DIR/results/$n.json" --source-sha256 "$source_sha" || helm_all_valid=false
  done
  if ! $resume || ! $helm_all_valid; then
    saved_resume=$resume; resume=false
    run_check helm_install 1200 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-helm-install
    run_check helm_upgrade 600 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-helm-upgrade
    run_check helm_rollback 600 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-helm-rollback
    run_check helm_uninstall 600 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-helm-uninstall
    resume=$saved_resume
  else
    echo "SKIP Helm lifecycle (four verified source/log checkpoints)"
  fi
fi

run_check sbom 1200 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-image-scan sbom "$RUN_DIR/sbom.cdx.json"
run_check vulnerability_scan 1200 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-image-scan vulnerability
run_check secret_scan 900 "$ROOT" bash article3/tools/run_test_matrix.sh --internal-secret-scan

report_rc=0
python3 "$HELPER" build-report --root "$ROOT" --run-dir "$RUN_DIR" --run-id "$run_id" --output "$ROOT/article3/reports/test_results.json" || report_rc=$?
if [[ -n "$propmt_hash" ]]; then
  [[ -f "$ROOT/propmt.txt" && "$(sha256sum "$ROOT/propmt.txt" | awk '{print $1}')" == "$propmt_hash" ]] || { echo "propmt.txt changed" >&2; exit 126; }
fi
echo "Phase D report: article3/reports/test_results.json"
exit "$report_rc"
