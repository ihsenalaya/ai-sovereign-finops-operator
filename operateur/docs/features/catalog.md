# Fonctionnalité — Catalogue par défaut (`internal/catalog`)

Catalogue **intégré** des providers/modèles LLM connus, pour que l'opérateur produise des
signaux de **coût** et de **souveraineté** utiles **dès l'installation** — *avant* d'écrire le
moindre [AIProvider](../crds/aiprovider.md) / [AIModel](../crds/aimodel.md). **Pur** (aucune
dépendance Kubernetes), unit-testé (`defaults_test.go`).

> **Défauts, jamais autorité.** Un CR utilisateur **écrase toujours** les défauts ; les défauts
> ne font que **combler les trous**. Les prix sont des **tarifs de liste publics** (pages des
> providers), revus à `PriceDate = 2026-01`, convertis USD→EUR ×0,92. Ce sont des **valeurs de
> référence best-effort pour la prévision, pas une source de facturation** — cohérent avec la
> règle du projet : ne jamais présenter un chiffre fabriqué comme une dépense mesurée.

## API
```go
func Known(model string) bool
func PriceForModel(model string) (costengine.TokenPricing, bool) // prix EUR par défaut
func ZoneForModel(model string) string                          // zone souveraineté par défaut
func DefaultPriceBook() costengine.PriceBook                    // seed du price-book
func EndpointToZone(endpoint string) (zone, provider string)    // déduire zone+provider d'un host
```

## 1. Prix & zone par modèle connu
`defaultModels` couvre les modèles publics établis (prix omis pour les ids inconnus/récents →
ils retombent sur une **recommandation data-quality** plutôt que de porter un prix inventé) :

| Famille | Zone | Exemples de modèles |
|---------|------|---------------------|
| OpenAI | **US** | `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `gpt-3.5-turbo` |
| Anthropic | **US** | `claude-3-5-sonnet`, `claude-3-5-haiku`, `claude-3-opus`, … |
| Mistral | **EU** | `mistral-large(-latest)`, `mistral-small(-latest)` |
| Google (Gemini) | US / régional via endpoint | `gemini-1.5-pro`, `gemini-1.5-flash` |

La résolution **normalise** l'id : minuscules + retrait d'un suffixe daté/versionné, donc
`Mistral-Large-2407` ou `claude-3-5-sonnet-20241022` résolvent vers leur id de base.

## 2. `EndpointToZone` — zone + provider à partir du *host*
Classe un flux par **où il va**, même sans entrée de catalogue (et, plus tard, à partir du SNI
observé par eBPF). Retourne des codes de zone canoniques via `sovereigntyengine.NormalizeZone`.

- **APIs directes** → zone fixe : `api.openai.com→US/openai`, `api.anthropic.com→US/anthropic`,
  `api.mistral.ai→EU/mistral`, `api.cohere.*→US`, `api.groq.com→US`.
- **Cloud-hébergé** (provider connu, **zone dérivée de la région dans le host**) :
  `*.openai.azure.com` / `services.ai.azure.com` (Azure OpenAI), `*.amazonaws.com` (Bedrock),
  `*.aiplatform.googleapis.com` (Vertex). La région est lue dans les labels du host :
  `francecentral→FR`, `westeurope`/`eu-*`/`europe-*→EU`, `eastus`/`us-*→US`, `uksouth→GB`,
  `ap-*`/`asia→AP`, `cn-*`/`china→CN`. Host inconnu → `("","")` (traité comme un trou
  data-quality, pas une devinette).

## 3. Câblage (fallback) — `internal/controller/catalog.go`
- `priceBook()` **seed** avec `DefaultPriceBook()` puis **superpose** le pricing des CRs
  utilisateur (le CR gagne). → le coût se calcule out-of-the-box pour un modèle connu, **sans
  AIProvider**.
- `zoneForModel()` retombe sur `ZoneForModel(model)` quand aucun CR ne couvre le modèle.
- `flows()` retombe sur `ZoneForModel`/`ProviderForModel` quand il n'y a **pas d'AIProvider CR**
  → la **souveraineté est autonome** (findings out-of-the-box, télémétrie provider-less classée) ;
  un flux qu'on ne peut placer ni par provider ni par modèle est ignoré (pas de faux « zone inconnue »).

Prouvé par `internal/controller/catalog_defaults_test.go` : **catalogue vide → `gpt-4o` résout
prix + zone US ; un CR override les défauts** ; `TestFlowsAutonomousZone`.

## 4. Auto-stub des modèles inconnus — `internal/controller/modelstubs.go`
Quand la télémétrie montre un modèle **inconnu** du catalogue user **et** des défauts (donc non
costable), le reconciler AIFinOpsReport crée un **AIModel stub** labellisé
`aiops.imperium.io/auto-stub=true` (`providerRef: unknown`, non pricé) — le pendant visible de la
recommandation `data-quality`. Idempotent, best-effort. Listable :
`kubectl get aimodel -l aiops.imperium.io/auto-stub=true`.

Consommé par : [costengine](costengine.md) (price-book), [sovereigntyengine](sovereigntyengine.md)
(zone des flux), [recommendationengine](recommendationengine.md) (candidats conformes).
