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
Le controller (`catalog.priceBook`) résout chaque [AIModel](../crds/aimodel.md) vers le pricing de
son [AIProvider](../crds/aiprovider.md). Devise prise du premier modèle pricé ; **pas de conversion
multi-devise** en MVP (documenté dans le rapport).

Consommé par : [AIFinOpsReport](../crds/aifinopsreport.md), [budgetengine](budgetengine.md),
[breakevenengine](breakevenengine.md).
