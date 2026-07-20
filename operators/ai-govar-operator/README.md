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
    ├── up.sh / down.sh           ← cluster kind complet en une commande (mode dev in-memory)
    ├── test-apps/                ← policies + binding + smoke test Job
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

Le script crée le secret d'identité, épingle le **digest** de l'image d'admission publiée,
installe l'admission en mode `devInMemory` (mono-replica, sans PostgreSQL), applique les
apps de test (`govar-demo`) : `AIBudgetPolicy` + `AIRoutingPolicy` + ServiceAccount
`payments-api` + `AIWorkloadBinding` conforme (nom = SA, zones normalisées), puis lance un
**Job de smoke test** qui vérifie `/healthz`, `/readyz` et la présence des métriques
`govar_*`.

```bash
kubectl -n govar-demo get aiwb
kubectl -n govar-demo logs job/govar-smoke-test
```

Grafana : `kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80`
→ dashboard **AI GOV-AR Operator — Governed Admission** (débit et ratio de refus des
décisions, latences p95/p99, transitions du ledger, backlog du worker durable, âge du
dernier heartbeat, claims par type de travail).

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

La démo n'émet **aucune requête d'admission** (le smoke test ne touche que `/healthz`,
`/readyz` et `/metrics`) et tourne en `devInMemory` (donc sans worker durable). Sont donc
alimentés les panels HTTP (débit par endpoint/classe de statut, latences p95/p99) ; les
panels décisions, ledger et worker restent vides tant qu'aucun trafic gouverné réel n'a
traversé le service. Les compteurs Prometheus n'apparaissent qu'après leur première
incrémentation — un panel vide n'y signifie pas une requête erronée.

## Intégration avec les autres opérateurs (optionnelle)

- **ai-finops-operator** : fournit naturellement le catalogue et les politiques que GOV-AR
  lit. S'il est absent, installez les CRs catalogue vous-même (les CRDs sont dans ce chart).
- **ai-confidential-operator** : indépendant ; aucune dépendance croisée.

> Ne pas installer cet opérateur **et** le chart monolithe `ai-sovereign-finops-operator`
> sur le même cluster : les webhooks `AIChangeRequest` seraient enregistrés deux fois.
