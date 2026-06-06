# Fonctionnalité — Moteur break-even (`internal/breakevenengine`)

Compare le coût mensuel d'un modèle managé à une alternative auto-hébergée GPU et calcule le point
mort. **Pur**, couverture ~87 %.

## API
```go
type Inputs struct {
    ManagedTokenCostMonthly, ProviderFixedMonthly float64
    GpuMonthly, OpsMonthly, StorageNetworkMonthly float64
    MigrationCost                                 float64
}
func Analyze(in Inputs, paybackThreshold float64) Result
func ExtrapolateMonthly(windowCost float64, windowDays int32) float64
```

## Modèle (MVP)
```
managed = ManagedTokenCostMonthly + ProviderFixedMonthly
self    = GpuMonthly + OpsMonthly + StorageNetworkMonthly
savings = managed - self
payback = MigrationCost / savings        (si savings > 0 ; 0 si migration nulle)
```

Recommandation :
- `savings ≤ 0` → **keep-managed** ;
- `payback ≤ seuil` (défaut 6 mois) → **self-host** ;
- sinon → **investigate**.

`ExtrapolateMonthly` ramène un coût observé sur `windowDays` à un mois de 30 jours.

## Intégration
Le controller [AIBreakEvenAnalysis](../crds/aibreakevenanalysis.md) calcule le coût du modèle
courant via [costengine](costengine.md), l'extrapole, applique `Analyze`, écrit les coûts/économie/
payback/recommandation et expose `ai_finops_breakeven_savings_eur`.
