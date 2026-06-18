# Fonctionnalité — Moteur de coût (`internal/costengine`)

Calcule la décomposition des coûts à partir des `UsageSample` et d'un price-book. **Pur** (sans
dépendance Kubernetes), couverture de tests ~95 %.

## API
```go
type TokenPricing struct { Currency string; InputPerMillion, OutputPerMillion float64 }
type PriceBook map[string]TokenPricing // clé = modelName
func Compute(samples []collectors.UsageSample, prices PriceBook) Breakdown
func TopByCost(m map[string]LineItem, n int) []LineItem
```

## Calcul
`coût = tokens / 1_000_000 × prixParMillion`, pour l'input et l'output.

`Breakdown` fournit :
- `Total` (requêtes, tokens, coûts) ;
- ventilations `ByModel`, `ByProvider`, `ByNamespace`, `ByTeam`, `ByApplication` ;
- `AvgCostPerRequest()`, `CostPerToken()` ;
- `UnpricedModels` — modèles vus sans pricing (coût 0 + recommandation data-quality).

## Construction du price-book
Le controller (`catalog.priceBook`) **seed** d'abord le [catalogue par défaut](catalog.md) (prix
de liste publics des modèles connus) puis **superpose** le pricing de chaque
[AIModel](../crds/aimodel.md)→[AIProvider](../crds/aiprovider.md) : le CR utilisateur gagne, les
défauts comblent les trous → le coût se calcule **out-of-the-box** sans déclarer chaque modèle.
Devise prise du premier modèle pricé ; **pas de conversion multi-devise** en MVP (documenté dans
le rapport).

Consommé par : [AIFinOpsReport](../crds/aifinopsreport.md), [budgetengine](budgetengine.md),
[breakevenengine](breakevenengine.md).
