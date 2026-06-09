# Fonctionnalité — Collecte de télémétrie (`internal/collectors`)

Abstraction de la source d'usage LLM, découplant les moteurs de toute gateway.

## Interface
```go
type UsageSample struct {
    Namespace, Application, Team, Provider, Model string
    Requests, InputTokens, OutputTokens, Errors  int64
    LatencyMillis                                 float64
}
type TelemetryCollector interface {
    Collect(ctx context.Context, window time.Duration) ([]UsageSample, error)
    Name() string
}
```

## Implémentations
| Impl | Package | État | Description |
|------|---------|------|-------------|
| `aigw` | `collectors/aigw` | ✔ | **Chemin réel de production.** Scrute l'endpoint OpenTelemetry/Prometheus d'**Envoy AI Gateway** et lit l'histogramme `gen_ai_client_token_usage` (tokens mesurés sur les réponses fournisseur). Attribue chaque modèle au workload via les headers `x-greenops-*` puis, à défaut, le catalogue `AIModel` (`serves*`). Testé (`aigw_test.go`). |
| `prometheus` | `collectors/prometheus` | ✔ | Scrute un endpoint text-exposition, parse des compteurs `ai_finops_*` labellisés `namespace/application/team/provider/model`. `Parse(io.Reader)` testable hors HTTP. |
| `configmap` | `collectors/configmap` | ✔ | Lit des échantillons d'usage réels depuis une `ConfigMap` (clé `usage.json`) dans le namespace de la gateway. |
| `fake` | `collectors/fake` | opt-in | Jeu de données déterministe pour démo/tests **uniquement sur `mode: fake` explicite** — jamais un repli silencieux. |

> Le mode télémétrie `litellm` (collecteur stub `ErrNotImplemented`) a été **retiré** : annoncer une
> non-feature induisait en erreur. Le `Type` de gateway `litellm` (techno réelle) reste valide.

## Sélection — pas de repli `fake` silencieux
Le controller choisit le collector via `AIGateway.spec.telemetry.mode` (`collectorFor`). **Il n'y a aucun
repli `fake` par défaut** : un produit dont la valeur est « des chiffres réels et vérifiables » ne doit
jamais servir des données fabriquées qu'on pourrait prendre pour de la vraie dépense. Sans source réelle
(pas de gateway, mode inconnu, `configmap` sans `sourceConfigMap`), `collectorFor` renvoie une **erreur**
et le controller pose une condition de status explicite (`NoTelemetrySource`). Le collector `fake` n'est
retourné que sur `mode: fake` explicite (démo).

## Évolutions
Durcir le chemin Envoy/OTel (latence, erreurs, détection de reset des compteurs gateway).
Lié : [AIGateway](../crds/aigateway.md), [costengine](costengine.md).
