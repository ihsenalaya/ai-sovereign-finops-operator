# CRD — AIBudgetPolicy

`aiops.imperium.io/v1alpha1`, namespaced. Short name : `aibudget`.

Définit un budget IA pour un namespace/équipe/application, avec seuils et actions **recommandées**
(jamais appliquées en MVP).

## Spec
| Champ | Type | Requis | Description |
|-------|------|:------:|-------------|
| `target.namespace` / `target.team` / `target.application` | string | | Portée du budget (filtres cumulatifs). |
| `period` | enum `daily\|weekly\|monthly` | ✔ (défaut `monthly`) | Fenêtre budgétaire. |
| `budgetEUR` | Quantity | ✔ | Plafond sur la période (EUR). |
| `warningThresholdPercent` | int32 (0–100, défaut 70) | | Seuil warning. |
| `criticalThresholdPercent` | int32 (0–100, défaut 90) | | Seuil critical. |
| `hardLimitPercent` | int32 (0–200, défaut 100) | | Seuil hard-limit. |
| `actions.onWarning/onCritical/onHardLimit[]` | []string | | Actions recommandées par palier. |
| `fallbackModelRef` | string | | Modèle moins cher recommandé. |

## Status
`observedGeneration`, `currentSpendEUR`, `usagePercent`, `phase`
(`Unknown\|WithinBudget\|Warning\|Critical\|Exceeded`), `conditions[]`.

## Comportement du controller
Collecte l'usage (collector de la gateway, sinon `fake`), calcule le coût de la cible via
[costengine](../features/costengine.md), évalue avec [budgetengine](../features/budgetengine.md),
écrit `phase`/`usagePercent`/`currentSpendEUR`, expose `ai_finops_budget_usage_percent`, et émet
un Event `Warning` listant les actions recommandées si un palier est franchi. **Aucun blocage.**

## Exemple
[`..._aibudgetpolicy.yaml`](../../config/samples/aiops_v1alpha1_aibudgetpolicy.yaml).

```bash
kubectl get aibudget rh-budget   # colonnes BUDGET(EUR) USAGE% PHASE
```
