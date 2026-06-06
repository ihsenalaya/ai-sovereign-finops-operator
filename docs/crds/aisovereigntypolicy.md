# CRD — AISovereigntyPolicy

`aiops.imperium.io/v1alpha1`, namespaced. Short name : `aisov`.

Définit les règles de résidence/souveraineté des données. En MVP : **`reportOnly`** — produit des
findings, ne bloque jamais le trafic. Ne constitue pas une attestation juridique.

## Spec
| Champ | Type | Description |
|-------|------|-------------|
| `dataResidency.allowedZones[]` | []string | Zones autorisées (ex. `FR`, `EU`). |
| `dataResidency.forbiddenZones[]` | []string | Zones interdites (ex. `US`, `CN`). |
| `sensitiveData.externalProvidersAllowed` | bool | Donnée sensible vers fournisseurs externes. |
| `sensitiveData.requireAnonymization` | bool | Anonymisation requise. |
| `audit.retainLogsDays` | int32 | Rétention attendue des logs d'audit. |
| `audit.immutableLogs` | bool | Logs inviolables attendus. |
| `enforcementMode` | enum `reportOnly\|warn\|enforce` (défaut `reportOnly`) | Réaction. MVP = `reportOnly`. |

## Status
`observedGeneration`, `findingsCount`, `conditions[]` (`Ready`).

## Comportement du controller
Collecte l'usage, déduit les fournisseurs employés, évalue via
[sovereigntyengine](../features/sovereigntyengine.md) :
- zone interdite ⇒ **critical** ;
- hors zones autorisées ⇒ **warning** (EU couvre les pays UE) ;
- fournisseur managé externe alors que `externalProvidersAllowed=false` ⇒ **warning** ;
- `requireAnonymization` ⇒ **info**.

Renseigne `findingsCount`, expose `ai_finops_sovereignty_findings_total{severity}`, émet un Event si
critiques. Les findings détaillés apparaissent dans [AIFinOpsReport](aifinopsreport.md).

## Exemple
[`..._aisovereigntypolicy.yaml`](../../config/samples/aiops_v1alpha1_aisovereigntypolicy.yaml).
