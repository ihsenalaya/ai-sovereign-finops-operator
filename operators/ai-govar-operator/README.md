# AI GOV-AR Operator

Opérateur Kubernetes **autonome** d'**admission gouvernée** de l'inférence IA (GOV-AR) :
identité par workload (`AIWorkloadBinding`), service d'admission temps réel branché sur
Envoy `ext_proc`, **ledger durable de réservation/liability sur PostgreSQL**, évidence de
calibration, et chaîne d'approbation de changement **fail-closed** (webhooks
`AIChangeRequest`).

C'est l'un des **3 opérateurs indépendants** issus de la décomposition de l'opérateur
`ai-sovereign-finops-operator` (voir [`../README.md`](../README.md)). Il s'installe et
fonctionne **seul** ; son chart embarque une copie des CRDs catalogue qu'il lit
(AIModel, AIProvider, AIRoutingPolicy, AIChangeRequest, AIBudgetPolicy) — Helm saute les
CRDs déjà installées, la coexistence avec l'opérateur FinOps est donc sans conflit.

## Contenu du dossier

```
ai-govar-operator/
├── README.md                     ← ce fichier
├── chart/ai-govar-operator/      ← Helm chart indépendant (manager + admission + calibration)
├── docs/                         ← fiches CRD + opérations + migration
└── automatisation/
    ├── up.sh / down.sh           ← cluster kind complet en une commande (mode dev in-memory ;
    │                               up.sh installe aussi l'opérateur FinOps, cf. plus bas)
    ├── test-apps/                ← catalogue+télémétrie (00), policies (01), binding (02),
    │                               smoke test (03), trafic d'admission (04), quality gate (05)
    └── dashboards/               ← dashboard Grafana "Governed Admission"
```

## Fonctionnement

GOV-AR gouverne **chaque requête d'inférence** en temps réel, avec une identité
prouvée et un ledger financier durable :

1. **Identité par workload** — un `AIWorkloadBinding` (nom = ServiceAccount, spec
   **immuable**) lie un ServiceAccount à un tenant, un budget, une politique de routage,
   une sensibilité et des zones autorisées. À l'exécution, le service d'admission
   résout le binding depuis le Pod **authentifié** (namespace + serviceAccountName via
   TokenReview) : ni une requête ni une annotation ne peuvent usurper un binding.
2. **Décision d'admission** — branché sur Envoy `ext_proc` (gRPC) ou en HTTP, le
   service évalue les **champs typés** du catalogue (`spec.govar` des AIModel/AIProvider,
   pricing versionné, politique de réservation/calibration/drift/risque de
   l'AIRoutingPolicy) et rend une décision fermée :
   `ADMIT` / `QUEUE` / `REJECT` / `ABSTAIN` / `REQUIRE_APPROVAL`.
3. **Ledger réservation → settlement** — chaque admission réserve un montant
   (liability) ; la fin de requête le règle (settlement) ; les transitions ne sont
   comptées **qu'après commit** de la transaction. Production : PostgreSQL avec rôles
   moindre-privilège ; dev : `devInMemory=true`, explicitement mono-replica.
4. **Worker durable** — un worker réconcilie les réservations ambiguës (claims par
   type de travail, backlog, heartbeat) pour qu'aucune liability ne reste ouverte
   silencieusement.
5. **Calibration & drift** — le `gov-ar-calibration-producer` (authentifié séparément)
   produit l'évidence de calibration autoritaire ; en cas de drift détecté, le profil
   passe en **mode conservateur**.
6. **Approbation fail-closed** — l'autorisation d'une route exacte passe par un
   `AIChangeRequest` (`authorize-gov-ar-route`) : le webhook tamponne l'identité
   vérifiée du reviewer, rejette l'auto-approbation, et l'approbation référence des
   UID/générations et un `scopeDigest` exacts avec expiration.

## Fonctionnalités

- **Admission temps réel** de l'inférence IA sur Envoy `ext_proc`, à décisions fermées
  (`ADMIT`/`QUEUE`/`REJECT`/`ABSTAIN`/`REQUIRE_APPROVAL`).
- **Identité workload non usurpable** : TokenReview + binding immuable nom=SA.
- **Ledger financier durable** : réservation/liability/settlement transactionnels sur
  PostgreSQL, mode in-memory réservé au dev et affiché comme tel.
- **Garde-fous fail-closed au rendu Helm** : sans digest immuable, `softwareSHA256`,
  secret d'identité ou PostgreSQL (hors dev explicite), le chart **refuse de rendre**.
- **Chaîne d'approbation à deux personnes** : demandeur ≠ reviewer, groupe IdP dédié,
  scope et expiration exacts.
- **Calibration + détection de drift** avec repli conservateur.
- **Observabilité sans fuite** : métriques `govar_*` sans aucun label
  tenant/workload/prompt ; traces OTLP (W3C) propagées via `ext_proc` ;
  `/healthz` / `/readyz` séparés.

## Composants

| Composant | Binaire / image | Rôle |
|---|---|---|
| **Manager** | `govar-operator` | Controller `AIWorkloadBinding` + webhooks fail-closed `AIChangeRequest` (tampon d'identité demandeur/reviewer, rejet de l'auto-approbation). |
| **Admission** | `gov-ar-admission` | Service d'admission (HTTP + Envoy `ext_proc` gRPC) : résout l'identité du Pod authentifié (TokenReview), décide admit/réserve/refuse à partir des **champs typés**, tient le ledger de réservation/liability. SA **lecture seule** dédiée. |
| **Calibration** | `gov-ar-calibration-producer` | Producteur d'évidence de calibration autoritaire, authentifié indépendamment (rôle PostgreSQL moindre privilège). |

## CRDs

| CRD | shortName | Rôle | Doc |
|---|---|---|---|
| **AIWorkloadBinding** (possédée) | `aiwb` | Identité GOV-AR d'un ServiceAccount → tenant/budget/routage/sensibilité/résidence. Nom = ServiceAccount, spec immuable. | [docs/aiworkloadbinding.md](docs/aiworkloadbinding.md) |
| AIModel / AIProvider / AIRoutingPolicy (lues) | — | Champs de sécurité typés `spec.govar` / pricing versionné / politique de réservation-calibration-drift-risque | [docs/govar-safety-fields.md](docs/govar-safety-fields.md) |
| AIChangeRequest (lue + webhooks) | `aicrq` | Autorisation de route exacte `authorize-gov-ar-route` | [docs/gov-ar-approval-migration.md](docs/gov-ar-approval-migration.md) |

Exploitation (PostgreSQL, réconciliation, métriques, traces) :
[docs/gov-ar-operations.md](docs/gov-ar-operations.md).

## Installation

### Prérequis

- Kubernetes ≥ 1.29, Helm ≥ 3.12.
- **Production** : PostgreSQL (ledger durable — le mode in-memory est explicitement
  dev-only et mono-replica), un collecteur OTLP/HTTP si tracing, Envoy avec `ext_proc`
  et mTLS sur les deux sauts.
- **kind/dev** : rien d'autre — `devInMemory=true` est le mode développement explicite.

### Installation du chart (production)

```bash
# 1. Secrets requis (fail-closed) :
kubectl -n govar-system create secret generic govar-identity-master \
  --from-literal=GOVAR_IDENTITY_MASTER_SECRET="$(openssl rand -hex 32)"
kubectl -n govar-system create secret generic govar-db \
  --from-literal=DATABASE_URL="postgres://govar:...@postgres:5432/govar?sslmode=require"

# 2. Chart :
helm install govar ./chart/ai-govar-operator \
  --namespace govar-system --create-namespace \
  --set govArAdmission.enabled=true \
  --set govArAdmission.postgres.enabled=true \
  --set govArAdmission.postgres.existingSecret=govar-db \
  --set govArAdmission.identity.masterExistingSecret=govar-identity-master \
  --set govArAdmission.softwareSHA256=<sha256 de l'artefact admission> \
  --set govArAdmission.tracing.enabled=true \
  --set govArAdmission.tracing.endpoint=http://otel-collector:4318
```

Les **garde-fous fail-closed du chart sont intentionnels** : sans digest d'image immuable,
`softwareSHA256`, secret d'identité ou PostgreSQL (hors mode dev explicite), le rendu échoue
plutôt que de déployer une configuration non prouvable.

Le rôle reviewer (`…-gov-ar-approval-reviewer`) est créé mais **jamais bindé par le chart** :
un administrateur le lie uniquement à des sujets du groupe
`aiops.imperium.io:gov-ar-approval-reviewers` (fourni par l'IdP).

### Vérification

```bash
kubectl -n govar-system get deploy
kubectl get mutatingwebhookconfigurations aiops-govar-change-approval
kubectl get crd aiworkloadbindings.aiops.imperium.io
```

## Démarrage rapide kind (tout-en-un)

```bash
cd automatisation
./up.sh          # kind + Prometheus/Grafana + manager + admission (dev in-memory) + tests
```

Le script :

1. crée le secret d'identité et **construit l'image d'admission depuis la source**
   (`Dockerfile.gov-ar-admission`, chargée dans kind ; l'image publiée est consommée par
   digest en production, mais `kind load` ne préserve pas les digests de registre) ;
2. installe l'admission en mode `devInMemory` (mono-replica, sans PostgreSQL) avec le
   ServiceMonitor dédié qui scrape les familles `govar_*` ;
3. **installe aussi l'opérateur FinOps** — requis pour que l'admission décide (voir
   [Intégration avec les autres opérateurs](#intégration-avec-les-autres-opérateurs)) ;
4. applique les apps de test (`govar-demo`) : catalogue `azure-openai` GOV-AR-faisable +
   télémétrie (`00`), `AIBudgetPolicy` + `AIRoutingPolicy` (`01`), ServiceAccount
   `payments-api` + `AIWorkloadBinding` conforme (`02`), smoke test `/healthz`+`/readyz`
   (`03`), **générateur de trafic d'admission** (`04`) et **quality gate** évalué par un
   vrai job (`05`) ; il horodate enfin `pricing.observedAt` (rejeté au-delà de 24 h).

```bash
kubectl -n govar-demo get aiwb
kubectl -n govar-demo logs job/govar-smoke-test
kubectl -n govar-demo logs job/govar-admission-traffic   # 24 ADMIT, 6 refus gouvernés
```

Grafana : `kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80`
→ dashboard **AI GOV-AR Operator — Governed Admission**. Le trafic de démo alimente
**8 des 12 panels** : décisions, ratio et raisons de refus, transitions du ledger,
débit/latences HTTP. Les 4 panels de worker durable (`Pending records`, `Last successful
pass age`, `Worker claims`, `Oldest pending`) restent vides — ces métriques n'existent
qu'avec PostgreSQL, jamais en `devInMemory`.

Démontage : `./down.sh`.

## Utilisation

### Lier un workload

```yaml
apiVersion: aiops.imperium.io/v1alpha1
kind: AIWorkloadBinding
metadata:
  name: payments-api            # DOIT égaler spec.serviceAccountName
  namespace: payments
spec:
  serviceAccountName: payments-api
  tenantID: tenant-payments
  team: payments
  application: payments-api
  budgetPolicyRef: payments-budget      # même namespace
  routingPolicyRef: payments-routing    # même namespace
  sensitivity: high
  allowedZones: ["eu", "francecentral"]
  requireGateway: true
```

GOV-AR résout le binding depuis le Pod **authentifié** (namespace + serviceAccountName via
TokenReview) — ni une requête ni une annotation ne peuvent choisir un binding. La spec est
**immuable** : tout changement = remplacement + nouvelle évidence.

### Autoriser une route (approbation fail-closed)

1. Créer un `AIChangeRequest` action `authorize-gov-ar-route` avec les UID/générations
   exacts, le hash du route-snapshot, `scopeDigest` et `validUntil`.
2. Un reviewer **différent du demandeur**, membre du groupe
   `aiops.imperium.io:gov-ar-approval-reviewers`, pose `spec.approval: Approved` — le
   webhook tamponne son identité vérifiée et rejette l'auto-approbation.
3. Attendre `status.phase: Approved` avec `approvedScopeDigest` et `expiresAt` exacts.

Migration depuis les anciennes API `AIAdmissionApproval*` :
[docs/gov-ar-approval-migration.md](docs/gov-ar-approval-migration.md).

### Observabilité

Métriques `govar_*` sur le `/metrics` du service d'admission — **aucun label** ne porte
d'identité tenant/workload/requête/prompt. Traces OTLP (W3C Trace Context) propagées via
`ext_proc`. `/healthz` (process) et `/readyz` (ledger transactionnel) séparés. Détails :
[docs/gov-ar-operations.md](docs/gov-ar-operations.md).

#### Scraper les métriques `govar_*`

Les familles `govar_*` sont exposées par le **service d'admission** (port `8084`, qui sert
aussi l'API), *pas* par le service `-metrics` du manager (port `8080`, métriques
controller-runtime). Le chart installe donc **deux ServiceMonitors** quand
`metrics.serviceMonitor.enabled=true` :

| ServiceMonitor | Service scrapé | Métriques |
|---|---|---|
| `<release>-ai-govar-operator` | `-metrics` (8080) | `controller_runtime_*`, `workqueue_*` du manager |
| `<release>-ai-govar-operator-gov-ar-admission` | `-gov-ar-admission` (8084, `/metrics`) | **toutes les `govar_*`** |

En **production**, `enforcement.networkPolicy.enabled=true` applique un ingress
fail-closed sur le pod d'admission (seule la gateway atteint le port `ext_proc`).
Prometheus ne peut alors pas scraper le port 8084 : il faut ouvrir explicitement le
namespace de supervision, sinon **le dashboard reste vide**.

```bash
--set govArAdmission.enforcement.networkPolicy.monitoringNamespaceSelector."kubernetes\.io/metadata\.name"=monitoring
```

L'API sur ce port authentifie chaque requête (TokenReview + HMAC) : autoriser le scrape
n'accorde aucun privilège d'admission.

#### Ce que montre la démo kind

Le Job `04-admission-traffic` émet du **vrai trafic d'admission gouverné** : il
s'authentifie avec le token projeté du ServiceAccount (audience `gov-ar-admission`) et
envoie 30 requêtes `/v1/admit`, dont une sur cinq demande plus de tokens de sortie que le
cap attesté au catalogue. La démo produit ainsi **24 `ADMIT`** (`highest_utility_feasible`),
**6 refus gouvernés** (`ABSTAIN` / `strict_cap_unverified`) et **30 transitions de ledger**
`NEW → RESERVED`. Sont donc alimentés **8 des 12 panels** : décisions, ratio de refus,
raisons de refus, transitions du ledger, débit/latences HTTP.

Un refus gouverné est un **HTTP 200 portant la décision dans le corps**, pas une erreur de
transport — c'est ce que le générateur inspecte.

Les **4 panels restants** (`Pending records`, `Last successful pass age`, `Worker claims`,
`Oldest pending work item age`) mesurent le **worker durable**, qui n'existe qu'avec
PostgreSQL. En `devInMemory` ces métriques ne sont jamais émises : **8/12 est le plafond de
ce mode**, pas un défaut.

## Intégration avec les autres opérateurs

- **ai-finops-operator** : **requis à l'exécution**, pas optionnel. GOV-AR s'installe seul
  (chart, CRDs et RBAC disjoints), mais aucune décision d'admission n'aboutit sans les
  contrôleurs FinOps — voir ci-dessous.
- **ai-confidential-operator** : indépendant ; aucune dépendance croisée.

### Pourquoi FinOps est requis pour décider

Avant tout `ADMIT`, le service d'admission vérifie une chaîne d'évidences qu'il ne produit
pas lui-même (le manager GOV-AR n'enregistre que le contrôleur `AIWorkloadBinding`) :

| Exigence | Raison de refus si absente | Producteur |
|---|---|---|
| `AIBudgetPolicy` réconciliée `Ready`, génération à jour | `policy_not_ready` | **FinOps** |
| source de télémétrie réelle (le contrôleur budget refuse `Ready` sinon) | `policy_not_ready` | **FinOps** |
| `AIProvider` avec snapshot de prix normalisé | `pricing_incomplete` | **FinOps** |
| `AIModel` `Ready` avec évidence de cap de sortie | `model_not_ready` | **FinOps** |
| observation qualité fraîche (`status.lastEvaluatedAt`) | `quality_observation_stale` | **FinOps** (`AIQualityGate`) |

Deux contraintes tiennent à l'administrateur, pas à un opérateur :

- **Type de provider** — GOV-AR n'accepte que `openai` et `azure-openai` : ce sont les seuls
  adaptateurs fermés de prix et d'usage autoritaire. Un provider `mistral` ou `anthropic`
  est déclarable dans l'API Kubernetes mais **jamais routable** par GOV-AR.
- **Fraîcheur des prix** — `spec.pricing.observedAt` doit dater de moins de 24 h, sinon le
  snapshot est rejeté (`pricing_stale`). Un catalogue figé dans git périme donc en un jour ;
  `automatisation/up.sh` l'horodate à l'application.

Ces refus sont **corrects et fail-closed** : GOV-AR préfère s'abstenir plutôt que d'engager
de l'argent sur une évidence qu'il ne peut pas prouver. Mais sans FinOps, le service est
inopérant. L'automatisation kind installe donc les deux opérateurs.

> Ne pas installer cet opérateur **et** le chart monolithe `ai-sovereign-finops-operator`
> sur le même cluster : les webhooks `AIChangeRequest` seraient enregistrés deux fois.
