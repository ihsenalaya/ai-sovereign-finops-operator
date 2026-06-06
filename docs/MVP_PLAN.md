# Plan technique du MVP

Réponse au §11 de la prompt produit ([PRODUCT_PROMPT.txt](PRODUCT_PROMPT.txt)) : architecture, CRDs,
packages, interfaces, plan de fichiers, commandes Kubebuilder, backlog. Les hypothèses non
bloquantes sont marquées **(hyp.)** comme demandé au §9.

## 1. Architecture technique

```
                 ┌─────────────────────────────────────────────┐
                 │            Kubernetes API server             │
                 │  CRDs aiops.imperium.io/v1alpha1 (7 kinds)   │
                 └───────────────▲──────────────────┬──────────┘
                                 │ watch/status      │ apply
        ┌────────────────────────┴───────────┐      │
        │      Operator (controller-runtime)  │      │
        │  ┌───────────────────────────────┐  │   ┌──┴───────────┐
        │  │ 7 reconcilers (internal/...)  │  │   │   kubectl /   │
        │  │  - status + conditions        │  │   │   GitOps      │
        │  │  - events, structured logs    │  │   └──────────────┘
        │  └───────┬───────────────────────┘  │
        │          │ utilise                   │
        │  ┌───────┴───────┐ ┌──────────────┐ │
        │  │ costengine    │ │ budgetengine │ │
        │  │ sovereignty.. │ │ breakeven..  │ │      (Sprints 2-5)
        │  └───────┬───────┘ └──────┬───────┘ │
        │          │ lit            │          │
        │  ┌───────┴────────────────┴───────┐ │
        │  │ collectors.TelemetryCollector  │ │
        │  │  fake | prometheus | litellm   │ │
        │  └───────┬─────────────────────────┘ │
        │          │ metrics Prometheus (/metrics)
        └──────────┼──────────────────────────┘
                   ▼
        ┌──────────────────┐   scrape    ┌──────────────┐
        │   AI Gateway      │◀────────────│ Prometheus    │──▶ Grafana
        │ LiteLLM / Envoy.. │ (read-only) └──────────────┘   (Sprint 6)
        └──────────────────┘
```

Principes : **lecture seule**, non-bloquant (`reportOnly`), valeur d'abord par le rapport,
réconciliation idempotente, briques **CNCF/OSS** uniquement.

## 2. CRDs — champs exacts

Groupe `aiops.imperium.io`, version `v1alpha1`. Implémentés dans `api/v1alpha1/`. Valeurs monétaires
en `resource.Quantity` **(hyp.)** pour des décimaux exacts et idiomatiques k8s (pas de float dans le schéma).
Tous les Status portent `observedGeneration` + `conditions[]` (type `Ready`).

- **AIGateway** : `spec.type` (litellm|envoy|kong|gateway-api|custom), `endpoint`, `namespaceSelector`,
  `telemetry{mode(fake|prometheus|litellm), metricsEndpoint}`, `auth.secretRef`. Status : `governedNamespaces[]`.
- **AIProvider** : `type`, `region`, `dataResidency`, `managed`, `pricing{currency, inputTokenPricePerMillion,
  outputTokenPricePerMillion, fixedMonthlyCost?}`, `compliance{allowedForSensitiveData, allowedCountries[]}`.
- **AIModel** : `providerRef`, `modelName`, `type(llm|embedding|…)`, `contextWindow`, `qualityTier`,
  `costTier`, `sensitiveDataAllowed`. Status : `resolvedProvider`.
- **AIBudgetPolicy** : `target{namespace,team,application}`, `period`, `budgetEUR`, `*ThresholdPercent`,
  `hardLimitPercent`, `actions{onWarning,onCritical,onHardLimit}`, `fallbackModelRef`. Status : `currentSpendEUR,
  usagePercent, phase`.
- **AISovereigntyPolicy** : `dataResidency{allowed/forbiddenZones}`, `sensitiveData{externalProvidersAllowed,
  requireAnonymization}`, `audit{retainLogsDays, immutableLogs}`, `enforcementMode(reportOnly|warn|enforce)`.
  Status : `findingsCount`.
- **AIBreakEvenAnalysis** : `target`, `currentModelRef`, `alternativeSelfHosted{modelName, runtime(vllm|tgi|
  ollama|sglang), gpuType, gpuCount, monthlyGpuCostEUR, estimatedOpsCostEUR?, storageNetworkCostEUR?,
  migrationCostEUR?}`, `analysisWindowDays`. Status : coûts mensuels, `monthlySavingsEUR`, `paybackMonths`,
  `recommendation(keep-managed|investigate|self-host)`.
- **AIFinOpsReport** : `target{namespace,period}`, `gatewayRef?`. Status : totaux tokens/coûts, `topModels[]`,
  `sovereigntyFindings[]`, `recommendations[]`, `generatedAt`.

## 3. Packages Go

| Package | Sprint | Rôle |
|---------|--------|------|
| `api/v1alpha1` | 1 ✅ | types CRD, deepcopy, conditions communes |
| `internal/controller` | 1 ✅ | 7 reconcilers + helpers status |
| `internal/collectors/{fake,prometheus,litellm}` | 2 | `TelemetryCollector` |
| `internal/costengine` | 2 | coûts par modèle/équipe/namespace/requête/token |
| `internal/budgetengine` | 3 | conso vs budget, seuils, recommandations |
| `internal/sovereigntyengine` | 4 | findings de souveraineté |
| `internal/breakevenengine` | 5 | point mort managé vs auto-hébergé |
| `internal/reporting` | 6 | export Markdown/JSON |
| `internal/metrics` | 3+ | métriques Prometheus `ai_finops_*` |

## 4. Interfaces principales (cibles)

```go
// internal/collectors
type UsageSample struct {
    Namespace, Application, Team   string
    Provider, Model                string
    Requests                       int64
    InputTokens, OutputTokens      int64
    LatencyMillis                  float64
    Errors                         int64
}
type TelemetryCollector interface {
    Collect(ctx context.Context, window time.Duration) ([]UsageSample, error)
}

// internal/costengine
type CostEngine interface {
    Cost(samples []UsageSample, providers []v1alpha1.AIProvider) CostBreakdown
}

// internal/breakevenengine
type BreakEvenEngine interface {
    Analyze(spec v1alpha1.AIBreakEvenAnalysisSpec, managedMonthly Money) BreakEvenResult
}
```

## 5. Plan de fichiers

Conforme au §8 de la prompt — voir l'arborescence réelle via `tree -L 3 -I 'bin|vendor'`. Sprint 1
remplit `api/`, `cmd/`, `internal/controller/`, `config/`, `docs/`, `Makefile`. Les dossiers
`internal/{collectors,costengine,…}`, `charts/`, `dashboards/` arrivent aux sprints suivants.

## 6. Commandes Kubebuilder utilisées

```bash
kubebuilder init --domain imperium.io \
  --repo github.com/imperium/ai-sovereign-finops-operator \
  --project-name ai-sovereign-finops-operator

for K in AIGateway AIProvider AIModel AIBudgetPolicy \
         AISovereigntyPolicy AIBreakEvenAnalysis AIFinOpsReport; do
  kubebuilder create api --group aiops --version v1alpha1 --kind $K --resource --controller
done
```

> Note d'environnement : controller-gen v0.14 (épinglé par Kubebuilder 3.14) ne compile pas sous
> Go 1.25. Le `Makefile` a été bumpé : controller-gen **v0.18.0**, kustomize **v5.4.3**, envtest k8s **1.31.0**.

## 7. Backlog sprint par sprint

Voir [ROADMAP.md](ROADMAP.md).
