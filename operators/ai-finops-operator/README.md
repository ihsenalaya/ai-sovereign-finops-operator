# AI FinOps Operator

Opérateur Kubernetes **autonome** de gouvernance FinOps & souveraineté du trafic IA :
attribution des coûts (EUR) par requête/modèle/équipe/namespace, budgets avec dégradation
gracieuse et fallback managé, contraintes de résidence des données, point mort
managé/auto-hébergé, quality gates par application, optimisation de routage et workflow
d'approbation humaine.

C'est l'un des **3 opérateurs indépendants** issus de la décomposition de l'opérateur
`ai-sovereign-finops-operator` (voir [`../README.md`](../README.md)). Il s'installe et
fonctionne **seul** — sans l'opérateur confidential ni l'opérateur GOV-AR.

## Contenu du dossier

```
ai-finops-operator/
├── README.md                  ← ce fichier
├── chart/ai-finops-operator/  ← Helm chart indépendant (11 CRDs + RBAC scopé)
├── docs/                      ← une fiche par CRD
└── automatisation/
    ├── up.sh / down.sh        ← cluster kind complet en une commande
    ├── test-apps/             ← applications de test (catalogue, usage, policies)
    └── dashboards/            ← dashboard Grafana "AI FinOps Operator — Overview"
```

## CRDs possédées (11)

| CRD | shortName | Rôle | Doc |
|---|---|---|---|
| AIProvider | `aiprov` | Fournisseur IA : tarifs, zone de résidence, conformité | [docs/aiprovider.md](docs/aiprovider.md) |
| AIModel | — | Modèle catalogué lié à un provider | [docs/aimodel.md](docs/aimodel.md) |
| AIGateway | `aigw` | Gateway observée + mode de télémétrie | [docs/aigateway.md](docs/aigateway.md) |
| AIBudgetPolicy | `aibudget` | Budget + seuils + fallback managé | [docs/aibudgetpolicy.md](docs/aibudgetpolicy.md) |
| AISovereigntyPolicy | `aisov` | Résidence des données / données sensibles | [docs/aisovereigntypolicy.md](docs/aisovereigntypolicy.md) |
| AIBreakEvenAnalysis | `aibreakeven` | Point mort API managée vs auto-hébergement | [docs/aibreakevenanalysis.md](docs/aibreakevenanalysis.md) |
| AIFinOpsReport | `aireport` | Rapport consolidé (Markdown/JSON en ConfigMap) | [docs/aifinopsreport.md](docs/aifinopsreport.md) |
| AIQualityGate | `aiqgate` | Validation qualité avant changement de modèle | [docs/aiqualitygate.md](docs/aiqualitygate.md) |
| AIRoutingPolicy | `airpolicy` | Politique d'optimisation continue du routage | [docs/airoutingpolicy.md](docs/airoutingpolicy.md) |
| AIRouteOverride | `airoverride` | Reroute manuel immédiat (réversible) | [docs/airouteoverride.md](docs/airouteoverride.md) |
| AIChangeRequest | `aicrq` | Demande de changement gouvernée (approbation humaine) | [docs/aichangerequest.md](docs/aichangerequest.md) |

Le manager (`operateur/cmd/finops-manager`) enregistre exactement les 11 controllers de ces
CRDs — pas de webhook, pas d'écriture hors des chemins d'enforcement déclarés.

## Installation

### Prérequis

- Kubernetes ≥ 1.29, Helm ≥ 3.12.
- *Optionnel* : Prometheus Operator (ServiceMonitor), Grafana, une gateway IA
  (Envoy AI Gateway, LiteLLM…) comme plan de données pour l'enforcement réel.

### Depuis le registre d'images publié

```bash
helm install finops ./chart/ai-finops-operator \
  --namespace finops-system --create-namespace
```

L'image par défaut est `ghcr.io/ihsenalaya/ai-sovereign-finops-operator/finops-operator`
(tag = `appVersion` du chart). Valeurs utiles :

```bash
# Image locale (kind) :
--set image.repository=finops-operator --set image.tag=dev --set image.pullPolicy=Never
# Prometheus Operator :
--set metrics.serviceMonitor.enabled=true --set metrics.serviceMonitor.labels.release=monitoring
# HA :
--set replicaCount=2 --set leaderElection.enabled=true
```

### Vérification

```bash
kubectl -n finops-system get deploy
kubectl get crd | grep aiops.imperium.io   # 11 CRDs FinOps
```

## Démarrage rapide kind (tout-en-un)

```bash
cd automatisation
./up.sh          # kind + Prometheus/Grafana + opérateur + apps de test + dashboard
```

Le script :

1. crée le cluster kind `finops-operator` ;
2. construit l'image depuis `operateur/Dockerfile.finops-operator` et la charge dans kind ;
3. installe `kube-prometheus-stack` (Grafana admin/admin) ;
4. installe le chart avec ServiceMonitor activé ;
5. déploie les **applications de test** dans `finops-demo` : catalogue 2 providers
   (Mistral EU / OpenAI US) + 3 modèles, gateway en télémétrie `configmap` avec usage mesuré
   statique (3 applications : chatbot-rh, marketing-assistant, support-triage), budget,
   politique de souveraineté FR/EU (le trafic US déclenche des constats), break-even H100 et
   rapport consolidé ;
6. importe le dashboard Grafana.

Vérifier les résultats :

```bash
kubectl -n finops-demo get aiprov,aimodel,aigw,aibudget,aisov,aireport
kubectl -n finops-demo get aireport monthly-demo-report -o yaml
kubectl -n finops-demo get configmap monthly-demo-report-report -o jsonpath='{.data.report\.md}'
```

Grafana : `kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80`
→ http://localhost:3000 → dashboard **AI FinOps Operator — Overview**
(coûts, budgets, souveraineté, enforcement, scores de routage, radar qualité/coût,
shadow-AI, économies potentielles, quality gates).

Démontage : `./down.sh`.

## Utilisation

### Boucle FinOps minimale

1. Déclarer le catalogue : `AIProvider` (tarifs + zone) et `AIModel`.
2. Pointer la télémétrie : `AIGateway` (`prometheus`, `configmap`, `aigw` — jamais de repli
   `fake` silencieux : sans source réelle, condition `NoTelemetrySource` explicite).
3. Poser les politiques : `AIBudgetPolicy`, `AISovereigntyPolicy`.
4. Consommer : `AIFinOpsReport` (ConfigMap Markdown/JSON), métriques `ai_finops_*`, events.

### Enforcement

`enforcementMode` sur les politiques : `reportOnly` → `warn` → `enforce`. En mode `enforce`
avec une Envoy AI Gateway, l'opérateur **actue réellement** : reroute budget vers le fallback
managé conforme, blocage souveraineté par backend réservé, `AIRouteOverride` immédiat, et
`AIChangeRequest` pour le changement gouverné par un humain.

### Métriques

Famille `ai_finops_*` sur `:8080/metrics` : coûts (`ai_finops_cost_eur`,
`ai_finops_cost_by_zone_eur`), budgets (`ai_finops_budget_usage_percent`), souveraineté
(`ai_finops_sovereignty_findings`), enforcement (`ai_finops_enforcement_actions`),
optimisation (`ai_finops_potential_savings_eur`, `ai_finops_routing_score`), qualité
(`ai_finops_quality_score`, `ai_finops_quality_gate_passed`), shadow-AI
(`ai_finops_shadow_ai_egress`).

## Intégration avec les autres opérateurs (optionnelle)

- **ai-govar-operator** : lit le catalogue (`AIModel`/`AIProvider`/`AIRoutingPolicy`) et le
  workflow `AIChangeRequest` (action `authorize-gov-ar-route`). Ses webhooks fail-closed
  tamponnent l'identité des reviewers d'approbation. Sans lui, le workflow `reroute`
  fonctionne normalement.
- **ai-confidential-operator** : indépendant ; aucune dépendance croisée.

> Ne pas installer cet opérateur **et** le chart monolithe `ai-sovereign-finops-operator`
> sur le même cluster : les deux réconcilieraient les mêmes CRDs.
