# Fonctionnalité — AI Quality Score (`internal/qualityengine`)

Calcule un score continu de qualite `0..100` pour un couple application/modele,
a partir d'un golden dataset, d'evidence de reponses et de telemetrie reelle de
la gateway. Le moteur est pur : il ne depend pas de Kubernetes et n'appelle pas
le reseau. Les embeddings locaux et le juge LLM souverain sont injectes par des
ports testables.

## Formule

```
Q = wc*Correctness + wr*Reliability + wl*Latency + ws*Semantic + wj*Judged
wc + wr + wl + ws + wj = 1
```

Poids par defaut :

| Dimension | Poids | Source |
|-----------|------:|--------|
| Correctness | 0.40 | golden dataset + evidence de reponse |
| Reliability | 0.20 | telemetrie reelle gateway |
| Latency | 0.15 | p95/latence observee vs seuil declare |
| Semantic | 0.15 | similarite reference/candidat par port local |
| Judged | 0.10 | juge LLM souverain optionnel |

Les poids sont declarables dans `AIQualityGate.spec.weights`. Si le juge est
desactive, son poids effectif est remis a `0` puis les poids sont renormalises.
Un poids explicite a `0` supprime la dimension correspondante.

## Dimensions

| Dimension | Calcul |
|-----------|--------|
| Correctness | moyenne de l'exact-match normalise, de la validite JSON quand attendue, du F1 par champ pour sorties structurees, et du ROUGE-L quand une reference textuelle existe |
| Reliability | `100 * (1 - errorRate - timeoutRate - invalidJSONRate)` sur telemetrie reelle |
| Latency | score decroissant de p95/latence observee : `<= seuil/2 => 100`, `seuil => 50`, `>= 2*seuil => 0`, interpolation lineaire |
| Semantic | similarite `0..100` fournie par un backend local d'embeddings, jamais par un service externe implicite |
| Judged | score `0..100` fourni par un juge LLM en zone conforme, temperature `0`, seed consigne |

Toutes les sorties sont bornees dans `[0,100]`. Sans dataset suffisant ou sans
telemetrie reelle pour le candidat, le verdict reste `insufficient-data`; aucun
score n'est invente.

## Verdict

Le verdict compare le candidat au modele source :

```
candidate-safe     si Q_candidate >= Q_source - tolerancePoints
candidate-risk     si Q_candidate <  Q_source - tolerancePoints
insufficient-data  si N_golden < minSamples ou si la telemetrie reelle manque
```

`minSamples` vaut `1` par defaut et `tolerancePoints` vaut `3`. Le score est
relatif : un modele candidat n'est pas juge "bon" dans l'absolu, il est juge
non degradant pour l'application cible.

## Auditabilite

Le `.status` de `AIQualityGate` expose :

- `qualityScore` pour le candidat ;
- `scoreBreakdown` et `dimensions` pour les cinq axes ;
- `weightsUsed`, `samples`, `evidenceRef` et le nombre d'echantillons ;
- `sourceObservation` et `candidateObservation` issus de la telemetrie.

Le detail complet du run est serialise dans la ConfigMap `evidenceRef` :
references du golden dataset, sorties source/candidat, composantes, poids,
modele juge et seed quand le juge est active.

Quand `evidenceRef` est absent ou incomplet, le controller ne fabrique pas de
resultats. Il cree un `Job` Kubernetes `quality-eval-*` qui reutilise l'image de
l'operateur, monte le golden dataset et appelle `spec.evaluation.endpoint` avec
les headers `x-ai-eg-model`, `x-greenops-namespace` et `x-greenops-app`. Le Job
ecrit les sorties source/candidat dans son message de terminaison ; seul le
controller les valide puis les persiste dans `evidenceRef`.

## Integration Azure AI Foundry

L'evaluation doit passer par la gateway IA de production (`spec.evaluation.endpoint`)
et les deploiements
Azure AI Foundry disponibles dans l'abonnement connecte. Les credentials restent
hors manifests : Key Vault et `automatisation/azure/kv-to-k8s.sh` synchronisent
les secrets vers Kubernetes. Le controller ne code aucun nom de deploiement en
dur et n'ecrit jamais de cle dans les logs, le status ou le dashboard.

La souverainete prime sur le score : un modele dans une zone interdite par une
`AISovereigntyPolicy` n'est jamais appele pour l'evaluation ni choisi comme juge.
Si Foundry, la gateway ou la telemetrie sont indisponibles, le gate expose une
condition `NoTelemetrySource` et un verdict `insufficient-data`.

## Radar Grafana

La metrique Prometheus est :

```
ai_finops_quality_score{namespace,app,provider,model,dimension}
```

`dimension` vaut `correctness`, `reliability`, `latency`, `semantic`, `judged`
ou `overall`. Le dashboard utilise le plugin Business Charts / ECharts
(`volkovlabs-echarts-panel`) pour afficher un radar dont les axes sont
`Correctness`, `Reliability`, `Latency`, `Semantic`, `Cost`, `Overall`, avec un polygone
par fournisseur detecte. L'axe `Cost` utilise `ai_finops_cost_score * 100` :
plus la valeur est haute, plus le provider est economique sur le trafic observe.

Dans la demo Kind reelle, les providers non conformes (`US`/`GLOBAL`) restent
visibles dans les findings de souverainete mais ne sont pas appeles par les Jobs
QualityScore. Le radar a trois polygones conformes quand `openai-fr`,
`mistral-eu` et le provider optionnel `openai-foundry-eu` ont chacun produit une
serie `dimension="overall"` issue d'une evaluation gateway reelle.

Liens : [AIQualityGate](../crds/aiqualitygate.md),
[collectors](collectors.md), [metrics](metrics.md).
