# Fonctionnalité — Moteur de budget (`internal/budgetengine`)

Compare la dépense à un budget et en déduit une phase + actions recommandées. **Pur**, couverture 100 %.

## API
```go
type Thresholds struct { WarningPct, CriticalPct, HardLimitPct int32 }
type Actions    struct { OnWarning, OnCritical, OnHardLimit []string }
func Evaluate(spend, budget float64, t Thresholds, a Actions) Result
```

## Logique
`usagePercent = round(spend/budget×100)`. Phase :
| Condition | Phase | Actions retournées |
|-----------|-------|--------------------|
| `pct ≥ hardLimit` | `Exceeded` | `onHardLimit` |
| `pct ≥ critical` | `Critical` | `onCritical` |
| `pct ≥ warning` | `Warning` | `onWarning` |
| sinon | `WithinBudget` | — |
| `budget ≤ 0` | `Unknown` | — |

## Intégration
Le controller [AIBudgetPolicy](../crds/aibudgetpolicy.md) calcule la dépense de la cible via
[costengine](costengine.md), appelle `Evaluate`, écrit `phase/usagePercent/currentSpendEUR`, expose
`ai_finops_budget_usage_percent`, émet un Event listant les actions. Quand un `fallbackModelRef`
managé est configuré avec `enforcementMode=enforce`, le controller peut aussi **actuer** un reroute
gateway vers ce fallback sous garde-fous (qualité, télémétrie disponible, modèle non partagé,
coût réellement inférieur). Le moteur de budget, lui, reste pur et ne décide que la phase.
