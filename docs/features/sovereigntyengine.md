# Fonctionnalité — Moteur de souveraineté (`internal/sovereigntyengine`)

Vérifie **chaque flux observé** (namespace/application → modèle → fournisseur) contre une policy de
souveraineté et émet des findings **attribués au flux** qui les déclenche. **Pur**, couverture ~80 %.
**Ne bloque jamais** (reportOnly) ; produit une traçabilité d'audit, pas une attestation juridique.

## API
```go
type Policy struct { AllowedZones, ForbiddenZones []string; ExternalProvidersAllowed, RequireAnonymization bool }

// Flow = un flux observé enrichi des attributs provider/modèle nécessaires à la vérification.
type Flow struct {
    Namespace, Application, Model, Provider, Zone string
    Managed, ProviderAllowsSensitive, ModelAllowsSensitive bool
    Requests int64
}

// Vérification par flux (recommandée) : findings attribués namespace/app/modèle/fournisseur,
// agrégés (requêtes sommées) par violation distincte.
func EvaluateFlows(policy Policy, flows []Flow) []Finding

// Vérification par catalogue de fournisseurs (historique, conservée).
type ProviderInfo struct { Name, Zone string; Managed, AllowedForSensitiveData bool }
func Evaluate(policy Policy, providers []ProviderInfo) []Finding

func CountBySeverity(findings []Finding) map[string]int
func NormalizeZone(z string) string
```

`Finding` porte `Severity, Message, Namespace, Application, Model, Provider, Zone, Requests`.

## Règles (par flux)
| Cas | Sévérité |
|-----|----------|
| Zone du fournisseur du flux ∈ `forbiddenZones` | **critical** |
| `allowedZones` non vide et zone du flux non couverte | **warning** |
| Flux vers fournisseur managé externe, `externalProvidersAllowed=false`, provider/modèle non habilité aux données sensibles | **warning** |
| `requireAnonymization=true` | **info** |

`NormalizeZone` mappe les formes libres (`france`→`FR`, `eastus`→`US`…). **`EU` couvre les pays
membres** (FR, DE, …). Findings triés par sévérité décroissante puis namespace/app/fournisseur/modèle.

## Intégration
Les controllers [AISovereigntyPolicy](../crds/aisovereigntypolicy.md) et
[AIFinOpsReport](../crds/aifinopsreport.md) construisent les `Flow` depuis la télémétrie + le catalogue
(`catalog.flows`), appellent `EvaluateFlows`, et renseignent `findingsCount`, les findings détaillés du
rapport, et la métrique `ai_finops_sovereignty_findings_total{namespace,application,policy,severity}`.
