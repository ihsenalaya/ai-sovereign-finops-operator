# AI Sovereign FinOps Operator

Un opérateur Kubernetes qui observe le trafic vers les LLM, calcule les coûts réels, vérifie la conformité de souveraineté des données, et recommande ou applique automatiquement le meilleur fournisseur pour chaque application.

---

## Table des matières

1. [Vue d'ensemble](#vue-densemble)
2. [Architecture](#architecture)
3. [Exigences de l'environnement](#exigences-de-lenvironnement)
4. [Installation](#installation)
5. [CRDs — référence complète](#crds--référence-complète)
   - [AIProvider](#aiprovider)
   - [AIModel](#aimodel)
   - [AIGateway](#aigateway)
   - [AIFinOpsReport](#aifinopsreport)
   - [AIBudgetPolicy](#aibudgetpolicy)
   - [AISovereigntyPolicy](#aisovereigntypolicy)
   - [AIQualityGate](#aiqualitygate)
   - [AIBreakEvenAnalysis](#aibreakevenanalysis)
   - [AIRoutingPolicy](#airoutingpolicy)
   - [AIRouteOverride](#airouteoverride)
   - [AIChangeRequest](#aichangerequest)
6. [Calcul des scores](#calcul-des-scores)
7. [Métriques Prometheus](#métriques-prometheus)
8. [Dashboard Grafana](#dashboard-grafana)

---

## Vue d'ensemble

Les entreprises utilisent des LLM via plusieurs fournisseurs (Azure OpenAI, Mistral, Anthropic...) sans visibilité consolidée sur les coûts, la latence ou la conformité réglementaire (RGPD, AI Act). L'**AI Sovereign FinOps Operator** résout ce problème en :

- **Observant** le trafic réel via l'Envoy AI Gateway (métriques OpenTelemetry `gen_ai_*`)
- **Attribuant** chaque dépense à un namespace, une application et une équipe
- **Vérifiant** la souveraineté des données : zone de résidence, données sensibles, fournisseurs autorisés
- **Calculant** un score de routage multi-critères (coût, qualité, latence, fiabilité, souveraineté)
- **Recommandant** ou **appliquant** automatiquement le meilleur modèle, avec approbation humaine optionnelle

Toutes les décisions sont auditables dans Kubernetes (status des CRs, Events) et dans Grafana via des métriques Prometheus.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cluster Kubernetes                                                       │
│                                                                          │
│  Applications utilisatrices                                              │
│  (marketing/content-writer, finance/risk-assistant, legal/sovereignty-demo) │
│         │                                                                │
│         ▼  HTTP /v1/chat/completions                                     │
│  ┌─────────────────┐                                                     │
│  │  Envoy AI       │  ──── x-ai-eg-model header ──▶  gpt-france-mini    │
│  │  Gateway        │                                   gpt-us-mini       │
│  └────────┬────────┘                                   mistral-large-latest │
│           │ gen_ai_client_token_usage (OTel)                             │
│           ▼                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  AI Sovereign FinOps Operator                                        │ │
│  │                                                                      │ │
│  │  Collector aigw ──▶ CostEngine ──▶ RoutingScoreEngine               │ │
│  │                                         │                            │ │
│  │  AIProvider / AIModel (catalogue) ──────┘                            │ │
│  │  AISovereigntyPolicy ──────────────▶ SovereigntyEngine               │ │
│  │  AIBudgetPolicy ───────────────────▶ BudgetEngine                    │ │
│  │  AIQualityGate ────────────────────▶ QualityEngine                   │ │
│  │                                         │                            │ │
│  │  Status des CRDs ◀──── AIFinOpsReport ◀─┘                           │ │
│  │  Métriques Prometheus ◀─────────────────┘                            │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│           │                                                              │
│           ▼                                                              │
│  Grafana (dashboard radar, coûts, souveraineté, budgets)                │
└─────────────────────────────────────────────────────────────────────────┘
```

L'opérateur ne modifie jamais les flux applicatifs directement. Il observe, calcule, et — lorsque configuré en mode `enforce` — modifie les ressources `AIGatewayRoute` d'Envoy pour rerouter automatiquement le trafic.

---

## Exigences de l'environnement

| Composant | Version minimale | Rôle |
|-----------|-----------------|------|
| Kubernetes | 1.28 | Cluster cible |
| Helm | 3.12 | Déploiement de l'opérateur |
| Envoy AI Gateway | 0.6.x | Source de télémétrie (`gen_ai_*`) |
| Prometheus | 2.x | Collecte des métriques de l'opérateur |
| Grafana | 11.x | Visualisation (plugin `ae3e-plotly-panel` requis pour le radar) |

**Droits RBAC requis** : l'opérateur a besoin de lire/écrire les CRDs `aiops.imperium.io/*`, lire les `AIGatewayRoute` d'Envoy, et lire les `ConfigMap`/`Secret` dans son namespace.

---

## Installation

### 1. Prérequis — Envoy AI Gateway

L'opérateur lit les métriques de l'Envoy AI Gateway. Celui-ci doit déjà être déployé et exposer un service `<nom>-metrics` sur le port `1064`.

### 2. Ajouter le dépôt Helm

```bash
helm repo add ai-sovereign-finops https://ghcr.io/ihsenalaya/ai-sovereign-finops-operator/charts
helm repo update
```

### 3. Installer l'opérateur

```bash
helm install greenops ai-sovereign-finops/ai-sovereign-finops-operator \
  --namespace greenops-system \
  --create-namespace
```

### 4. Vérifier le déploiement

```bash
kubectl get pods -n greenops-system
# greenops-ai-sovereign-finops-operator-xxx   1/1   Running

kubectl get crd | grep aiops.imperium.io
# 11 CRDs installés
```

### 5. Mise à jour (Helm ne met pas à jour les CRDs automatiquement)

Lors d'un `helm upgrade`, les CRDs existants ne sont **pas** mis à jour par Helm. Appliquez-les manuellement :

```bash
kubectl apply -f https://raw.githubusercontent.com/.../config/crd/bases/
```

---

## CRDs — référence complète

### AIProvider

**Rôle** : déclare un fournisseur de modèles IA avec ses tarifs et ses attributs de souveraineté. C'est la source de vérité sur le prix et la zone de résidence des données.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIProvider
metadata:
  name: azure-openai-fr
  namespace: default
spec:
  # Type du fournisseur
  # Valeurs : openai | azure-openai | mistral | anthropic | bedrock | vertex | self-hosted | custom
  type: azure-openai

  # Région cloud du fournisseur (ex: francecentral, westeurope, eastus)
  region: francecentral

  # Zone de résidence des données — utilisée par le moteur de souveraineté
  # Valeurs libres, en majuscules dans la politique (FR, EU, US, CN...)
  dataResidency: fr

  # true = API managée, false = GPU auto-hébergé
  managed: true

  pricing:
    # Code ISO 4217
    currency: EUR
    # Prix pour 1 000 000 tokens d'entrée
    inputTokenPricePerMillion: "0.15"
    # Prix pour 1 000 000 tokens de sortie
    outputTokenPricePerMillion: "0.60"
    # Coût fixe mensuel optionnel (engagement, capacité réservée)
    fixedMonthlyCost: "0"

  compliance:
    # Ce fournisseur peut-il traiter des données sensibles ?
    allowedForSensitiveData: true
    # Codes pays/zones autorisés
    allowedCountries: [FR, EU]
```

**Status** : `conditions[type=Ready]` indique si l'objet a été reconcilié correctement.

---

### AIModel

**Rôle** : catalogue un modèle disponible via un fournisseur. Associe un nom de déploiement côté fournisseur à un niveau de qualité, un niveau de coût, et optionnellement à l'application qui l'utilise (pour l'attribution du trafic gateway).

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIModel
metadata:
  name: gpt-france-mini
  namespace: default
spec:
  # Référence vers l'AIProvider (même namespace)
  providerRef: azure-openai-fr

  # Identifiant exact côté fournisseur — doit correspondre au header x-ai-eg-model
  # envoyé à l'Envoy AI Gateway
  modelName: gpt-france-mini

  # Type de modèle
  # Valeurs : llm | embedding | reranker | vision | audio
  type: llm

  # Taille de la fenêtre de contexte en tokens (optionnel)
  contextWindow: 128000

  # Niveau de qualité catalogué — utilisé dans le score de qualité
  # Valeurs : low | medium | high
  qualityTier: high

  # Niveau de coût catalogué — informatif
  # Valeurs : low | medium | high
  costTier: low

  # Ce modèle peut-il traiter des données sensibles ?
  sensitiveDataAllowed: true

  # Attribution du trafic gateway :
  # L'Envoy AI Gateway émet des métriques avec les labels k8s_namespace et k8s_app.
  # Ces trois champs indiquent quelle application utilise ce modèle,
  # permettant d'attribuer les coûts et scores par namespace/application.
  servesNamespace: marketing
  servesApplication: content-writer
  servesTeam: marketing
```

**Status** : `resolvedProvider` contient le type du fournisseur une fois la référence résolue.

> **Important** : `modelName` doit correspondre exactement à la valeur du header `x-ai-eg-model` que l'application envoie à l'Envoy AI Gateway. C'est ce label qui apparaît dans `gen_ai_request_model` des métriques OTel.

---

### AIGateway

**Rôle** : pointe l'opérateur vers une instance d'Envoy AI Gateway pour collecter la télémétrie. Un seul AIGateway suffit pour gouverner l'ensemble du cluster.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIGateway
metadata:
  name: greenops-aigw
  namespace: default
spec:
  # Type de gateway
  # Valeurs : litellm | envoy | kong | gateway-api | custom
  type: envoy

  # URL du gateway (utilisée pour les requêtes de contrôle)
  endpoint: http://greenops-aigw.envoy-gateway-system.svc.cluster.local:80

  telemetry:
    # Mode de collecte de télémétrie
    # aigw      : lit directement les métriques gen_ai_* de l'Envoy AI Gateway
    # prometheus : scrape un endpoint Prometheus générique
    # configmap  : lit des samples JSON depuis un ConfigMap (utile pour les tests)
    # fake       : génère des données synthétiques (développement uniquement)
    mode: aigw
```

**Status** : `governedNamespaces` liste les namespaces couverts par le sélecteur.

---

### AIFinOpsReport

**Rôle** : déclenche la génération d'un rapport FinOps complet pour un namespace ou tout le cluster. L'opérateur remplit le `.status` avec les coûts observés, les findings de souveraineté, les recommandations, et les scores de routage par application/modèle.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIFinOpsReport
metadata:
  name: ai-report-all
  namespace: default
spec:
  target:
    # Namespace à couvrir (vide = tout le cluster)
    namespace: ""
    # Fenêtre d'observation
    # Valeurs : daily | weekly | monthly
    period: monthly

  # Optionnel : restreindre la collecte à un AIGateway spécifique
  gatewayRef: greenops-aigw
```

**Status généré** :

| Champ | Description |
|-------|-------------|
| `totalCostEUR` | Dépense totale observée sur la période |
| `projectedMonthlyCostEUR` | Projection sur un mois complet (run-rate) |
| `totalInputTokens` / `totalOutputTokens` | Volume de tokens consommés |
| `topModels` | Classement des modèles par coût décroissant |
| `sovereigntyFindings` | Violations de souveraineté détectées |
| `recommendations` | Recommandations d'optimisation (coût, souveraineté) |
| `routingScores` | Score multi-critères par tuple namespace/application/modèle |

---

### AIBudgetPolicy

**Rôle** : définit un plafond de dépense pour un namespace, une équipe ou une application. L'opérateur suit la dépense en temps réel et passe par les phases `WithinBudget → Warning → Critical → Exceeded`. En mode `enforce`, il peut déclencher un reroute vers un modèle moins cher.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIBudgetPolicy
metadata:
  name: marketing-budget
  namespace: default
spec:
  target:
    namespace: marketing
    team: marketing
    application: content-writer

  # Fenêtre budgétaire
  period: monthly

  # Plafond en EUR
  budgetEUR: "10.00"

  # Seuils d'alerte (en % du budget)
  warningThresholdPercent: 70   # → phase Warning
  criticalThresholdPercent: 90  # → phase Critical
  hardLimitPercent: 100          # → phase Exceeded

  actions:
    onWarning: [alert]
    onCritical: [alert]
    onHardLimit: [blockOrRequireApproval]

  # Modèle de repli moins cher (référence un AIModel)
  fallbackModelRef: gpt-us-mini

  # Mode d'application du repli
  # reportOnly : recommandation uniquement (défaut)
  # warn       : alerte différenciée
  # enforce    : reroute actif dans l'Envoy AI Gateway
  enforcementMode: reportOnly

  # Phase à partir de laquelle le repli est activé (si enforce)
  fallbackOnPhase: Exceeded

  # Guardrails sur le modèle de repli
  maxFallbackLatencyMillis: 2000
  maxFallbackErrorPercent: 5
  minFallbackQualityTier: medium
```

**Status** : `phase`, `usagePercent`, `currentSpendEUR`, `projectedMonthlySpendEUR`, `fallbackActuated`.

---

### AISovereigntyPolicy

**Rôle** : déclare les règles de souveraineté des données applicables au namespace. L'opérateur évalue chaque modèle observé contre cette politique : un modèle dont le fournisseur est dans une zone interdite reçoit un **score de souveraineté de 0** et son score global est forcé à 0 (il ne sera jamais recommandé).

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AISovereigntyPolicy
metadata:
  name: eu-sovereignty
  namespace: default
spec:
  dataResidency:
    # Zones autorisées (codes libres, comparés en majuscules avec AIProvider.dataResidency)
    allowedZones: [FR, EU]
    # Zones explicitement interdites
    forbiddenZones: [US, CN]

  sensitiveData:
    # Les fournisseurs externes managés peuvent-ils recevoir des données sensibles ?
    externalProvidersAllowed: false
    # Faut-il anonymiser les prompts avant envoi ?
    requireAnonymization: false

  audit:
    retainLogsDays: 90
    immutableLogs: true

  # Réaction aux violations
  # reportOnly : enregistre les findings sans bloquer
  # warn       : lève des alertes Kubernetes Events + métriques
  # enforce    : bloque ou rereoute le trafic non-conforme
  enforcementMode: reportOnly
```

**Status** : `findingsCount` — nombre de violations détectées au dernier cycle.

---

### AIQualityGate

**Rôle** : valide qu'un modèle candidat est sûr pour remplacer le modèle actuel d'une application. Combine des vérifications déterministes (golden dataset) et des observations opérationnelles (taux d'erreur, latence).

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: content-writer-gate
  namespace: default
spec:
  target:
    namespace: marketing
    application: content-writer

  # Modèle actuel (source)
  sourceModel: gpt-france-mini

  # Modèle candidat à valider
  candidateModel: gpt-us-mini

  # ConfigMap contenant le golden dataset (fichier prompts.yaml ou prompts.json)
  goldenDatasetRef:
    name: content-writer-prompts
    namespace: marketing

  # ConfigMap optionnel avec les résultats CI du golden run (requis pour schemaValid)
  evidenceRef:
    name: content-writer-evidence
    namespace: marketing

  period: monthly

  requiredChecks:
    # Le modèle candidat doit produire des réponses conformes au schéma attendu
    schemaValid: true
    # Le modèle ne doit pas refuser des prompts valides
    noUnexpectedRefusal: true
    # Pas de fuite de données sensibles
    noSensitiveDataLeak: true
    # Mots-clés obligatoires dans les réponses
    requiredKeywords: [conformité, RGPD]
    # Taux d'erreur HTTP max (%)
    maxErrorRatePercent: 5
    # Augmentation de latence max par rapport au modèle source (%)
    maxLatencyIncreasePercent: 30

  canary:
    enabled: true
    percent: 10
    duration: 30m

  rollback:
    enabled: true
    onErrorRatePercent: 10
    onLatencyIncreasePercent: 50
```

**Status** : `phase` (Pending/Passed/Failed), `verdict` (candidate-safe/candidate-risk/insufficient-data), `failureMessages`, `sourceObservation`, `candidateObservation`.

---

### AIBreakEvenAnalysis

**Rôle** : compare le coût réel d'une API managée avec le coût estimé d'un déploiement GPU auto-hébergé, et calcule la durée de retour sur investissement.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIBreakEvenAnalysis
metadata:
  name: finance-breakeven
  namespace: default
spec:
  target:
    namespace: finance
    application: risk-assistant

  # Modèle managé actuellement utilisé
  currentModelRef: gpt-france-mini

  # Option auto-hébergée à comparer
  alternativeSelfHosted:
    modelName: llama-3-70b
    runtime: vllm           # vllm | tgi | ollama | sglang
    gpuType: l40s
    gpuCount: 2
    monthlyGpuCostEUR: "2400"
    estimatedOpsCostEUR: "300"
    storageNetworkCostEUR: "50"
    migrationCostEUR: "5000"

  # Fenêtre d'extrapolation (jours)
  analysisWindowDays: 30
```

**Status** : `managedMonthlyCostEUR`, `selfHostedMonthlyCostEUR`, `monthlySavingsEUR`, `paybackMonths`, `recommendation` (keep-managed/investigate/self-host).

---

### AIRoutingPolicy

**Rôle** : évalue en continu tous les couples application/modèle observés et identifie le meilleur modèle disponible selon un objectif (coût, qualité, latence). Peut créer des `AIChangeRequest` pour approbation humaine avant tout reroute.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRoutingPolicy
metadata:
  name: cost-optimizer
  namespace: default
spec:
  # Dimension principale d'optimisation
  # Valeurs : cost | quality | latency
  objective: cost

  guardrails:
    # Score minimum du modèle candidat pour être sélectionné (0-1)
    minQualityScore: 0.70
    # Latence mesurée maximale acceptée (ms)
    maxLatencyMillis: 3000
    # Rejeter les modèles non-conformes à la souveraineté
    requireSovereigntyCompliance: true

  canary:
    enabled: true
    percent: 10
```

**Status** : `recommendations` liste les candidats évalués avec leur score, économies estimées, et raison de blocage éventuel.

---

### AIRouteOverride

**Rôle** : reroute manuellement le trafic d'un modèle vers un autre dans l'Envoy AI Gateway. La suppression de la ressource **reverte automatiquement** la route originale.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRouteOverride
metadata:
  name: force-fr-backend
  namespace: default
spec:
  # Modèle actuellement servi
  sourceModel: gpt-us-mini
  # Modèle vers lequel rerouter
  targetModel: gpt-france-mini
  reason: "Incident US — basculement vers FR pour conformité EU"
```

**Status** : `phase` (Pending/Actuated/Failed/Reverted), `actuatedRoutes` (routes Envoy modifiées).

---

### AIChangeRequest

**Rôle** : représente un changement de routage proposé qui nécessite une **approbation humaine** avant exécution. L'opérateur génère ces objets automatiquement (via AIRoutingPolicy ou AIBudgetPolicy) ; un opérateur humain approuve ou rejette.

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIChangeRequest
metadata:
  name: switch-to-fr-mini
  namespace: default
spec:
  action: reroute
  sourceModel: gpt-us-mini
  targetModel: gpt-france-mini
  reason: "Économie de 0.42 EUR/mois + conformité EU"
  expectedSavingEUR: "0.42"
  qualityScore: "0.87"
  latencyImpact: "+40ms"
  riskLevel: low
  expiresAfter: 48h

  # Décision humaine : Pending (défaut) | Approved | Rejected
  approval: Pending
```

**Workflow** :
```
Pending ──[humain approuve]──▶ Approved ──[opérateur actue]──▶ Actuated
Pending ──[humain rejette]───▶ Rejected
Pending ──[délai expiré]─────▶ Expired
```

**Status** : `phase`, `message`, `actuatedAt`, `expiresAt`, `actuatedRoutes`.

---

## Calcul des scores

### Score de routage global

Le score final est un agrégat pondéré de quatre composantes, toutes dans [0, 1] (plus haut = meilleur) :

```
Score = 0.40 × CostScore
      + 0.30 × QualityScore
      + 0.20 × LatencyScore
      + 0.10 × ReliabilityScore
```

> **Règle de souveraineté (hard gate)** : si le fournisseur du modèle est dans une zone interdite par l'`AISovereigntyPolicy` active, le score est forcé à **0**, indépendamment des autres composantes. Ce modèle ne peut jamais être recommandé.

Les poids par défaut peuvent être surchargés dans l'`AIRoutingPolicy`.

---

### CostScore — score de coût

Normalisation inverse sur l'ensemble des modèles observés simultanément. Le modèle le moins cher obtient 1, le plus cher obtient 0.

```
CostPerRequest = (inputTokens × prixInput + outputTokens × prixOutput) / nbRequêtes

CostScore = 1 − (CostPerRequest − min) / (max − min)
```

Si tous les modèles ont le même coût unitaire : CostScore = 1 pour tous.

---

### QualityScore — score de qualité

Dérivé du champ `qualityTier` de l'`AIModel` :

| `qualityTier` | QualityScore |
|---------------|-------------|
| `high`        | 1.00        |
| `medium`      | 0.75        |
| `low`         | 0.50        |
| (absent)      | 0.60        |

---

### LatencyScore — score de latence

Normalisation inverse sur la latence moyenne mesurée (en ms), lorsque la télémétrie est disponible :

```
LatencyScore = 1 − (latenceMoyenne − min) / (max − min)
```

Si la télémétrie de latence n'est pas disponible pour un modèle (ex: aucun header de timing dans les réponses), la composante latence prend la **valeur neutre** `0.5` et le champ `latencyTelemetryAvailable` est `false`. Le score n'est jamais fabriqué.

---

### ReliabilityScore — score de fiabilité

Dérivé du taux d'erreur HTTP observé sur la fenêtre :

```
ReliabilityScore = 1 − (erreurs / requêtes)
```

0 erreur → 1.0 | 100% d'erreurs → 0.0

---

### SovereigntyScore — score de souveraineté

Binaire, calculé à partir de la politique `AISovereigntyPolicy` active dans le namespace :

```
SovereigntyScore = 1  si AIProvider.dataResidency ∈ allowedZones
                 = 0  si AIProvider.dataResidency ∈ forbiddenZones
                        ou non présent dans allowedZones
```

Une zone vide dans `forbiddenZones` signifie "pas de zone explicitement interdite" — la logique s'appuie alors sur `allowedZones` seul.

---

## Métriques Prometheus

L'opérateur expose les métriques suivantes sur le port `8080` (chemin `/metrics`) :

### Scores par application/modèle

| Métrique | Labels | Description |
|----------|--------|-------------|
| `ai_finops_routing_score` | namespace, application, model | Score global [0,1] |
| `ai_finops_cost_score` | namespace, application, model | Composante coût [0,1] |
| `ai_finops_quality_score` | namespace, application, model | Composante qualité [0,1] |
| `ai_finops_latency_score` | namespace, application, model | Composante latence [0,1] |
| `ai_finops_reliability_score` | namespace, application, model | Composante fiabilité [0,1] |
| `ai_finops_sovereignty_score` | namespace, application, model | 1=conforme, 0=violation |

> Les modèles sans trafic réel (catalogue uniquement) portent le label `application="catalog"`. Les modèles avec trafic réel portent le namespace et l'application de l'app consommatrice.

### Coûts et volumes

| Métrique | Labels | Description |
|----------|--------|-------------|
| `ai_finops_cost_eur` | namespace, application, model | Coût EUR observé |
| `ai_finops_cost_by_zone_eur` | zone | Coût EUR agrégé par zone de résidence |
| `ai_finops_input_tokens` | namespace, application, model | Tokens d'entrée |
| `ai_finops_output_tokens` | namespace, application, model | Tokens de sortie |
| `ai_finops_requests` | namespace, application, model | Nombre de requêtes |
| `ai_finops_budget_usage_percent` | namespace, application | % du budget consommé |

### Souveraineté

| Métrique | Labels | Description |
|----------|--------|-------------|
| `ai_finops_sovereignty_findings` | namespace, application, severity | Violations détectées |
| `ai_finops_sovereignty_requests` | namespace, application, zone | Requêtes par zone |
| `ai_finops_shadow_ai_egress` | namespace | Trafic IA non-gouverné (eBPF) |

---

## Dashboard Grafana

Le dashboard **AI FinOps Overview** (`dashboards/ai-finops-overview.json`) requiert le plugin [`ae3e-plotly-panel`](https://grafana.com/grafana/plugins/ae3e-plotly-panel/) pour le panneau radar. Construire une image Grafana avec le plugin pré-installé :

```dockerfile
# Dockerfile.grafana
FROM grafana/grafana:11.2.2
RUN grafana-cli plugins install ae3e-plotly-panel
```

**Panneaux principaux** :

| Panneau | Description |
|---------|-------------|
| Routing Score par app | Score global de routage par application (time series) |
| Radar multi-dimensionnel | Comparaison visuelle des modèles sur les 5 axes (spider chart) |
| Coûts par namespace/zone | Répartition des dépenses par zone de résidence |
| Budget par application | % de consommation budgétaire avec seuils |
| Findings de souveraineté | Violations par namespace et sévérité |
| Quality Gate | Résultats des AIQualityGate (Passed/Failed/Pending) |
| Break-even analysis | Comparaison managé vs auto-hébergé |
| Latence observée | Latence moyenne par modèle |
