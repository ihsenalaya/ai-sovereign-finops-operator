#!/usr/bin/env bash
# End-to-end demonstration of the AI Sovereign FinOps Operator on a local kind
# cluster. Brings up the operator (if needed), applies a representative set of
# CRs exercising every engine, prints a guided tour of the results, deploys a
# lean Prometheus + Grafana, and opens the dashboard.
#
# Usage:
#   ./demo.sh up      # deploy everything + guided tour + open Grafana (default)
#   ./demo.sh tour    # just re-print the guided tour of current results
#   ./demo.sh down    # remove demo observability + demo CRs (keeps the operator)
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "${HERE}/../.." && pwd)"
NS="greenops-system"
CTX="${KCTX:-kind-greenops}"
IMG="ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.2.3"
K="kubectl --context ${CTX}"

bold()  { printf '\033[1m%s\033[0m\n' "$*"; }
hr()    { printf '\033[2m%s\033[0m\n' "----------------------------------------------------------------------"; }
step()  { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }

ensure_context() {
  if ! kubectl config get-contexts -o name | grep -qx "${CTX}"; then
    echo "Context ${CTX} not found. Create the cluster first (automatisation/ make local) or set KCTX." >&2
    exit 1
  fi
}

ensure_operator() {
  step "Operator"
  if ! ${K} -n "${NS}" get deploy greenops-ai-sovereign-finops-operator >/dev/null 2>&1; then
    bold "Operator not found — installing via Helm (image ${IMG})"
    if command -v helm >/dev/null; then
      helm --kube-context "${CTX}" upgrade --install greenops "${REPO}/charts/ai-sovereign-finops-operator" \
        --namespace "${NS}" --create-namespace \
        --set image.repository="${IMG%:*}" --set image.tag="${IMG##*:}" --set image.pullPolicy=IfNotPresent
    else
      echo "helm not found and operator not installed; install the operator first." >&2; exit 1
    fi
  fi
  ${K} -n "${NS}" rollout status deploy/greenops-ai-sovereign-finops-operator --timeout=120s
}

seed_usage() {
  step "Real telemetry (live LLM calls -> measured tokens)"
  local key_path="${OPENAI_KEY:-${REPO}/docs/openaikey.txt}"
  local fbase="${FOUNDRY_BASE:-https://greenops-foundry.services.ai.azure.com/models}"
  local fdeploy="${FOUNDRY_DEPLOYMENT:-mistral-large-latest}"
  local fver="${FOUNDRY_API_VERSION:-2024-05-01-preview}"
  local frg="${FOUNDRY_RG:-greenops-rg}"
  local facct="${FOUNDRY_ACCOUNT:-greenops-foundry}"
  local pdays="${PROJECT_DAYS:-1}"   # 1 = seed one day of consumption; operator forecasts the month
  local fkey=""
  if command -v az >/dev/null 2>&1; then
    fkey="$(az cognitiveservices account keys list -g "${frg}" -n "${facct}" --query key1 -o tsv 2>/dev/null || true)"
  fi
  local args=(-openai-key "${key_path}" -calls "${SEED_CALLS:-3}" -project-days "${pdays}" -out /tmp/greenops-usage.json)
  if [ -n "${fkey}" ]; then
    args+=(-foundry-base "${fbase}" -foundry-key "${fkey}" -foundry-deployment "${fdeploy}" -foundry-api-version "${fver}")
    bold "Calling OpenAI (gpt-4o, US) + Mistral-Large (Foundry, EU)..."
  else
    warn "No Foundry key (az not logged in / resource absent) — seeding OpenAI flows only."
  fi
  ( cd "${REPO}" && go run ./cmd/seed-usage "${args[@]}" )
  ${K} -n default create configmap greenops-usage \
    --from-file=usage.json=/tmp/greenops-usage.json --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} label configmap greenops-usage -n default aiops.imperium.io/demo=true --overwrite >/dev/null
  bold "Measured usage -> ConfigMap default/greenops-usage (the operator reads it)."
}

warn() { printf '\033[33mWARN:\033[0m %s\n' "$*" >&2; }

apply_crs() {
  step "Applying CRs (catalogue + policies + reports)"
  ${K} apply -k "${REPO}/config/samples/" >/dev/null
  ${K} apply -f "${HERE}/demo-extra.yaml" >/dev/null
  # Nudge the report/policy/budget reconcilers so statuses are fresh.
  for kind_name in "aifinopsreport/monthly-ai-report-rh" "aifinopsreport/all-flows-report" \
                   "aisovereigntypolicy/regulated-france-policy" "aibudgetpolicy/rh-tight-budget"; do
    ${K} -n default annotate "${kind_name}" demo/reconcile="$(date +%s)" --overwrite >/dev/null 2>&1 || true
  done
  echo "Applied. Waiting a few seconds for reconciliation..."
  sleep 8
}

deploy_observability() {
  step "Observability (Prometheus + Grafana)"
  # (Re)create the dashboard ConfigMap from the repo's dashboard so it stays in sync.
  ${K} -n "${NS}" create configmap demo-grafana-dashboard \
    --from-file=ai-finops-overview.json="${REPO}/dashboards/ai-finops-overview.json" \
    --dry-run=client -o yaml | ${K} apply -f - >/dev/null
  ${K} label configmap demo-grafana-dashboard -n "${NS}" aiops.imperium.io/demo=true --overwrite >/dev/null
  ${K} apply -f "${HERE}/observability.yaml" >/dev/null
  ${K} -n "${NS}" rollout status deploy/demo-prometheus --timeout=120s
  ${K} -n "${NS}" rollout status deploy/demo-grafana --timeout=120s
}

tour() {
  step "GUIDED TOUR — all operator functionalities"

  bold "1) Catalogue & gateway (AIGateway / AIProvider / AIModel)"
  ${K} get aigw,aiprov,aimodel -A
  hr

  bold "2) Cost attribution (AIFinOpsReport .status) — REAL measured tokens, observed + projected"
  ${K} -n default get aifinopsreport -o custom-columns=\
'NAME:.metadata.name,WINDOW:.spec.target.period,IN_TOK:.status.totalInputTokens,OUT_TOK:.status.totalOutputTokens,OBSERVED_EUR:.status.totalCostEUR,PROJECTED_MONTHLY_EUR:.status.projectedMonthlyCostEUR'
  echo "  (observed = real cost of measured tokens over the window; projected = run-rate x 30.4/window)"
  hr

  bold "3) Sovereignty — FLOW-AWARE verification (per namespace/app)"
  echo "All-flows report findings (note the attribution: namespace/app/model/provider/zone/requests):"
  ${K} -n default get aifinopsreport all-flows-report -o jsonpath='{range .status.sovereigntyFindings[*]}{"  - ["}{.severity}{"] "}{.namespace}{"/"}{.application}{" -> "}{.model}{"@"}{.provider}{" ("}{.zone}{") x"}{.requests}{" req\n"}{end}' 2>/dev/null | grep -v -- '  - \[info\] / -> @' || true
  echo
  ${K} -n default get aisovereigntypolicy -o custom-columns='POLICY:.metadata.name,MODE:.spec.enforcementMode,FINDINGS:.status.findingsCount'
  hr

  bold "4) Budget — forecast vs monthly budget (AIBudgetPolicy .status)"
  ${K} -n default get aibudgetpolicy -o custom-columns=\
'NAME:.metadata.name,MONTHLY_BUDGET:.spec.budgetEUR,OBSERVED:.status.currentSpendEUR,PROJECTED_MONTHLY:.status.projectedMonthlySpendEUR,USAGE%:.status.usagePercent,PHASE:.status.phase'
  echo "rh-tight-budget condition:"
  ${K} -n default get aibudgetpolicy rh-tight-budget -o jsonpath='  {.status.conditions[0].message}{"\n"}' 2>/dev/null || true
  hr

  bold "5) Break-even — managed vs self-hosted (AIBreakEvenAnalysis .status)"
  ${K} -n default get aibreakevenanalysis -o custom-columns=\
'NAME:.metadata.name,CURRENT:.spec.currentModelRef,RECO:.status.recommendation,SAVINGS_EUR:.status.monthlySavingsEUR'
  ${K} -n default get aibreakevenanalysis chatbot-rh-analysis -o jsonpath='  {.status.conditions[0].message}{"\n"}' 2>/dev/null || true
  hr

  bold "6) Reporting — Markdown report (ConfigMap <report>-report)"
  echo "Stored in ConfigMap all-flows-report-report (keys: report.md, report.json). Sovereignty section:"
  ${K} -n default get cm all-flows-report-report -o jsonpath='{.data.report\.md}' 2>/dev/null | sed -n '/Sovereignty findings/,/Recommendations/p' | sed 's/^/  /' || true
  hr

  bold "7) Observability — metrics + Grafana"
  echo "  Prometheus metrics: ai_finops_{cost_eur,requests,input/output_tokens,budget_usage_percent,"
  echo "                      sovereignty_findings{namespace,application,severity},breakeven_savings_eur,recommendations}_*"
  echo "  Grafana dashboard 'AI Sovereign FinOps — Overview' (9 panels incl. 'Sovereignty findings by application')."
}

open_grafana() {
  step "Open Grafana"
  bold "Grafana → http://localhost:3000   (anonymous admin; login admin/admin if asked)"
  bold "Prometheus → http://localhost:9090"
  echo "Port-forwarding... press Ctrl-C to stop (the demo stays deployed; run './demo.sh down' to clean up)."
  ${K} -n "${NS}" port-forward svc/demo-prometheus 9090:9090 >/dev/null 2>&1 &
  PF_PROM=$!
  trap 'kill ${PF_PROM} >/dev/null 2>&1 || true' EXIT
  ${K} -n "${NS}" port-forward svc/demo-grafana 3000:3000
}

down() {
  step "Tearing down demo (operator is kept)"
  ${K} -n "${NS}" delete deploy,svc,configmap -l aiops.imperium.io/demo=true --ignore-not-found
  ${K} -n default delete configmap -l aiops.imperium.io/demo=true --ignore-not-found
  ${K} -n default delete -f "${HERE}/demo-extra.yaml" --ignore-not-found
  echo "Done. Operator and base samples remain."
}

main() {
  ensure_context
  case "${1:-up}" in
    up)
      ensure_operator
      seed_usage
      apply_crs
      deploy_observability
      tour
      open_grafana
      ;;
    tour) tour ;;
    down) down ;;
    *) echo "usage: $0 [up|tour|down]" >&2; exit 1 ;;
  esac
}
main "$@"
