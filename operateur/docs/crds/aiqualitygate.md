# CRD — AIQualityGate

`aiops.imperium.io/v1alpha1`, namespaced. Short name : `aiqgate`.

Valide si un modèle candidat est suffisamment sûr pour une application donnée. La granularité prévue
est **un `AIQualityGate` par application**.

## Spec
| Champ | Type | Requis | Description |
|-------|------|:------:|-------------|
| `target.namespace` / `target.application` | string | ✔ | Application protégée par le gate. |
| `target.team` | string | | Filtre optionnel si le même nom d'application existe dans plusieurs équipes. |
| `sourceModel` | string | ✔ | Modèle actuel. |
| `candidateModel` | string | ✔ | Modèle candidat. |
| `goldenDatasetRef.name/namespace/key` | ConfigMap ref | ✔ | Prompts métier de référence (`prompts.yaml` par défaut). |
| `evidenceRef.name/namespace/key` | ConfigMap ref | | Résultats déterministes d'un run golden (`results.yaml` par défaut). Requis si des checks de réponse sont activés. |
| `gatewayRef` | string | | Gateway à utiliser pour la télémétrie. Par défaut, première `AIGateway` du namespace. |
| `period` | enum `daily\|weekly\|monthly` | | Fenêtre de télémétrie, défaut `monthly`. |
| `requiredChecks.schemaValid` | bool | | Vérifie via evidence que le format attendu est respecté. |
| `requiredChecks.noUnexpectedRefusal` | bool | | Vérifie via evidence qu'il n'y a pas de refus inattendu. |
| `requiredChecks.noSensitiveDataLeak` | bool | | Vérifie via evidence qu'il n'y a pas de fuite sensible. |
| `requiredChecks.requiredKeywords[]` | []string | | Vérifie via evidence que les termes attendus sont présents. |
| `requiredChecks.maxErrorRatePercent` | int32 | | Seuil d'erreur observée du modèle candidat. |
| `requiredChecks.maxLatencyIncreasePercent` | int32 | | Seuil de dégradation latence candidat vs source. |
| `requiredChecks.maxRetryIncreasePercent` | int32 | | Réservé à la télémétrie retries. Si configuré aujourd'hui, le gate reste pending. |
| `canary.enabled/percent/duration` | object | | Contrat de canary attendu avant reroute complet. |
| `rollback.enabled/onErrorRatePercent/onLatencyIncreasePercent` | object | | Contrat de rollback attendu après actuation. |

## Status
`observedGeneration`, `phase` (`Pending|Passed|Failed`), `verdict`
(`insufficient-data|candidate-safe|candidate-risk`), `checkedSamples`, `failedChecks`,
`failureMessages[]`, `canaryStatus`, `sourceObservation`, `candidateObservation`, `conditions[]`.

## Comportement du controller
Le controller :

- lit et valide le golden dataset ;
- lit l'evidence ConfigMap si des checks de réponse sont activés ;
- collecte la télémétrie via l'`AIGateway` ;
- compare source et candidat pour les seuils opérationnels disponibles ;
- expose `ai_finops_quality_gate_passed` et `ai_finops_quality_gate_failed_checks` ;
- écrit un status lisible par un humain.

Il ne lance pas encore lui-même les prompts golden ni le canary de trafic. Ces deux étapes sont le
prochain morceau d'actuation ; cette première version fournit le contrat Kubernetes et la décision
auditable basée sur evidence + télémétrie.

## Exemple
[`..._aiqualitygate.yaml`](../../config/samples/aiops_v1alpha1_aiqualitygate.yaml).

```bash
kubectl apply -f config/samples/aiops_v1alpha1_aiqualitygate.yaml
kubectl get aiqgate finance-risk-assistant-quality-gate -o yaml
```
