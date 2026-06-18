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
6. [Guides opérationnels](#guides-opérationnels)
   - [Ajouter un nouveau fournisseur](#ajouter-un-nouveau-fournisseur)
   - [Routage par fraction — canary 80/20](#routage-par-fraction--canary-8020)
   - [Reroute manuel immédiat](#reroute-manuel-immédiat)
   - [Workflow d'approbation humaine](#workflow-dapprobation-humaine)
   - [Budget avec reroute automatique](#budget-avec-reroute-automatique)
   - [Enforcement de souveraineté](#enforcement-de-souveraineté)
7. [Shadow AI — détection eBPF avec Tetragon](#shadow-ai--détection-ebpf-avec-tetragon)
8. [Calcul des scores](#calcul-des-scores)
9. [Métriques Prometheus](#métriques-prometheus)
10. [Dashboard Grafana](#dashboard-grafana)
11. [Troubleshooting](#troubleshooting)
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

## Guides opérationnels

### Ajouter un nouveau fournisseur

Ajouter un fournisseur se fait entièrement en YAML, **sans modifier ni redéployer l'opérateur**. Il faut créer 4 ressources dans l'ordre suivant :

#### Étape 1 — Secret avec la clé API

```bash
kubectl create secret generic mon-provider-apikey \
  --from-literal=apiKey="<votre-clé>" \
  -n default
```

#### Étape 2 — AIProvider (tarifs + souveraineté)

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIProvider
metadata:
  name: mon-provider
  namespace: default
spec:
  type: azure-openai           # openai | azure-openai | mistral | anthropic | bedrock | vertex
  region: francecentral
  dataResidency: fr            # utilisé par AISovereigntyPolicy pour valider la zone
  managed: true
  pricing:
    currency: EUR
    inputTokenPricePerMillion: "1.84"
    outputTokenPricePerMillion: "5.52"
  compliance:
    allowedForSensitiveData: true
    allowedCountries: [FR, EU]
```

#### Étape 3 — AIModel (catalogue)

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIModel
metadata:
  name: mon-modele
  namespace: default
spec:
  providerRef: mon-provider
  # Doit correspondre exactement au header x-ai-eg-model envoyé par les clients
  modelName: mon-modele-v1
  type: llm
  qualityTier: high
  costTier: medium
  sensitiveDataAllowed: true
  # Attribution du trafic gateway à une application
  servesNamespace: production
  servesApplication: mon-app
  servesTeam: data
```

#### Étape 4 — Ressources Envoy AI Gateway

Ces 4 objets câblent le routage réel dans l'Envoy AI Gateway :

```yaml
---
# Backend Envoy : endpoint HTTPS du fournisseur
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: Backend
metadata:
  name: mon-provider-backend
  namespace: default
spec:
  endpoints:
    - fqdn:
        hostname: mon-endpoint.openai.azure.com
        port: 443
  type: Endpoints
---
# TLS vers le backend
apiVersion: gateway.networking.k8s.io/v1alpha3
kind: BackendTLSPolicy
metadata:
  name: mon-provider-tls
  namespace: default
spec:
  targetRefs:
    - group: gateway.envoyproxy.io
      kind: Backend
      name: mon-provider-backend
  validation:
    wellKnownCACertificates: "System"
    hostname: mon-endpoint.openai.azure.com
---
# Backend IA (schéma de traduction + référence au Backend Envoy)
apiVersion: aigateway.envoyproxy.io/v1beta1
kind: AIServiceBackend
metadata:
  name: mon-provider-backend
  namespace: default
spec:
  schema:
    name: AzureOpenAI           # AzureOpenAI | OpenAI
    version: "2024-10-21"
  backendRef:
    name: mon-provider-backend
    kind: Backend
    group: gateway.envoyproxy.io
---
# Authentification par clé API
apiVersion: aigateway.envoyproxy.io/v1beta1
kind: BackendSecurityPolicy
metadata:
  name: mon-provider-apikey-policy
  namespace: default
spec:
  targetRefs:
    - group: aigateway.envoyproxy.io
      kind: AIServiceBackend
      name: mon-provider-backend
  type: APIKey
  apiKey:
    secretRef:
      name: mon-provider-apikey
      namespace: default
---
# Route : le header x-ai-eg-model détermine quel backend reçoit la requête
apiVersion: aigateway.envoyproxy.io/v1beta1
kind: AIGatewayRoute
metadata:
  name: mon-provider-route
  namespace: default
spec:
  parentRefs:
    - name: greenops-aigw
      kind: Gateway
      group: gateway.networking.k8s.io
  rules:
    - matches:
        - headers:
            - type: Exact
              name: x-ai-eg-model
              value: mon-modele-v1
      backendRefs:
        - name: mon-provider-backend
```

Après quelques minutes, le gateway route le trafic vers le nouveau fournisseur, l'opérateur lit les métriques `gen_ai_*` et les scores apparaissent dans Grafana — sans aucune modification de code.

---

### Routage par fraction — canary 80/20

Le routage par fraction permet d'envoyer un pourcentage du trafic vers un modèle candidat pendant qu'on valide sa qualité. L'Envoy AI Gateway supporte le poids (`weight`) dans les `backendRefs`.

#### Configuration gateway (20% vers le candidat)

```yaml
apiVersion: aigateway.envoyproxy.io/v1beta1
kind: AIGatewayRoute
metadata:
  name: content-writer-canary
  namespace: default
spec:
  parentRefs:
    - name: greenops-aigw
      kind: Gateway
      group: gateway.networking.k8s.io
  rules:
    - matches:
        - headers:
            - type: Exact
              name: x-ai-eg-model
              value: gpt-france-mini
      backendRefs:
        # 80% vers le modèle actuel
        - name: greenops-fr-openai
          weight: 80
        # 20% vers le modèle candidat
        - name: greenops-us-openai
          weight: 20
```

#### Surveiller le canary avec l'AIQualityGate

Pendant le canary, créer un `AIQualityGate` pour comparer les deux modèles :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIQualityGate
metadata:
  name: canary-gate
  namespace: default
spec:
  target:
    namespace: marketing
    application: content-writer
  sourceModel: gpt-france-mini       # modèle à 80%
  candidateModel: gpt-us-mini        # modèle à 20%
  goldenDatasetRef:
    name: content-writer-prompts
    namespace: marketing
  period: weekly
  requiredChecks:
    maxErrorRatePercent: 5
    maxLatencyIncreasePercent: 30
```

#### Promotion vers 100% (une fois la gate Passed)

```bash
# Vérifier que la quality gate est passée
kubectl get aiqgate canary-gate -o jsonpath='{.status.phase}'
# → Passed

# Basculer à 100% sur le candidat via un reroute manuel
kubectl apply -f - <<EOF
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRouteOverride
metadata:
  name: promote-us-mini
  namespace: default
spec:
  sourceModel: gpt-france-mini
  targetModel: gpt-us-mini
  reason: "Quality gate passed — promotion à 100%"
EOF
```

#### Rollback immédiat

```bash
# Supprimer le reroute revient automatiquement à la route originale
kubectl delete airoverride promote-us-mini
```

---

### Reroute manuel immédiat

Pour basculer tout le trafic d'un modèle vers un autre sans passer par un workflow d'approbation (incident, maintenance, test) :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIRouteOverride
metadata:
  name: incident-basculement
  namespace: default
spec:
  sourceModel: gpt-us-mini
  targetModel: gpt-france-mini
  reason: "Incident US — basculement EU pour conformité"
```

```bash
kubectl apply -f override.yaml

# Vérifier
kubectl get airoverride incident-basculement
# NAME                   SOURCE         TARGET           PHASE      AGE
# incident-basculement   gpt-us-mini    gpt-france-mini  Actuated   10s

# Revenir à la normale
kubectl delete airoverride incident-basculement
# → la route originale est automatiquement restaurée
```

---

### Workflow d'approbation humaine

L'opérateur peut proposer des changements de routage pour revue humaine avant application. Ce workflow garantit qu'aucune modification de route n'est appliquée sans validation explicite.

#### 1. L'opérateur crée automatiquement un AIChangeRequest

Quand l'`AIRoutingPolicy` ou l'`AIBudgetPolicy` détecte une meilleure option, elle génère un `AIChangeRequest` :

```bash
kubectl get aicrq -n default
# NAME                     ACTION    SOURCE         TARGET           APPROVAL   PHASE    RISK
# switch-to-fr-mini-x8z2   reroute   gpt-us-mini    gpt-france-mini  Pending    Pending  low
```

#### 2. Examiner la proposition

```bash
kubectl describe aicrq switch-to-fr-mini-x8z2
# Spec:
#   Reason:             Économie estimée 0.42 EUR/mois + zone EU conforme
#   Expected Saving:    0.42 EUR
#   Quality Score:      0.87
#   Latency Impact:     +40ms
#   Risk Level:         low
#   Expires After:      48h
#   Approval:           Pending
```

#### 3. Approuver ou rejeter

```bash
# Approuver — l'opérateur applique le changement dans l'Envoy AI Gateway
kubectl patch aicrq switch-to-fr-mini-x8z2 \
  --type=merge \
  -p '{"spec":{"approval":"Approved"}}'

# Rejeter
kubectl patch aicrq switch-to-fr-mini-x8z2 \
  --type=merge \
  -p '{"spec":{"approval":"Rejected"}}'
```

#### 4. Vérifier l'exécution

```bash
kubectl get aicrq switch-to-fr-mini-x8z2
# PHASE: Actuated

kubectl get events --field-selector reason=RouteActuated
```

Si personne n'approuve dans le délai `expiresAfter` (défaut 48h), la requête passe en phase `Expired` et est ignorée.

---

### Budget avec reroute automatique

Par défaut, un `AIBudgetPolicy` en mode `reportOnly` alerte mais ne bloque pas. Pour activer le reroute automatique quand le budget est dépassé :

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIBudgetPolicy
metadata:
  name: finance-budget-strict
  namespace: default
spec:
  target:
    namespace: finance
    application: risk-assistant

  period: monthly
  budgetEUR: "5.00"

  warningThresholdPercent: 70
  criticalThresholdPercent: 90
  hardLimitPercent: 100

  # Modèle de repli moins cher
  fallbackModelRef: gpt-us-mini

  # enforce = l'opérateur modifie l'AIGatewayRoute au lieu de juste alerter
  enforcementMode: enforce

  # Phase à partir de laquelle le repli est activé
  # Critical = dès 90% du budget | Exceeded = dès 100%
  fallbackOnPhase: Critical

  # Guardrails : le repli ne s'active que si le modèle candidat est acceptable
  maxFallbackLatencyMillis: 3000   # rejeté si latence > 3s
  maxFallbackErrorPercent: 5       # rejeté si taux d'erreur > 5%
  minFallbackQualityTier: medium   # rejeté si qualité < medium
```

**Cycle de vie** :

```
WithinBudget (0-70%)
  → Warning (70-90%)   : alerte + recommendations
  → Critical (90-100%) : reroute vers fallbackModelRef si guardrails OK
  → Exceeded (>100%)   : blocage ou escalade
  → retour automatique à WithinBudget au début de la prochaine période
```

Vérifier l'état :

```bash
kubectl get aibudget finance-budget-strict
# BUDGET(EUR)   USAGE%   PHASE
# 5             87       Critical

kubectl get aibudget finance-budget-strict -o jsonpath='{.status.fallbackActuated}'
# true

kubectl get aibudget finance-budget-strict -o jsonpath='{.status.fallbackReason}'
# "fallback actuated: gpt-us-mini latency=820ms errors=0% quality=high — all guardrails passed"
```

---

### Enforcement de souveraineté

#### Mode reportOnly (défaut) — journaliser sans bloquer

```yaml
spec:
  enforcementMode: reportOnly
```

Toutes les violations sont enregistrées dans `status.sovereigntyFindings` et dans les métriques Prometheus (`ai_finops_sovereignty_findings`). Aucun trafic n'est bloqué.

#### Mode warn — alertes différenciées

```yaml
spec:
  enforcementMode: warn
```

En plus des findings, l'opérateur lève des `Kubernetes Events` de type `Warning` sur la `AISovereigntyPolicy`, et la métrique `ai_finops_enforcement_actions` est incrémentée. Utile pour déclencher des alertes PagerDuty/Slack sans bloquer.

#### Mode enforce — blocage actif

```yaml
spec:
  enforcementMode: enforce
```

L'opérateur modifie l'`AIGatewayRoute` pour supprimer ou rerouter les backends non-conformes. Un modèle dont le fournisseur est dans une zone interdite reçoit un score de 0 et son route est bloquée au niveau du gateway.

> **Prérequis `enforce`** : l'opérateur doit avoir des droits d'écriture sur les ressources `AIGatewayRoute` dans les namespaces concernés. Vérifier le RBAC du chart Helm.

#### Tester la politique

```bash
# Voir les violations actuelles
kubectl get aisov eu-sovereignty -o jsonpath='{.status.findingsCount}'

# Détail des violations dans le rapport
kubectl get aireport ai-report-all -o jsonpath='{.status.sovereigntyFindings}' | python3 -m json.tool

# Métriques
curl -s http://localhost:9090/api/v1/query?query=ai_finops_sovereignty_findings | jq .
```

---

## Shadow AI — détection eBPF avec Tetragon

### Problème

Les collecteurs basés sur l'Envoy AI Gateway n'observent que le trafic qui **passe par le gateway**. Un pod qui appelle `api.openai.com` directement contourne tout — c'est le **Shadow AI** : le principal angle mort de souveraineté. Il est invisible pour les métriques OTel, les budgets, et les policies.

### Solution : Tetragon (eBPF)

[Tetragon](https://tetragon.io/) (CNCF, Apache-2.0) observe les connexions TCP sortantes **dans le noyau Linux** via eBPF, indépendamment de tout gateway ou SDK. Il détecte l'appel dès l'établissement de la connexion TLS (port 443), avant même l'envoi du premier octet de payload.

```
pod ──TLS──▶ api.openai.com (US)    ← jamais dans le gateway
  │
  └─ Tetragon (eBPF / kprobe tcp_connect)
       │
       ▼
  forwarder.sh ──▶ ConfigMap shadow-egress
       │
       ▼
  Opérateur (shadowengine)
       │
       ├─▶ ai_finops_shadow_ai_egress{namespace, application, zone, severity}
       └─▶ Kubernetes Events (Warning/ShadowAI) sur l'AISovereigntyPolicy
```

Les données sont **réelles** : connexions eBPF observées, jamais fabriquées. Aucune coopération de l'application n'est requise.

### Pourquoi Tetragon et pas Hubble

Tetragon est un **DaemonSet autonome** — il fonctionne sur n'importe quel CNI sans recréer le cluster. Hubble exige Cilium comme CNI (remplacement disruptif). L'opérateur est agnostique du backend : tout ce qui remplit le ConfigMap `shadow-egress` est compatible.

### Exigences

- Noyau Linux avec BPF + BTF (`/sys/kernel/btf/vmlinux`)
- Fonctionne sur AKS, EKS, GKE et kind/WSL (avec fallback `process_exec` sur kind)

### Installation

```bash
# 1. Installer Tetragon + la TracingPolicy d'egress TCP port 443
./automatisation/tetragon/install.sh

# 2. (Démo) déployer un workload "rogue" qui appelle OpenAI US directement
kubectl apply -f automatisation/tetragon/rogue-app.yaml

# 3. Transférer les événements eBPF vers le ConfigMap que l'opérateur lit
NS=default ./automatisation/tetragon/forwarder.sh

# 4. Observer
kubectl get events --field-selector reason=ShadowAI
kubectl get metric ai_finops_shadow_ai_egress  # via Prometheus / Grafana
```

### TracingPolicy

La politique eBPF observe toutes les connexions TCP sortantes sur le port 443 :

```yaml
apiVersion: cilium.io/v1alpha1
kind: TracingPolicy
metadata:
  name: greenops-egress-connect
spec:
  kprobes:
    - call: "tcp_connect"
      syscall: false
      args:
        - index: 0
          type: "sock"
      selectors:
        - matchArgs:
            - index: 0
              operator: "DPort"
              values: ["443"]
            - index: 0
              operator: "Family"
              values: ["AF_INET"]
```

### Workload rogue (démo)

`automatisation/tetragon/rogue-app.yaml` déploie un pod `finance/rogue-script` qui appelle `api.openai.com` directement (sans clé API — la connexion TLS seule suffit à déclencher la détection). Avec une `AISovereigntyPolicy` autorisant `EU` et interdisant `US`, ce trafic est classifié **critical** et émet :

- Métrique : `ai_finops_shadow_ai_egress{namespace="finance", application="rogue-script", zone="US", severity="critical"}`
- Event Kubernetes : `Warning ShadowAI` sur la `AISovereigntyPolicy`
- Panneau Grafana : **Shadow-AI egress details** + **Shadow-AI hotspots by workload**

### Limitations honnêtes

| Limitation | Explication |
|-----------|-------------|
| Mapping IP→host | `forwarder.sh` résout les IPs des providers LLM connus. Un CDN partagé peut brouiller la résolution ; SNI est plus précis (TracingPolicy TLS-SNI disponible en upgrade). |
| Schéma d'événements | Les chemins `jq` suivent le schéma `process_kprobe` de Tetragon. Adapter à la version installée si nécessaire (`tetra getevents -o json` pour inspecter). |
| Fallback kind/WSL | Si `tcp_connect` n'est pas exporté, le forwarder utilise `process_exec` (URLs dans les arguments de `curl`). La démo reste fonctionnelle. |
| ECH | SNI est en clair aujourd'hui. Encrypted Client Hello réduira la visibilité SNI à terme ; l'observation IP/port reste stable. |

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
| Shadow-AI egress | Trafic IA non-gouverné détecté par eBPF |

---

## Troubleshooting

### L'opérateur est en CrashLoopBackOff

```bash
kubectl logs -n greenops-system deploy/greenops-ai-sovereign-finops-operator --previous
```

Causes fréquentes :
- **CRD manquant** (`no matches for kind AIChangeRequest`) : Helm ne met pas à jour les CRDs sur `upgrade`. Appliquer manuellement : `kubectl apply -f operateur/config/crd/bases/`
- **RBAC insuffisant** : vérifier que le ServiceAccount a accès aux nouveaux CRDs : `kubectl auth can-i list aichangerequests --as=system:serviceaccount:greenops-system:greenops-ai-sovereign-finops-operator`

### Mistral / nouveau modèle n'apparaît pas dans le radar Grafana

Le radar n'affiche que les modèles avec du trafic réel passant par l'Envoy AI Gateway. Vérifier :

```bash
# 1. Le modèle a-t-il du trafic dans le gateway ?
curl -s http://<metrics-svc>:1064/metrics | grep gen_ai_client_token_usage | grep <model-name>

# 2. L'AIModel a-t-il un servesNamespace/Application ?
kubectl get aimodel <nom> -o jsonpath='{.spec.servesNamespace} {.spec.servesApplication}'

# 3. Le score est-il calculé ?
curl -s http://localhost:8082/metrics | grep ai_finops_cost_score | grep <model-name>
```

Si le modèle n'a pas de trafic gateway, seuls `ai_finops_quality_score` et `ai_finops_sovereignty_score` sont émis (avec `application="catalog"`). Le radar affiche quand même le modèle avec les dimensions disponibles (les autres à 0).

### Les métriques Prometheus sont vides

```bash
# Vérifier que le ServiceMonitor scrape l'opérateur
kubectl get servicemonitor -n greenops-system
kubectl get endpoints -n greenops-system | grep operator

# Vérifier que Prometheus a la bonne target
curl http://localhost:9090/api/v1/targets | python3 -m json.tool | grep ai-finops
```

### Un AIRouteOverride est en phase Failed

```bash
kubectl describe airoverride <nom>
# Status.Message indique la raison

# Cause la plus fréquente : l'AIGatewayRoute n'existe pas pour le sourceModel
kubectl get aigatewayroute -n default | grep <source-model>
```

### La quality gate reste en phase Pending

La gate évalue le candidat sur la fenêtre `period` configurée. Si le candidat n'a aucun trafic observé sur la fenêtre, le verdict est `insufficient-data` :

```bash
kubectl get aiqgate <nom> -o jsonpath='{.status.verdict}'
# insufficient-data

# Vérifier les observations
kubectl get aiqgate <nom> -o jsonpath='{.status.candidateObservation}'
```

Envoyer du trafic vers le modèle candidat (canary 5-10% pendant la durée de la période) pour accumuler assez de données.

### Le budget n'active pas le fallback

```bash
kubectl get aibudget <nom> -o jsonpath='{.status.fallbackReason}'
```

Causes fréquentes :
- `enforcementMode: reportOnly` (défaut) — changer en `enforce`
- Le modèle de fallback ne passe pas les guardrails : latence trop haute, taux d'erreur trop élevé, ou qualityTier insuffisant
- La phase actuelle est inférieure à `fallbackOnPhase` (ex: Warning quand fallbackOnPhase=Critical)

### Helm upgrade ne met pas à jour les CRDs

C'est un comportement intentionnel de Helm (protection contre la perte de données). Toujours appliquer les CRDs manuellement après un upgrade :

```bash
kubectl apply -f operateur/config/crd/bases/
# ou, pour un CRD spécifique :
kubectl apply -f operateur/config/crd/bases/aiops.imperium.io_aichangerequests.yaml
```
| Latence observée | Latence moyenne par modèle |
