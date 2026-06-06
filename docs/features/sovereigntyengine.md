# Fonctionnalité — Moteur de souveraineté (`internal/sovereigntyengine`)

Évalue les fournisseurs utilisés contre une policy de souveraineté et émet des findings. **Pur**,
couverture ~80 %. **Ne bloque jamais** (reportOnly) ; produit une traçabilité d'audit, pas une
attestation juridique.

## API
```go
type Policy struct { AllowedZones, ForbiddenZones []string; ExternalProvidersAllowed, RequireAnonymization bool }
type ProviderInfo struct { Name, Zone string; Managed, AllowedForSensitiveData bool }
func Evaluate(policy Policy, providers []ProviderInfo) []Finding
func CountBySeverity(findings []Finding) map[string]int
func NormalizeZone(z string) string
```

## Règles
| Cas | Sévérité |
|-----|----------|
| Zone du fournisseur ∈ `forbiddenZones` | **critical** |
| `allowedZones` non vide et zone non couverte | **warning** |
| Fournisseur managé externe et `externalProvidersAllowed=false` | **warning** |
| `requireAnonymization=true` | **info** |

`NormalizeZone` mappe les formes libres (`france`→`FR`, `eastus`→`US`…). **`EU` couvre les pays
membres** (FR, DE, …). Findings triés par sévérité décroissante.

## Intégration
Le controller [AISovereigntyPolicy](../crds/aisovereigntypolicy.md) renseigne `findingsCount` et
`ai_finops_sovereignty_findings_total{severity}`. Les findings détaillés sont aussi inclus dans
[AIFinOpsReport](../crds/aifinopsreport.md).
