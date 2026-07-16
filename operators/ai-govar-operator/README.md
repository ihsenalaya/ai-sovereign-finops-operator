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
décisions, latences p95/p99, transitions du ledger, retard de réconciliation, âge de la
dernière passe réussie).

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

## Intégration avec les autres opérateurs (optionnelle)

- **ai-finops-operator** : fournit naturellement le catalogue et les politiques que GOV-AR
  lit. S'il est absent, installez les CRs catalogue vous-même (les CRDs sont dans ce chart).
- **ai-confidential-operator** : indépendant ; aucune dépendance croisée.

> Ne pas installer cet opérateur **et** le chart monolithe `ai-sovereign-finops-operator`
> sur le même cluster : les webhooks `AIChangeRequest` seraient enregistrés deux fois.
