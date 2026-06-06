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
| `fake` | `collectors/fake` | ✔ | Jeu de données déterministe (démo/tests), aucune gateway requise. Profil par défaut aligné sur `config/samples`. |
| `prometheus` | `collectors/prometheus` | ✔ | Scrute un endpoint au format text-exposition, parse les compteurs `ai_finops_*` labellisés `namespace/application/team/provider/model`. `Parse(io.Reader)` testable hors HTTP. |
| `litellm` | `collectors/litellm` | stub | Surface prête (endpoint + clé) ; intégration HTTP `/spend/logs` à venir (`ErrNotImplemented`). |

## Sélection
Le controller choisit le collector via `AIGateway.spec.telemetry.mode` (`collectorFor`). Sans
gateway ou en mode `fake`, le collector `fake` est utilisé → toujours démontrable.

## Évolutions
Collecteur **Envoy/OpenTelemetry** (CNCF) ; LiteLLM complet (auth, pagination).
Lié : [AIGateway](../crds/aigateway.md), [costengine](costengine.md).
