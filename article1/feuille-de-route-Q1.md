# PROMPT CODEX — Article 1 Q1 DCasv6 / AKS SEV-SNP — FULL HARDENING v9 + SCHEDULER SECURITY

Tu es Codex. Tu travailles dans un fork PRIVÉ du dépôt :

`ai-sovereign-finops-operator`

## OVERRIDE 2026-07-05 — cible actuelle

- Registry images : GHCR uniquement,
  `ghcr.io/ihsenalaya/ai-sovereign-finops-operator`; ne pas utiliser ACR.
- Image tag de relais : `0.5.11`.
- Région/SKU AKS pour la prochaine campagne : `westus2`,
  `Standard DCasv6 Family vCPUs`, `Standard_DC8as_v6`, quota 32 vCPU.
- Résultats du papier principal : AKS réel SEV-SNP uniquement.
- `kind` et `kwok` restent CI/debug/régression, pas des résultats
  d'évaluation sécurité/performance pour le papier principal.

Tu dois continuer le projet de l’article 1 et le faire avancer vers un vrai niveau Q1.

Le directeur fait confiance pour avancer. Tu dois travailler de manière autonome, phase par phase, sans demander validation à chaque étape, mais tu ne dois jamais inventer un résultat, une citation, une mesure ou un claim expérimental.

---

# 1. CONTEXTE SCIENTIFIQUE

Article cible initial :

`Attestation-Aware Kubernetes Scheduling for Confidential AI Inference`

Avec les ressources actuellement disponibles, le quota obtenu est :

`Standard DCasv6 Family vCPUs`

Donc l’environnement expérimental réel disponible est :

- AKS ;
- westus2 ;
- Confidential VM node pool ;
- AMD SEV-SNP ;
- DCasv6 (`Standard_DC8as_v6`) ;
- attestation réelle au niveau nœud/VM si Azure Guest Attestation / MAA est validée ;
- pas de GPU ;
- pas de NVIDIA Confidential GPU ;
- pas de TDX réel.

Par conséquent, sauf si Confidential Containers / Kata-CC avec attestation pod-level est réellement activé, mesuré et vérifié, le claim doit être reformulé honnêtement.

Titre recommandé par défaut :

`Attestation-Aware Scheduling for Verifiable AI Placement on SEV-SNP Confidential Kubernetes Nodes`

ou :

`Attestation-Aware Kubernetes Scheduling for Verifiable Placement on Confidential Nodes`

Le GPU confidentiel n’est PAS requis pour terminer l’article 1.  
Le GPU confidentiel sera traité plus tard avec le quota NCC/H100.

Pour l’article 1, il faut produire une contribution forte sur :

1. l’intégration de l’evidence d’attestation vérifiée dans le cycle de scheduling Kubernetes ;
2. le re-check PreBind fail-closed anti-TOCTOU ;
3. le placement token signé et vérifiable offline ;
4. la séparation node-agent / central-verifier ;
5. la campagne adversariale A1-A10 ;
6. la baseline forte B4 ;
7. les résultats AKS réels SEV-SNP node-level ;
8. les statistiques et figures traçables ;
9. un manuscrit LaTeX honnête et défendable.

---

# 2. DÉCISION STRATÉGIQUE

Ne pas attendre le quota GPU.

Continuer maintenant avec :

`AKS DCasv6 / SEV-SNP / confidential nodes`

Le GPU doit apparaître seulement comme :

- extension future ;
- design GPU-aware si déjà présent dans la policy ;
- limitation explicite ;
- validation future avec NCCads_H100_v5.

Claim autorisé :

`This work integrates verified attestation evidence into the Kubernetes scheduling cycle and evaluates verifiable placement on real SEV-SNP confidential AKS nodes. Confidential GPU validation is left to the NCC H100 phase.`

Claims interdits :

- `We evaluate confidential GPU execution.`
- `We provide NVIDIA H100 confidential GPU results.`
- `We provide pod-level confidential AI attestation.`
- `We evaluate TDX.`
- `We evaluate pod-level TEE attestation.`
- `We evaluate Confidential GPU on DCasv6.`

---

# 3. RÈGLES ABSOLUES

Ne jamais :

- inventer résultat ;
- inventer citation ;
- inventer DOI ;
- présenter un test kind comme résultat réel AKS ;
- présenter un test engine-level comme évaluation de sécurité ;
- écrire `real MAA` sans preuve brute ;
- écrire `evidenceMode=real` si le token/rapport n’est pas réellement vérifié ;
- prétendre pod-level si l’attestation est node-level ;
- prétendre GPU ;
- prétendre TDX ;
- mélanger kind, kwok et AKS sans colonne `env` ;
- créer une baseline strawman ;
- supprimer les échecs ;
- sélectionner seulement les meilleurs runs ;
- produire des tests vides ;
- utiliser `|| true`, `skip`, `--validate=false`, `continue on error` ;
- committer kubeconfig, token, clé privée, tfstate, subscription ID ou secret ;
- pousser vers remote public.

Chaque résultat papier doit être traçable vers :

`article1/results/raw/`

Chaque figure doit venir d’un script reproductible.

Chaque ligne de résultat doit contenir au minimum :

`timestamp,env,run_id,seed,baseline,scenario,expected_result,actual_result,success,raw_log_path`

---

# 4. QUESTIONS REVIEWER Q1 À BATTRE

Créer :

`article1/status/reviewer_attack_matrix.md`

avec colonnes :

`reviewer_objection,response_section,experiment_or_test,raw_file,status,remaining_gap`

Inclure au moins ces objections :

1. Qui vérifie réellement l’attestation ?
2. Le nœud peut-il s’auto-attester ?
3. L’evidence est-elle réelle ou simulée ?
4. Est-ce node-level ou pod-level ?
5. Pourquoi ce n’est pas seulement nodeSelector ?
6. Pourquoi RuntimeClass seul ne suffit pas ?
7. Pourquoi schedulingGates + external controller ne suffit pas ?
8. Pourquoi ce n’est pas C8s ?
9. Pourquoi ce n’est pas dstack-capsule ?
10. Pourquoi ce n’est pas Confidential Containers / Trustee ?
11. Les attaques sont-elles adversariales ?
12. Les résultats kind sont-ils représentatifs ?
13. Les résultats AKS sont-ils réels ?
14. Le TOCTOU est-il mesuré ?
15. Le token de placement est-il vérifiable offline ?
16. Les résultats sont-ils statistiquement solides ?
17. Les données brutes sont-elles conservées ?
18. Les limites sont-elles honnêtes ?

Chaque objection critique doit être reliée à :
- un test ;
- une expérience ;
- un raw file ;
- une section du manuscrit.

---

# 5. GATES Q1 OBLIGATOIRES

Créer :

`article1/status/q1_gate_status.md`

Chaque gate doit avoir un statut :

`PASS | FAIL | PARTIAL | BLOCKED | NOT_STARTED`

## G0 — Evidence Trust Chain Gate

PASS seulement si :

- le node-agent ne peut pas écrire `AttestationEvidence` finale ;
- seul un `central-verifier-controller` peut create/update/status `AttestationEvidence` ;
- le node-agent crée seulement `RawAttestationReport` ;
- RBAC prouve cette séparation ;
- test envtest vérifie que le ServiceAccount node-agent est refusé ;
- A10 existe et passe.

## G1 — Real AKS SEV-SNP Attestation Gate

PASS seulement si :

- AKS réel ;
- node pool DCasv6 / SEV-SNP réel ;
- Azure Guest Attestation / MAA réellement appelé ;
- token/JWT vérifié ;
- `AttestationEvidence.evidenceMode=real` ;
- `simulated=false` ;
- claims importants hashés ;
- raw outputs conservés dans `article1/results/raw/aks/`.

Si MAA réel impossible :

- G1=FAIL ;
- documenter pourquoi ;
- ne jamais transformer cela en evidence réelle.

## G2 — Real Adversarial Campaign Gate

PASS seulement si :

- A1-A10 exécutées contre le système déployé ;
- chaque attaque utilise un ServiceAccount/kubeconfig adversarial distinct ;
- mapping ADV-1..ADV-4 ;
- CSV brut ;
- pas de “100 % blocked” sans ablation.

## G3 — Strong Baseline Gate

PASS seulement si :

- baseline B4 réellement implémentée ;
- B4 = schedulingGates + external verifier controller ;
- B4 vérifie freshness/révocation/TEE ;
- B4 comparée à B5 ;
- fenêtre TOCTOU mesurée ;
- absence de placement token vérifiable documentée.

## G4 — Performance / Scalability Gate

PASS seulement si :

- overhead AKS réel ;
- N≥30 pour sécurité/performance/race/ablation ;
- warm-up exclu mais conservé ;
- médiane, p95, p99, IC 95 % ;
- CPU/mémoire scheduler ;
- kind/kwok utilisés honnêtement.

## G5 — Manuscript Readiness Gate

PASS seulement si :

- threat model formel ;
- related work vérifié ;
- C8s / dstack-capsule discutés ;
- limitations honnêtes ;
- figures/tables reproductibles ;
- LaTeX compile ;
- aucune section stub.

## G6 — Reviewer Attack Gate

PASS seulement si :

- `reviewer_attack_matrix.md` contient au moins 15 objections ;
- chaque objection a une réponse expérimentale ou textuelle ;
- aucune objection critique n’est sans réponse.

## G7 — Evidence-to-Claim Traceability Gate

Créer :

`article1/paper/tables/claim_evidence_mapping.csv`

Colonnes :

`claim,evidence_level,environment,raw_file,limitation,status`

PASS seulement si :

- chaque claim du papier est mappé à un raw file ;
- aucun claim non supporté dans abstract/introduction/conclusion ;
- toutes les figures/tables ont script + raw input.

## G8 — Scheduler Self-Security Gate

Nouveau gate obligatoire.

L’article ne doit pas seulement sécuriser le placement des pods. Il doit aussi démontrer que le scheduler lui-même est durci, limité par RBAC, auditable et incapable d’abuser de ses privilèges au-delà de son rôle.

PASS seulement si :

- le scheduler a un ServiceAccount dédié ;
- le scheduler a un RBAC minimal ;
- le scheduler peut lire `ConfidentialInferencePolicy` ;
- le scheduler peut lire `AttestationEvidence` ;
- le scheduler peut créer/mettre à jour `AIPlacementDecision` ;
- le scheduler peut créer des `pods/binding` uniquement si nécessaire ;
- le scheduler NE PEUT PAS créer/modifier `AttestationEvidence` ;
- le scheduler NE PEUT PAS créer/modifier `RawAttestationReport` ;
- le scheduler NE PEUT PAS modifier les policies ;
- le scheduler NE PEUT PAS modifier les labels des nœuds ;
- le scheduler NE PEUT PAS lire tous les secrets du cluster ;
- la clé de signature du placement token est protégée ;
- aucun faux scheduler ne peut binder un pod sensible ;
- le scheduler est configuré en fail-closed ;
- l’image du scheduler est durcie ;
- les décisions du scheduler sont auditées ;
- les tests S1-S30 existent et passent sur kind ;
- les tests critiques S1-S12 sont rejoués sur AKS si possible.

Ce gate doit être visible dans :

```text
article1/status/q1_gate_status.md
article1/status/scheduler_security_report.md
article1/paper/sections/scheduler-security.tex
article1/paper/tables/scheduler_security_tests.csv
```


---

# 6. VARIABLES CLOUD ET EXPÉRIENCES

Créer/utiliser ces variables :

```bash
AZURE_APPLY_APPROVED=${AZURE_APPLY_APPROVED:-true}
RUN_AKS_REAL=${RUN_AKS_REAL:-true}
AZURE_REGION=${AZURE_REGION:-westus2}
AZURE_CONFIDENTIAL_VCPU_QUOTA=${AZURE_CONFIDENTIAL_VCPU_QUOTA:-32}
AZURE_CONFIDENTIAL_VM_FAMILY=${AZURE_CONFIDENTIAL_VM_FAMILY:-Standard DCasv6 Family vCPUs}
CONF_SKU=${CONF_SKU:-Standard_DC8as_v6}
REGISTRY=${REGISTRY:-ghcr.io/ihsenalaya/ai-sovereign-finops-operator}
IMAGE_TAG=${IMAGE_TAG:-0.5.11}
DESTROY_AKS_AFTER_RUN=${DESTROY_AKS_AFTER_RUN:-true}
KEEP_AKS_FOR_DEBUG=${KEEP_AKS_FOR_DEBUG:-false}
FULL_Q1_CAMPAIGN=${FULL_Q1_CAMPAIGN:-true}
N_RUNS_SECURITY=${N_RUNS_SECURITY:-30}
N_RUNS_PERFORMANCE=${N_RUNS_PERFORMANCE:-30}
N_RUNS_RACE=${N_RUNS_RACE:-30}
N_RUNS_ABLATION=${N_RUNS_ABLATION:-30}
N_RUNS_SCALABILITY=${N_RUNS_SCALABILITY:-10}
```

Avant toute commande Azure créatrice :

1. exécuter `az account show` ;
2. ne jamais écrire le subscription ID dans un fichier ;
3. vérifier région `westus2` ;
4. vérifier quota `Standard DCasv6 Family vCPUs >= 32` ;
5. vérifier quota régional total ;
6. vérifier disponibilité `Standard_DC8as_v6` ou `Standard_DC4as_v6` ;
7. vérifier disponibilité d’un system pool non confidentiel ;
8. vérifier scripts stop/destroy ;
9. écrire `article1/results/raw/aks/preflight-summary.txt`.

Après campagne AKS :

1. exporter tous les résultats ;
2. exécuter `make aks-stop` ;
3. si `DESTROY_AKS_AFTER_RUN=true`, exécuter `make aks-down` ;
4. vérifier destruction du resource group ;
5. écrire `article1/results/raw/aks/cleanup-summary.txt`.

Si `KEEP_AKS_FOR_DEBUG=true`, ne pas détruire, mais écrire :
- ressources actives ;
- coût estimé ;
- commande exacte pour détruire ;
- durée recommandée avant destruction.

---

# 7. ÉTAPE 0 — INVENTAIRE INITIAL

Créer :

`article1/status/q1_execution_inventory.md`

Inclure :

- tree utile du projet ;
- état git ;
- fichiers article1 ;
- CRDs existantes ;
- scheduler ;
- webhook ;
- token ;
- Terraform ;
- tests ;
- scripts ;
- résultats bruts existants ;
- ce qui est simulé ;
- ce qui est réel ;
- ce qui manque ;
- risques de régression ;
- commandes disponibles ;
- commandes dangereuses ;
- chemins à modifier.

Ne modifie pas les claims papier avant cet inventaire.

---

# 8. ÉTAPE 1 — ROADMAP Q1 v3

Créer :

`article1-roadmap-q1-v3.md`

Elle doit remplacer la logique checklist par une logique gates reviewer.

Inclure :

- G0 à G7 ;
- statut actuel ;
- fichiers requis ;
- tests requis ;
- expériences requises ;
- ordre de priorité ;
- risques ;
- décision node-level vs pod-level ;
- mention claire : GPU réel hors article 1.

---

# 9. ÉTAPE 2 — CLAIM SCIENTIFIQUE ET MANUSCRIT

Mettre à jour les sections LaTeX pour éviter tout sur-claim.

Formulation obligatoire :

`This work does not replace pod-level confidential-container attestation systems. Instead, it makes verified attestation evidence a first-class scheduling signal and binds the final placement decision to independently verifiable cryptographic evidence.`

Limitation obligatoire :

`When using AKS confidential VM node pools, the attestation evidence refers to the confidential worker node/VM, not to a per-pod TEE, unless a confidential-container runtime is explicitly enabled and measured. Therefore, all node-level experiments are reported as attested-node placement, not pod-level workload attestation.`

Ajouter macros LaTeX :

```latex
\newcommand{\claimnodelevel}{...}
\newcommand{\claimrealattestation}{...}
\newcommand{\claimsimulateddev}{...}
\newcommand{\claimgpufuture}{...}
```

Aucune section finale ne doit rester TODO/stub.

---

# 10. ÉTAPE 3 — TRUST CHAIN ARCHITECTURE

Implémenter/préparer :

## CRD `RawAttestationReport`

Champs :

```text
nodeName
nodeUID
provider
rawTokenRef
rawTokenHash
nonce
collectedAt
agentPodUID
agentServiceAccount
simulated
```

## `node-attestation-agent`

- tourne sur nœud confidentiel ;
- collecte quote/report/token brut ;
- ne peut PAS écrire `AttestationEvidence` finale ;
- crée seulement `RawAttestationReport`.

## `central-verifier-controller`

- lit `RawAttestationReport` ;
- vérifie MAA/claims/signature/nonce/freshness/node identity ;
- crée/écrit seul `AttestationEvidence` finale ;
- met `evidenceMode=real` seulement après vérification réelle ;
- met `evidenceMode=simulated` seulement kind/dev.

## RBAC

- node-agent : create `RawAttestationReport` seulement ;
- verifier : create/update/status `AttestationEvidence` ;
- scheduler : read-only `AttestationEvidence`.

Tests obligatoires :

- node-agent cannot create `AttestationEvidence` ;
- node-agent cannot update `AttestationEvidence` ;
- verifier can create `AttestationEvidence` ;
- scheduler cannot write `AttestationEvidence` ;
- compromised node ServiceAccount attack blocked.

Mettre à jour `AttestationEvidence` :

```text
evidenceMode
verifiedBy
verifierPodUID
nodeName
nodeUID
provider
teeType
attestationType
maaTokenHash
claimsDigest
nonce
issuedAt
expiresAt
freshnessSeconds
simulated
verificationStatus
failureReason
conditions
```

---

# 11. ÉTAPE 4 — ATTAQUE A10

Créer :

`article1/experiments/attacks/a10_compromised_node_forge_evidence.sh`

Objectif :

Un nœud compromis ou son agent local tente de créer/modifier une `AttestationEvidence` favorable.

Résultat attendu :

1. RBAC refuse l’écriture directe ;
2. si un `RawAttestationReport` frauduleux est accepté, le verifier central refuse de produire une `AttestationEvidence` finale valide ;
3. scheduler refuse le placement ;
4. `verify-placement` échoue ou aucune décision valide n’existe.

Créer :

```text
article1/results/raw/kind/a10_compromised_node_forge_evidence.csv
article1/results/raw/aks/a10_compromised_node_forge_evidence.csv
```

Colonnes :

`timestamp,env,attack_id,adversary,service_account,attempted_action,expected_result,actual_result,blocked,reason,raw_log_path`

---

# 12. ÉTAPE 5 — MODÈLE D’ADVERSAIRE FORMEL

Créer/mettre à jour :

```text
article1/paper/sections/threat-model.tex
article1/paper/tables/adversary_attack_mapping.csv
```

Adversaires :

- ADV-1 namespace-scoped application user ;
- ADV-2 compromised non-confidential node ;
- ADV-3 compromised confidential node post-attestation ;
- ADV-4 object/label manipulation adversary.

Hors périmètre :

- API server compromis ;
- etcd compromis ;
- cloud provider totalement malveillant au-delà du modèle SEV-SNP ;
- side channels ;
- attaques physiques ;
- compromission de la clé privée du signer.

Table :

`Attack,adversary,capability,target_property,expected_defense,experiment_source`

---

# 13. ÉTAPE 6 — SCRIPTS AKS SAFE CAMPAIGN

Créer :

```text
article1/experiments/aks/README_AKS_CAMPAIGN.md
article1/experiments/aks/estimate_cost.sh
article1/experiments/aks/aks_up.sh
article1/experiments/aks/aks_stop.sh
article1/experiments/aks/aks_destroy.sh
article1/experiments/aks/run_full_campaign.sh
```

Le README doit expliquer :

- région ;
- SKU ;
- nombre de nœuds ;
- quota ;
- coût estimé ;
- ordre des commandes ;
- comment vérifier la destruction ;
- où les logs sont sauvegardés.

La campagne doit produire :

```text
article1/results/raw/aks/preflight-summary.txt
article1/results/raw/aks/attestation-real-summary.json
article1/results/raw/aks/security_attacks_A1_A10.csv
article1/results/raw/aks/scheduling_latency.csv
article1/results/raw/aks/prebind_race.csv
article1/results/raw/aks/ablation.csv
article1/results/raw/aks/verify_placement_audit.csv
article1/results/raw/aks/resource_usage_scheduler.csv
article1/results/raw/aks/cleanup-summary.txt
```

---

# 14. ÉTAPE 7 — REAL MAA / GUEST ATTESTATION

Créer package :

`pkg/attestation/maa`

Fonctions attendues :

```text
ParseMAAToken(token string)
VerifyMAAToken(token string, options)
ExtractClaims(token string)
ComputeClaimsDigest(claims)
ValidateFreshness(claims, nonce, maxAge)
ValidateTEEClaims(claims, expectedTEE)
ValidateNodeBinding(claims, nodeName/nodeUID when available)
Return VerificationResult
```

Ne jamais coder un faux succès.

Si vérification réelle impossible :

```text
verificationStatus=Unavailable
evidenceMode=unverified
```

et ne jamais transformer cela en :

```text
evidenceMode=real
```

Tests :

- malformed token -> fail ;
- expired token -> fail ;
- wrong nonce -> fail ;
- wrong TEE type -> fail ;
- missing required claim -> fail ;
- simulated token cannot be marked real ;
- real mode requires verifier signature/status.

---

# 15. ÉTAPE 8 — BASELINE B4 FORTE

Implémenter réellement B4 :

`internal/baselines/schedulinggate/`

Comportement :

- pod créé avec `schedulingGate` ;
- external controller vérifie policy + evidence + freshness + revocation ;
- si OK, enlève schedulingGate ;
- scheduler standard fait ensuite le placement ;
- mesurer fenêtre TOCTOU entre gate removal et bind final ;
- montrer que B4 ne produit pas le même binding cryptographique de décision que B5 ;
- ne pas caricaturer B4.

Créer :

```text
article1/experiments/baselines/run_b4_vs_b5.sh
article1/results/raw/aks/b4_vs_b5.csv
```

Colonnes :

`env,run_id,baseline,attack_id,gate_check_time_ms,bind_time_ms,toctou_window_ms,placement_verified,blocked,latency_p50,latency_p95,latency_p99,notes`

---


# 15. ÉTAPE 9 — SÉCURITÉ DU SCHEDULER LUI-MÊME

Cette étape est obligatoire.  
Le scheduler devient un composant critique de sécurité.  
Il faut donc tester et documenter sa propre sécurité.

Créer :

```text
article1/status/scheduler_security_report.md
article1/paper/sections/scheduler-security.tex
article1/paper/tables/scheduler_security_tests.csv
article1/experiments/scheduler-security/
article1/experiments/scheduler-security/run_scheduler_security_tests.sh
```

## 15.1 RBAC minimal du scheduler

Le scheduler doit avoir uniquement les droits nécessaires.

Autorisé :

```text
get/list/watch Pods
get/list/watch Nodes
get/list/watch Namespaces si nécessaire au matching policy
get/list/watch ConfidentialInferencePolicy
get/list/watch AttestationEvidence
create/update/status AIPlacementDecision
create Events
create pods/binding si nécessaire au second scheduler
get Secret contenant uniquement sa clé de signature si mode Secret local
```

Interdit :

```text
create/update/delete AttestationEvidence
create/update/delete RawAttestationReport
create/update/delete ConfidentialInferencePolicy
patch/update Nodes ou labels de nœuds
read tous les Secrets du cluster
create/update arbitrary Pods
create/update MutatingWebhookConfigurations sauf si non nécessaire
cluster-admin
wildcards "*" sur resources ou verbs
```

Créer des tests RBAC :

```text
S1 scheduler cannot create AttestationEvidence
S2 scheduler cannot update AttestationEvidence
S3 scheduler cannot create RawAttestationReport
S4 scheduler cannot update ConfidentialInferencePolicy
S5 scheduler cannot patch Node labels
S6 scheduler cannot read arbitrary Secrets
S7 scheduler can read AttestationEvidence
S8 scheduler can create AIPlacementDecision
S9 scheduler can create pods/binding only if required
```

Chaque test doit produire une ligne CSV :

```text
timestamp,env,test_id,service_account,verb,resource,namespace,expected,actual,pass,raw_log_path
```

## 15.2 Protection de la clé de signature

Le placement token est signé par le scheduler ou par un signer associé.  
La clé privée est donc critique.

Exigences :

- aucune clé privée dans Git ;
- aucune clé privée dans values.yaml ;
- aucun secret dans les logs ;
- clé de dev/kind stockée dans un Secret dédié ;
- seul le ServiceAccount scheduler peut lire ce Secret ;
- keyID/version inclus dans le placement token ;
- support de rotation de clé ou au minimum documentation claire ;
- future production : KMS/Vault/signer externe recommandé.

Tests :

```text
S10 fake scheduler cannot read signing key
S11 token signed with old/unknown keyID fails verify-placement
S12 modified token signature fails verify-placement
```

Dans le papier, écrire honnêtement :

```text
In the prototype, the scheduler signing key is stored in a dedicated Kubernetes Secret restricted to the scheduler ServiceAccount. For production deployments, the signer should be externalized to a KMS/Vault-backed signing service to reduce the impact of scheduler compromise.
```

## 15.3 Durcissement du pod scheduler

Le pod scheduler doit être déployé avec :

```yaml
runAsNonRoot: true
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
seccompProfile:
  type: RuntimeDefault
```

Interdit sauf justification documentée :

```text
privileged: true
hostPID: true
hostIPC: true
hostNetwork: true
hostPath mounts
automountServiceAccountToken inutile
```

Ajouter tests Helm/Kubernetes manifest :

```text
S13 scheduler pod runs as non-root
S14 scheduler root filesystem read-only
S15 scheduler drops all capabilities
S16 scheduler has no hostPath
S17 scheduler is not privileged
S18 scheduler has seccomp RuntimeDefault
```

## 15.4 NetworkPolicy du scheduler

Créer une NetworkPolicy si le cluster supporte CNI policy.

Objectif :

- autoriser scheduler → kube-apiserver ;
- autoriser Prometheus → scheduler metrics ;
- bloquer egress inutile ;
- bloquer ingress inutile.

Tests :

```text
S19 scheduler metrics reachable only from allowed namespace
S20 scheduler cannot egress to arbitrary external endpoint if NetworkPolicy enabled
```

Si NetworkPolicy n’est pas applicable dans kind, marquer :

```text
NOT_APPLICABLE_KIND_CNI
```

mais tester ou documenter sur AKS si possible.

## 15.5 Protection contre faux scheduler

Un attaquant ne doit pas pouvoir lancer un faux scheduler qui bind un pod sensible.

Créer tests :

```text
S21 fake scheduler ServiceAccount cannot create pods/binding
S22 fake scheduler cannot create AIPlacementDecision
S23 fake scheduler cannot produce valid placement token because it lacks signing key
```

Résultat attendu :

```text
blocked=true
reason=RBAC_DENIED or INVALID_SIGNATURE
```

## 15.6 Fail-closed du scheduler

Tester :

```text
S24 policy API unavailable -> reject
S25 evidence API unavailable -> reject
S26 signing key unavailable -> reject
S27 PreBind API read error -> reject
S28 malformed evidence -> reject
S29 missing evidence -> reject
S30 invalid signature path -> reject
```

Aucun de ces cas ne doit finir en placement accepté.

## 15.7 Audit du scheduler

Chaque décision critique du scheduler doit être loggée :

```text
pod_received
policy_loaded
policy_hash_captured
evidence_loaded
evidence_hash_captured
node_rejected
node_scored
reserve_decision
permit_wait
prebind_recheck
prebind_reject
bind_success
placement_token_signed
placement_decision_created
```

Ajouter métriques :

```text
ai_scheduler_rbac_denied_total
ai_scheduler_fail_closed_total
ai_scheduler_prebind_reject_total
ai_scheduler_token_sign_total
ai_scheduler_token_sign_fail_total
ai_scheduler_fake_scheduler_blocked_total
ai_scheduler_security_test_pass_total
ai_scheduler_security_test_fail_total
```

## 15.8 Expérience dédiée pour l’article

Créer une expérience :

```text
Exp 7 — Scheduler Self-Security Evaluation
```

Elle doit répondre :

```text
Can the scheduler itself be abused to forge evidence, bypass policy, bind pods, or sign invalid placements?
```

Inclure dans les résultats :

```text
article1/results/raw/kind/scheduler_security_tests.csv
article1/results/raw/aks/scheduler_security_tests.csv
article1/results/tables/scheduler_security.csv
```

Inclure dans le papier :

- section `Scheduler Self-Security`;
- tableau `Scheduler security tests`;
- discussion : que se passe-t-il si le scheduler est totalement compromis ?
- limitation honnête : si le scheduler et sa clé privée sont entièrement compromis, il peut signer de mauvaises décisions ; mitigation future = signer externe KMS/Vault.

Phrase obligatoire :

```text
The scheduler is part of the trusted computing base of the prototype. Therefore, we minimize its privileges, separate it from the attestation verifier, restrict its signing key, test fail-closed behavior, and evaluate whether fake schedulers or compromised service accounts can forge placement decisions.
```


# 16. ÉTAPE 10 — EXPÉRIENCES Q1

## Exp 1 — Security adversarial AKS

- A1-A10 ;
- principals distincts ;
- N≥30 ;
- résultat CSV brut ;
- mapping ADV-x.

## Exp 2 — Scheduling overhead AKS

- admission/filter/score/permit/prebind/bind latency ;
- N≥30 ;
- warm-up exclu ;
- p50/p95/p99/IC95 ;
- CPU/memory scheduler.

## Exp 3 — Scalability

- kind : 10/50/100/500 pods si hôte OK ;
- kwok : scheduler-only ;
- CPU/memory sous charge ;
- claims limités.

## Exp 4 — Race/freshness

- revoke-after-Filter ;
- expire-during-scheduling ;
- policy-update-during-scheduling ;
- comparaison B4 vs B5 ;
- au moins revoke-after-Filter sur AKS.

## Exp 5 — Ablation

Configurations :

- policy only ;
- + admission ;
- + Filter ;
- + PreBind ;
- + token/verify ;
- full system.

Mesurer :

- blocked rate ;
- bypass rate ;
- latency ;
- verifiability.

## Exp 6 — Identity binding / Verifiable placement

- prendre un vrai `AIPlacementDecision` AKS ;
- vérifier offline avec clé publique seule ;
- muter chaque champ :
  - podUID ;
  - nodeUID ;
  - policyHash ;
  - evidenceHash ;
  - timestamp ;
  - nonce ;
  - signer ;
  - decisionID ;
- chaque mutation doit donner `verify FAIL` avec raison précise.

Créer :

```text
article1/results/tables/security.csv
article1/results/tables/performance.csv
article1/results/tables/scalability.csv
article1/results/tables/ablation.csv
article1/results/tables/identity_binding.csv
article1/results/tables/scheduler_security.csv
```

---

# 17. ÉTAPE 11 — STATS ET FIGURES

Créer scripts reproductibles :

```text
article1/scripts/analyze_security.py
article1/scripts/analyze_performance.py
article1/scripts/analyze_scalability.py
article1/scripts/analyze_ablation.py
article1/scripts/make_figures.py
```

Stats :

- median ;
- p95 ;
- p99 ;
- bootstrap CI 95 % ;
- Mann-Whitney U pour B4 vs B5 si applicable ;
- effect size si possible ;
- taille d’échantillon ;
- colonne `env`.

Figures attendues :

```text
article1/paper/figures/security_bar.pdf
article1/paper/figures/scheduling_latency_cdf.pdf
article1/paper/figures/scalability_line.pdf
article1/paper/figures/toctou_window_b4_b5.pdf
article1/paper/figures/ablation_security_latency.pdf
article1/paper/figures/identity_binding_matrix.pdf
article1/paper/figures/scheduler_security_matrix.pdf
```

---

# 18. ÉTAPE 12 — RELATED WORK Q1

Créer :

```text
article1/paper/sections/related-work.tex
article1/paper/references_verified.bib
article1/paper/literature_audit.md
```

Ne pas inventer de citations.

Faire une recherche bibliographique réelle. Ajouter seulement des références vérifiées.

Sections minimales :

1. Kubernetes scheduling and scheduler framework ;
2. Kubernetes admission control and scheduling gates ;
3. Confidential computing and TEEs ;
4. AMD SEV-SNP / Intel TDX / remote attestation ;
5. Confidential Containers / Kata / CoCo ;
6. Confidential Kubernetes systems : C8s ;
7. Pod-level attestation : dstack-capsule ;
8. Cluster-level confidential systems : Constellation ;
9. Difference from this work.

Positionnement obligatoire :

- C8s : architecture confidentielle Kubernetes plus large, notre travail cible l’intégration de l’evidence vérifiée dans le scheduling et le placement token.
- dstack-capsule : pod-level remote attestation ; notre système ne revendique pas pod-level sauf si implémenté.
- Confidential Containers : runtime/isolation/attestation de containers ; notre système orchestre la décision de placement selon l’attestation.
- Constellation : cluster confidentiel ; ne pas le présenter comme baseline directe de scheduler.
- Occlum : LibOS SGX, background seulement, pas baseline directe.

---

# 19. ÉTAPE 13 — MANUSCRIT LATEX

Structure recommandée :

1. Introduction
2. Motivation and Problem Statement
3. Threat Model and Trust Assumptions
4. System Design
5. Evidence Trust Chain
6. Attestation-Aware Scheduling Algorithm
7. Verifiable Placement Token
8. Scheduler Self-Security
9. GPU-aware Extension and Scope
10. Implementation
11. Experimental Methodology
12. Security Evaluation
13. Scheduler Security Evaluation
14. Performance and Scalability
15. Ablation and Baseline Comparison
16. Related Work
17. Threats to Validity
18. Limitations
19. Conclusion

Section GPU obligatoire :

`GPU-aware Extension and Scope`

Contenu :
- policy peut exprimer `requireConfidentialGPU` si présent ;
- DCasv6 ne fournit pas de GPU ;
- Article 1 ne rapporte aucun résultat confidential GPU ;
- GPU NCC/H100 est future validation ;
- scheduler doit fail-closed si une policy exige GPU confidentiel sans evidence GPU réelle.

Ajouter test/attaque optionnelle :

`A11 — Confidential GPU required but no real GPU evidence`

Résultat attendu :
- pod non schedulé ;
- reason = `CONFIDENTIAL_GPU_EVIDENCE_MISSING`;
- aucune revendication GPU réelle.

Aucune section finale ne doit rester “TODO” ou “stub”.

---

# 20. ÉTAPE 14 — CHECKLIST GO/NO-GO

Créer :

`article1/paper/q1_readiness_checklist.md`

Inclure :

- MAA real evidence present? yes/no + path ;
- central verifier implemented? yes/no + tests ;
- node-agent blocked from writing AttestationEvidence? yes/no + test path ;
- A10 passed on AKS? yes/no + CSV path ;
- A1-A10 AKS completed? yes/no ;
- B4 implemented? yes/no ;
- B4 vs B5 measured? yes/no ;
- scheduler self-security tests S1-S30 passed? yes/no + CSV path ;
- fake scheduler blocked? yes/no + test path ;
- scheduler cannot modify AttestationEvidence? yes/no + test path ;
- scheduler signing key protected? yes/no + test path ;
- scheduler fail-closed behavior tested? yes/no + test path ;
- performance AKS N≥30? yes/no ;
- figures generated from scripts? yes/no ;
- all claims traceable? yes/no ;
- related work verified? yes/no ;
- limitations honest? yes/no ;
- GPU claims absent or future-only? yes/no ;
- artifact/IP status reviewed? yes/no.

---

# 21. ÉTAPE 15 — COMMANDES DE VALIDATION

À la fin, exécuter ou préparer :

```bash
go test ./...
make test
make manifests
make generate
make docker-build
make kind-e2e
make article1-analyze
make article1-figures
latexmk -pdf article1/paper/main.tex
```

Si une commande échoue :

- corriger si possible ;
- sinon écrire la cause exacte dans :

`article1/status/q1_execution_report.md`

---

# 22. ÉTAPE 16 — RAPPORT FINAL

Créer :

`article1/status/q1_execution_report.md`

Le rapport doit contenir :

- résumé des modifications ;
- fichiers créés/modifiés ;
- tests passés ;
- tests échoués ;
- expériences réellement exécutées ;
- expériences préparées mais non exécutées ;
- résultats disponibles ;
- éléments encore bloquants ;
- commandes exactes à lancer sur AKS ;
- statut du nettoyage Azure ;
- risques scientifiques restants ;
- go/no-go submission verdict.

Verdicts possibles :

```text
GO_Q1_DRAFT
NO_GO_MISSING_REAL_ATTESTATION
NO_GO_WEAK_BASELINE
NO_GO_INSUFFICIENT_ADVERSARIAL_EVAL
NO_GO_RELATED_WORK_NOT_VERIFIED
NO_GO_RESULTS_NOT_REPRODUCIBLE
NO_GO_GPU_CLAIMS_OVERSTATED
NO_GO_SCHEDULER_SECURITY_NOT_TESTED
```

---

# 23. ORDRE D’EXÉCUTION OBLIGATOIRE

1. inventaire ;
2. roadmap v3 ;
3. reviewer attack matrix ;
4. claim evidence mapping ;
5. trust chain central verifier + RBAC ;
6. scheduler self-security gate G8 ;
7. A10 ;
8. threat model ;
9. scripts AKS safe campaign ;
10. real MAA package ;
11. B4 baseline ;
12. experiments including Exp 7 Scheduler Self-Security ;
13. analysis/figures ;
14. related work ;
15. LaTeX ;
16. final checklist ;
17. final report.

Ne commence pas par embellir le papier.

Commence par rendre la preuve scientifique solide.

Le meilleur résultat Q1 est un résultat honnête, robuste, traçable et défendable.
