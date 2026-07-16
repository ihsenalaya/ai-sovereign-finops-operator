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
→ dashboard **AI Confidential Operator — Attestation & Placement** (évidences actives,
simulé vs TEE réel, décisions de placement, taux d'allow du key-release, raisons de deny,
révocations, âge du checkpoint d'audit, runtime simulée).

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
