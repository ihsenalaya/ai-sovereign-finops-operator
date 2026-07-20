# AI Confidential Operator

Opérateur Kubernetes **autonome** de gouvernance confidential-computing pour workloads IA :
chaîne d'attestation **RATS** (SEV-SNP/TDX, simulée en kind), placement vérifiable signé
(Ed25519), libération de clés conditionnée à l'attestation, révocation bornée et journal
d'audit chaîné. Il héberge aussi les **webhooks pods** (injection sidecar + validation
confidentielle).

C'est l'un des **3 opérateurs indépendants** issus de la décomposition de l'opérateur
`ai-sovereign-finops-operator` (voir [`../README.md`](../README.md)). Il s'installe et
fonctionne **seul** — sans l'opérateur FinOps ni l'opérateur GOV-AR.

## Contenu du dossier

```
ai-confidential-operator/
├── README.md                        ← ce fichier
├── chart/ai-confidential-operator/  ← Helm chart indépendant (7 CRDs + RBAC scopé)
├── docs/                            ← une fiche par CRD
└── automatisation/
    ├── up.sh / down.sh              ← cluster kind complet en une commande (mode simulé)
    ├── test-apps/                   ← chaîne d'attestation simulée + policies + pod démo
    └── dashboards/                  ← dashboard Grafana "Attestation & Placement"
```

## Architecture single-writer

```
node-attestation-agent (DaemonSet)          central-verifier (Deployment)
        │  émet (non fiable)                        │  UNIQUE writer
        ▼                                           ▼
RawAttestationReport ──── appraisal RATS ──→ AttestationEvidence (status)
                                                    │
                              ┌─────────────────────┼──────────────────┐
                              ▼                     ▼                  ▼
                    AIPlacementDecision      AIKeyReleasePolicy   AIEvidenceRecord
                    (scheduler, token        (key-release-        (audit chaîné,
                     Ed25519 signé)           gateway)             ancrage externe)
```

Le manager de cet opérateur (`operateur/cmd/confidential-manager`) réconcilie les CRDs,
bootstrappe les **runtime classes simulées** (`simulated-kata-qemu-tdx/snp`) et les webhooks
pods. L'appraisal réel reste dans le binaire dédié `central-verifier` (RBAC prouve le
single-writer) ; en kind, l'évidence simulée est explicite (`simulated: true`, jamais
convertible en évidence réelle).

## Fonctionnement

La chaîne suit le modèle **RATS** (Remote ATtestation procedureS) avec une séparation
stricte émetteur / vérifieur / consommateur :

1. **Ingestion** — chaque nœud TEE émet un `RawAttestationReport` (rapport brut,
   **non fiable par construction**) via le `node-attestation-agent`. En kind, le
   provider `simulator` produit des rapports simulés explicites.
2. **Appraisal (single-writer)** — seul le `central-verifier` (ou le mode simulé du
   manager en dev) a le droit RBAC d'écrire une `AttestationEvidence` : le rapport brut
   est appraisé (mesures, TCB, fraîcheur) et l'évidence résultante porte
   `simulated: true|false` — une évidence simulée n'est **jamais convertible** en
   évidence réelle.
3. **Placement** — `AIPlacementDecision` confronte les évidences valides aux exigences
   de la `ConfidentialInferencePolicy` (type de TEE, fraîcheur, runtime class, digest
   d'image) et publie une décision **signée Ed25519**, vérifiable hors cluster.
4. **Webhooks pods** — le manager héberge le mutating webhook (injection du sidecar
   confidentiel) et le validating webhook (conformité runtime class / policy). En mode
   `warn` il annote, en mode `enforce` il **rejette** les pods non conformes. Les
   namespaces système et celui de l'opérateur sont exclus (anti-deadlock).
5. **Libération de clés** — `AIKeyReleasePolicy` conditionne la libération d'une clé
   (via le `key-release-gateway`) à la présence d'une évidence valide : pas
   d'attestation → pas de clé → pas de données déchiffrées.
6. **Révocation & audit** — `AIRevocationPolicy` révoque de façon **bornée** (par nœud,
   par période) évidences et placements ; chaque étape est journalisée dans
   `AIEvidenceRecord`, un journal **append-only chaîné** (hash précédent → suivant)
   ancrable sur un checkpoint externe.
7. **Bootstrap dev** — sur kind, le manager crée automatiquement les runtime classes
   simulées `simulated-kata-qemu-tdx` / `simulated-kata-qemu-snp` et expose la métrique
   `ai_simulated_runtimeclass_in_use` pour rendre le mode simulé **visible** (jamais
   silencieux).

## Fonctionnalités

- **Chaîne d'attestation RATS complète** : rapport brut → appraisal → évidence typée,
  avec séparation émetteur/vérifieur prouvée par le RBAC (single-writer).
- **Mode simulé honnête** pour kind/dev : SEV-SNP/TDX simulés, marqués comme tels de
  bout en bout (CRs, métriques, dashboard) — impossible à confondre avec du TEE réel.
- **Placement vérifiable signé** (Ed25519) : la décision de scheduler un workload
  confidentiel est un artefact vérifiable, pas un effet de bord.
- **Politiques par workload** (`ConfidentialInferencePolicy`) : exigences TEE,
  fraîcheur d'évidence, runtime class, digest d'image, `reportOnly`/`warn`/`enforce`.
- **Key-release conditionné à l'attestation** : libération de clés seulement contre
  évidence valide, avec raisons de refus auditables.
- **Révocation bornée** : rayon d'action limité par policy (pas de révocation globale
  accidentelle).
- **Audit chaîné append-only** avec ancrage externe optionnel (non-répudiation).
- **Injection & validation de pods** par webhooks, avec exclusions anti-deadlock.

## CRDs possédées (7)

| CRD | shortName | Rôle | Doc |
|---|---|---|---|
| ConfidentialInferencePolicy | `cip` | Exigences TEE/fraîcheur/runtime d'un workload | [docs/confidentialinferencepolicy.md](docs/confidentialinferencepolicy.md) |
| RawAttestationReport | `rar` | Rapport brut non appraisé (agent nœud) | [docs/rawattestationreport.md](docs/rawattestationreport.md) |
| AttestationEvidence | `aevid` | Évidence appraisée (single-writer) | [docs/attestationevidence.md](docs/attestationevidence.md) |
| AIPlacementDecision | `apd` | Décision de placement signée | [docs/aiplacementdecision.md](docs/aiplacementdecision.md) |
| AIKeyReleasePolicy | `akrp` | Libération de clé conditionnée à l'attestation | [docs/aikeyreleasepolicy.md](docs/aikeyreleasepolicy.md) |
| AIRevocationPolicy | `airvp` | Révocation bornée d'évidence/placement | [docs/airevocationpolicy.md](docs/airevocationpolicy.md) |
| AIEvidenceRecord | `aier` | Journal d'audit append-only chaîné | [docs/aievidencerecord.md](docs/aievidencerecord.md) |

## Installation

### Prérequis

- Kubernetes ≥ 1.29, Helm ≥ 3.12.
- **Production** : nœuds SEV-SNP/TDX (ex. AKS confidential), Microsoft Azure Attestation
  (provider `maa`), les déploiements `central-verifier`, `node-attestation-agent` et
  `key-release-gateway` (images publiées sur le même registre).
- **kind/dev** : rien d'autre — le mode simulé est bootstrappé automatiquement.

### Installation du chart

```bash
helm install confidential ./chart/ai-confidential-operator \
  --namespace confidential-system --create-namespace
```

Image par défaut : `ghcr.io/ihsenalaya/ai-sovereign-finops-operator/confidential-operator`.
Valeurs utiles :

```bash
# Image locale (kind) :
--set image.repository=confidential-operator --set image.tag=dev --set image.pullPolicy=Never
# Prometheus Operator :
--set metrics.serviceMonitor.enabled=true --set metrics.serviceMonitor.labels.release=monitoring
# Fallbacks embarqués dev-only (le single-writer reste le central-verifier) :
--set compat.enableEmbeddedVerifier=true
```

### Vérification

```bash
kubectl -n confidential-system get deploy
kubectl get runtimeclasses | grep simulated       # bootstrappées par l'opérateur
kubectl get mutatingwebhookconfigurations aiops-sidecar-injector
```

## Démarrage rapide kind (tout-en-un)

```bash
cd automatisation
./up.sh          # kind + Prometheus/Grafana + opérateur + chaîne simulée + dashboard
```

Les applications de test (`confidential-demo`) installent : une
`ConfidentialInferencePolicy` (warn), un `RawAttestationReport` simulé, une
`AttestationEvidence` SEV-SNP simulée, un `AIEvidenceRecord`, une `AIKeyReleasePolicy`,
une `AIRevocationPolicy` et un **pod démo** sur la runtime class `simulated-kata-qemu-snp`
(validé par le webhook).

```bash
kubectl -n confidential-demo get cip,rar,aevid,akrp,airvp,aier
kubectl -n confidential-demo get pod confidential-demo-app -o jsonpath='{.spec.runtimeClassName}'
```

Grafana : `kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80`
→ dashboard **AI Confidential Operator — Attestation & Placement** (réconciliations
par contrôleur et erreurs, taux de succès et latence p99 des webhooks pods, workqueue,
runtime class simulée en usage, latences de réconciliation p50/p95/p99).

Démontage : `./down.sh`.

## Utilisation

### Passer en mode enforce

```yaml
spec:
  enforcementMode: enforce   # le webhook rejette les pods non conformes
```

Le webhook de validation exclut les namespaces système et le namespace de l'opérateur
(anti-deadlock) ; la politique s'applique aux namespaces sélectionnés par `target`.

### Production (TEE réel)

1. Déployer `central-verifier` + `node-attestation-agent` (DaemonSet) + `key-release-gateway`.
2. Les agents émettent des `RawAttestationReport` (provider `maa`) ; le verifier produit les
   `AttestationEvidence` avec `evidenceMode: real`.
3. `ConfidentialInferencePolicy` en `enforce`, avec `requireImageDigest` et TTL courts.
4. La chaîne d'audit `AIEvidenceRecord` peut être ancrée sur un checkpoint externe.

## Intégration avec les autres opérateurs (optionnelle)

Aucune dépendance : FinOps et GOV-AR fonctionnent sans cet opérateur, et réciproquement.
Sur un même cluster, chaque chart n'installe que ses CRDs.

> Ne pas installer cet opérateur **et** le chart monolithe `ai-sovereign-finops-operator`
> sur le même cluster : les deux enregistreraient les webhooks pods.
