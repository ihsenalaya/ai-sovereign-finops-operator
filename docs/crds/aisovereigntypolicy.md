# CRD — AISovereigntyPolicy

`aiops.imperium.io/v1alpha1`, namespaced. Short name : `aisov`.

Définit les règles de résidence/souveraineté des données et **pilote l'enforcement** via
`enforcementMode`. Ne constitue pas une attestation juridique.

## Spec
| Champ | Type | Description |
|-------|------|-------------|
| `dataResidency.allowedZones[]` | []string | Zones autorisées (ex. `FR`, `EU`). |
| `dataResidency.forbiddenZones[]` | []string | Zones interdites (ex. `US`, `CN`). |
| `sensitiveData.externalProvidersAllowed` | bool | Donnée sensible vers fournisseurs externes. |
| `sensitiveData.requireAnonymization` | bool | Anonymisation requise. |
| `audit.retainLogsDays` | int32 | Rétention attendue des logs d'audit. |
| `audit.immutableLogs` | bool | Logs inviolables attendus. |
| `enforcementMode` | enum `reportOnly\|warn\|enforce` (défaut `reportOnly`) | Réaction aux violations (voir ci-dessous). |

## Modes d'enforcement
Sur chaque réconciliation, les findings **critiques** (zone interdite) sont transformés en décisions par
[`enforcementengine`](../features/enforcementengine.md) :

| Mode | Action | Effet |
|------|--------|-------|
| `reportOnly` | `report` | Constat seul, aucune action (défaut). |
| `warn` | `warn` | **Alerte différenciée** : Event Kubernetes `Warning`/`Enforcement` + série `ai_finops_enforcement_actions{action="warn",actuated="true"}`. Ne bloque pas. |
| `enforce` | `reroute` ou `block` | **Reroute réellement actué** dans Envoy AI Gateway : la route du modèle interdit bascule vers le backend conforme (+ réécriture du `model`), `actuated="true"`, réversible (annotation + finalizer). Sans fallback conforme : `block` (décidé, actuation à venir). |

## Status
`observedGeneration`, `findingsCount`, `conditions[]` (`Ready`, dont le message résume la posture
d'enforcement : `… enforcement: N warn, N reroute, N block`).

## Comportement du controller
Collecte l'usage, déduit les fournisseurs employés, évalue via
[sovereigntyengine](../features/sovereigntyengine.md) :
- zone interdite ⇒ **critical** ;
- hors zones autorisées ⇒ **warning** (EU couvre les pays UE) ;
- fournisseur managé externe alors que `externalProvidersAllowed=false` ⇒ **warning** ;
- `requireAnonymization` ⇒ **info**.

Renseigne `findingsCount`, expose `ai_finops_sovereignty_findings{severity}` et, selon le mode,
`ai_finops_enforcement_actions` + des Events d'enforcement. Un **finalizer** purge les séries
d'enforcement de la policy à sa suppression. Les findings détaillés apparaissent dans
[AIFinOpsReport](aifinopsreport.md).

## Exemple
[`..._aisovereigntypolicy.yaml`](../../config/samples/aiops_v1alpha1_aisovereigntypolicy.yaml).
