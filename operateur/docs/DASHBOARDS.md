# Dashboards — valeurs & exigences

Ce document explique **chaque panneau** du dashboard Grafana
[`dashboards/ai-finops-overview.json`](../dashboards/ai-finops-overview.json) : *ce qu'il montre*,
*la requête* et surtout *ce qui est requis pour qu'il affiche de la donnée* (le « no data » vient
presque toujours d'un prérequis manquant, pas d'un bug).

Toutes les valeurs proviennent de **trafic réel** — l'opérateur n'invente jamais de chiffres
(principe no-fake). Un panneau vide = la source réelle correspondante n'est pas (encore) branchée.

## Les deux plans de données

Le dashboard agrège **deux plans indépendants** :

| Plan | Source de la donnée | Voit… | Alimente les panneaux |
|------|---------------------|-------|-----------------------|
| **Gateway** | Envoy AI Gateway (tokens + durée mesurés) → collecteur `aigw` de l'opérateur | le trafic qui **passe par** la gateway | coût, tokens, requêtes, budget, zone, souveraineté (gateway), recommandations, latence observée |
| **eBPF / Shadow-AI** | Tetragon (egress par pod) → ConfigMap `shadow-egress` → opérateur | le trafic qui **contourne** la gateway | Shadow-AI egress |

## Exigences globales (pour voir *quelque chose*)

1. **Opérateur** déployé (chart `ai-sovereign-finops-operator`) — expose `/metrics:8080`.
2. **Prometheus** scrute `…-operator-metrics:8080` (job `ai-finops-operator`).
3. **Grafana** avec la datasource Prometheus + le dashboard chargé (ConfigMap `demo-grafana-dashboard`).

Stack de démo prête à l'emploi : [`automatisation/demo/observability.yaml`](../../automatisation/demo/observability.yaml).

## Matrice panneau → prérequis

| # | Panneau | Type | Valeur affichée | **Requis pour avoir de la donnée** |
|---|---------|------|-----------------|-------------------------------------|
| 1 | Total cost (EUR) — observed | stat | `sum(ai_finops_cost_eur)` — dépense réelle cumulée | Gateway + apps + catalogue (prix) |
| 2 | Tokens (total in + out) | stat | tokens mesurés | Gateway + apps |
| 3 | Requests (total) | stat | requêtes LLM routées | Gateway + apps |
| 4 | Budget usage (%) by policy | bargauge | prévision mensuelle ÷ budget, par AIBudgetPolicy | Gateway + apps + ≥1 `AIBudgetPolicy` |
| 5 | Cost (EUR) by namespace | timeseries | coût par namespace dans le temps | Gateway + apps |
| 6 | Tokens by namespace | timeseries | tokens in/out par namespace | Gateway + apps |
| 7 | Requests violating sovereignty (by app) | bargauge | requêtes vers provider non conforme | Gateway + apps + `AISovereigntyPolicy` |
| 8 | Spend by sovereignty zone (EUR) | piechart | part de dépense par zone (EU vs US) | Gateway + apps + providers (zone) |
| 9 | Enforcement actions (sovereignty) | table | décisions d'enforcement par workload | `AISovereigntyPolicy` (warn/enforce pour actuer) |
| 10 | **Shadow-AI egress details** | table | pods appelant un LLM **hors gateway**, par zone | **Tetragon + forwarder** (ConfigMap `shadow-egress`) + `AISovereigntyPolicy` |
| 11 | **Shadow-AI hotspots by workload** | bargauge | connexions shadow-AI par workload/zone | idem #10 |
| 12 | **Observed latency telemetry** | table | latence mesurée en ms si disponible, disponibilité de télémétrie | Gateway + apps + métrique réelle `gen_ai_server_request_duration_seconds` |

> **La zone par défaut marche sans CR** : grâce au [catalogue intégré](features/catalog.md), le coût
> et la zone d'un modèle connu (`gpt-4o`…) se résolvent même **sans** AIProvider/AIModel — les CRs
> ne font qu'affiner/override.

## Comment remplir chaque plan

### Plan gateway (coût / tokens / requêtes / budget / souveraineté)
Il faut un **vrai trafic** via l'Envoy AI Gateway. Démo réelle clé en main :
[`automatisation/envoy-aigw/`](../../automatisation/envoy-aigw/README.md) (`deploy.sh up`) — installe
Envoy Gateway + Envoy AI Gateway, le catalogue, et des **apps consommatrices** qui font de vrais
appels OpenAI (US) + Mistral (EU). L'opérateur lit les tokens mesurés et, quand elle existe, la
durée réelle via le collecteur `aigw`. Si la durée n'est pas observée, le dashboard affiche
`latency_telemetry_available=0`, sans latence mesurée inventée.

### Plan eBPF (Shadow-AI egress)
Il faut **Tetragon** + le forwarder : [`automatisation/tetragon/`](../../automatisation/tetragon/README.md).
```bash
automatisation/tetragon/install.sh                 # Tetragon (DaemonSet eBPF) + TracingPolicy
kubectl apply -f automatisation/tetragon/rogue-app.yaml   # (démo) pod qui appelle OpenAI en direct
NS=default automatisation/tetragon/forwarder.sh    # eBPF -> ConfigMap shadow-egress
```
L'opérateur classe l'egress par zone à la réconciliation de l'`AISovereigntyPolicy` et émet
`ai_finops_shadow_ai_egress`.

## Lire les valeurs
- **Coût** = tokens mesurés × tarifs (AIProvider, ou défauts du catalogue). EUR.
- **Budget %** : vert < 70, jaune ≥ 70, orange ≥ 90, **rouge ≥ 100 (dépassé)** — prévision mensuelle.
- **Spend by zone** : la part **US** = exposition souveraineté ; **EU** = conforme.
- **Severity** (souveraineté & shadow) : `critical` = zone interdite, `warning` = hors zones autorisées.
- **Shadow-AI egress** : nombre de **connexions** (pas un %) vers un endpoint LLM hors gateway.
- **Enforcement `actuated`** : `false` = recommandé/observé, `true` = réellement actué dans la gateway.
- **Observed latency telemetry** : `ai_finops_measured_latency_millis` apparaît seulement si
  une latence réelle a été scrapée, sinon `latency_telemetry_available=0`.

## Accès rapide
```bash
# Prometheus libre :
kubectl -n greenops-system port-forward svc/demo-prometheus 9090:9090
# Grafana :
kubectl -n greenops-system port-forward svc/demo-grafana 3000:3000   # ou type LoadBalancer
# Métriques brutes de l'opérateur :
kubectl -n greenops-system port-forward svc/<release>-ai-sovereign-finops-operator-metrics 8080:8080
curl -s localhost:8080/metrics | grep ai_finops_
```

Détail des métriques : [`features/metrics.md`](features/metrics.md). Détail des panneaux
gateway (PromQL + lecture) : [`automatisation/envoy-aigw/README.md`](../../automatisation/envoy-aigw/README.md).
