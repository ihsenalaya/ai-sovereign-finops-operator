# Fonctionnalité — Observabilité / métriques (`internal/metrics`)

Enregistre les métriques `ai_finops_*` sur le registre **controller-runtime**, exposées via
l'endpoint `/metrics` du manager (port `:8080` par défaut). Stack CNCF : Prometheus + Grafana.

## Métriques exposées
| Métrique | Type | Labels | Source |
|----------|------|--------|--------|
| `ai_finops_requests_total` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_input_tokens_total` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_output_tokens_total` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_cost_eur_total` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_budget_usage_percent` | gauge | `namespace,policy` | AIBudgetPolicy |
| `ai_finops_sovereignty_findings_total` | gauge | `namespace,application,policy,severity` | AISovereigntyPolicy, AIFinOpsReport |
| `ai_finops_breakeven_savings_eur` | gauge | `namespace,analysis` | AIBreakEvenAnalysis |
| `ai_finops_recommendations_total` | gauge | `type,namespace,application,severity` | AIFinOpsReport |
| `ai_finops_potential_savings_eur` | gauge | (none) | AIFinOpsReport (recommendation engine) |
| `ai_finops_potential_savings_by_app_eur` | gauge | `namespace,application` | AIFinOpsReport (recommendation engine) |
| `ai_finops_projected_monthly_cost_eur` | gauge | `namespace` | AIFinOpsReport |
| `ai_finops_sovereignty_requests_total` | gauge | `namespace,application,policy,severity` | AIFinOpsReport |

> Ce sont des **agrégats dérivés** (positionnés à chaque réconciliation), modélisés en gauges même
> quand le nom porte `_total` (conservé pour coller à la spec produit et refléter les compteurs
> côté gateway).

## Accès
```bash
kubectl -n greenops-system port-forward svc/<release>-ai-sovereign-finops-operator-metrics 8080:8080
curl -s localhost:8080/metrics | grep ai_finops_
```

## Dashboard
[`dashboards/ai-finops-overview.json`](../../dashboards/ai-finops-overview.json) : coût total, budget
%, findings critiques, coût/tokens par namespace, économies break-even, recommandations par type.
Un `ServiceMonitor` (Prometheus Operator) est activable via `metrics.serviceMonitor.enabled` du chart.
