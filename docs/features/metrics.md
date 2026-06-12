# Fonctionnalité — Observabilité / métriques (`internal/metrics`)

Enregistre les métriques `ai_finops_*` sur le registre **controller-runtime**, exposées via
l'endpoint `/metrics` du manager (port `:8080` par défaut). Stack CNCF : Prometheus + Grafana.

## Métriques exposées
| Métrique | Type | Labels | Source |
|----------|------|--------|--------|
| `ai_finops_requests` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_input_tokens` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_output_tokens` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_cost_eur` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_cost_by_zone_eur` | gauge | `zone` | AIFinOpsReport |
| `ai_finops_budget_usage_percent` | gauge | `namespace,policy` | AIBudgetPolicy |
| `ai_finops_sovereignty_findings` | gauge | `namespace,application,policy,severity` | AISovereigntyPolicy, AIFinOpsReport |
| `ai_finops_sovereignty_requests` | gauge | `namespace,application,policy,severity` | AIFinOpsReport |
| `ai_finops_enforcement_actions` | gauge | `policy,namespace,application,mode,action,actuated` | AISovereigntyPolicy, AIBudgetPolicy |
| `ai_finops_shadow_ai_egress` | gauge | `namespace,application,zone,provider,severity` | AISovereigntyPolicy (plan eBPF) |
| `ai_finops_breakeven_savings_eur` | gauge | `namespace,analysis` | AIBreakEvenAnalysis |
| `ai_finops_recommendations` | gauge | `type,namespace,application,severity` | AIFinOpsReport |
| `ai_finops_potential_savings_eur` | gauge | (none) | AIFinOpsReport (recommendation engine) |
| `ai_finops_potential_savings_by_app_eur` | gauge | `namespace,application` | AIFinOpsReport (recommendation engine) |
| `ai_finops_cost_saving_eur` | gauge | `namespace,application,current_model,recommended_model` | AIFinOpsReport |
| `ai_finops_projected_monthly_cost_eur` | gauge | `namespace` | AIFinOpsReport |

> Ce sont des **agrégats dérivés par fenêtre de reporting** (positionnés à chaque réconciliation),
> donc des **gauges** — et non des compteurs monotones. Les noms évitent désormais le suffixe `_total`
> (convention Prometheus : `_total` = counter ; l'apposer sur une gauge casse `rate()`/`increase()`).
> Les compteurs cumulés côté gateway gardent `_total` dans leurs propres exporters.
>
> **Pas de séries fantômes** : chaque report/policy ne purge que **ses propres** séries (via un tracker
> par-UID + finalizer), au lieu d'un `Reset()` global qui effaçait les séries des autres objets.
> `ai_finops_enforcement_actions` (mode `report`/`warn`/`reroute`/`block`, `actuated` true/false) reflète
> la **décision d'enforcement** prise pour chaque workload/policy, qu'elle vienne d'une policy de
> souveraineté ou d'un fallback budgétaire — voir `docs/crds/aisovereigntypolicy.md` et
> `docs/crds/aibudgetpolicy.md`.
>
> `ai_finops_shadow_ai_egress` est le **plan de souveraineté indépendant de la gateway** : il compte les
> connexions egress (observées par eBPF/Tetragon) vers un endpoint LLM connu dans une zone non conforme —
> c.-à-d. le **shadow-AI** qui contourne la gateway. Voir [`shadowengine.md`](shadowengine.md).

## Accès
```bash
kubectl -n greenops-system port-forward svc/<release>-ai-sovereign-finops-operator-metrics 8080:8080
curl -s localhost:8080/metrics | grep ai_finops_
```

## Dashboard
[`dashboards/ai-finops-overview.json`](../../dashboards/ai-finops-overview.json) : coût total, budget
%, findings critiques, coût/tokens par namespace, dépense par zone de souveraineté, recommandations
cost-saving (action + gain €) et **décisions d'enforcement** (par workload / mode / action).
Un `ServiceMonitor` (Prometheus Operator) est activable via `metrics.serviceMonitor.enabled` du chart.
