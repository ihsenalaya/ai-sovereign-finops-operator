# PROMPT CODEX — Plateforme AI Confidential Governance (v2)

⚠️ NOTE À L'OPÉRATEUR HUMAIN (à lire avant de lancer Codex — ne concerne pas Codex)
==================================================================================
1. Ce prompt fait implémenter ET documenter le protocole de libération conditionnelle
   des clés (key-release-gateway, placement token, docs/security/key-release-protocol.md).
   Tant que les règles IP/brevet de l'institution ne sont pas clarifiées, exécuter ce
   prompt UNIQUEMENT sur un fork/dépôt PRIVÉ. Ne rien pousser vers le dépôt public
   ai-sovereign-finops-operator pour les composants trust/key-release/scheduler.
   En Europe, une publication (y compris un push GitHub public) détruit la nouveauté.
2. Ne pas lancer les 10 phases en une seule session. Donner ce prompt en préambule,
   puis demander explicitement : « Exécute Phase 0 et Phase 1 seulement », et itérer.

CHANGEMENTS v2 (résumé — peut rester dans le fichier)
=====================================================
- Ajout du MODE D'EXÉCUTION PAR PHASES (stop obligatoire entre phases).
- Interdiction pour Codex de pousser vers un remote (commits locaux uniquement).
- governance-operator : mise à niveau statistique du calcul du quality gate
  (non-infériorité, percentiles, score composite, hystérésis, SPRT optionnel).
- confidential-execution-operator : stratégie RuntimeClass simulée en kind
  (les handlers kata-qemu-tdx/snp n'existent pas dans kind).
- attestation-scheduler : scaffold scheduler-plugins, matrice de versions Kubernetes,
  cible ≥ 1.34 (DRA GA) pour que la préparation Confidential GPU soit réelle.
- trust-evidence-operator : ancrage externe de la chaîne d'audit (checkpoints signés)
  + maîtrise de la volumétrie des AIEvidenceRecord (batching, rétention).
- Métriques, tests, docs et Définition de Done mis à jour en conséquence.

======================================================================

Tu es Codex. Tu travailles dans le dépôt racine du projet ai-sovereign-finops-operator
(fork privé de travail).

MODE D'EXÉCUTION PAR PHASES
===========================
- Sauf instruction contraire explicite dans le message qui accompagne ce prompt,
  exécute UNIQUEMENT la ou les phases demandées (par défaut : Phase 0 puis Phase 1),
  puis STOPPE et produis le rapport de fin de session (format en fin de prompt).
- Ne commence jamais une phase suivante « pour avancer » sans demande explicite.
- Ne pousse JAMAIS vers un remote (pas de git push, pas de PR). Commits locaux
  uniquement, messages de commit clairs par phase.
- Si une phase est trop grosse pour une session, découpe-la, termine un sous-lot
  propre et testé, et dis exactement ce qui reste.

OBJECTIF GLOBAL
================
Transformer l'opérateur existant en une plateforme Kubernetes modulaire pour ma
thèse de doctorat et ma future startup.

Le sujet scientifique à supporter est :

"Attestation-Aware Kubernetes Scheduling for Confidential AI Inference:
Verifiable Placement, Conditional Key Release and Auditable Evidence"

Le système final doit permettre de tester et démontrer :

1. le placement vérifiable de workloads IA sensibles ;
2. l'utilisation de Confidential Containers via RuntimeClass/Kata ;
3. l'intégration future du Confidential GPU via DRA/GPU Operator/NVIDIA Confidential Computing ;
4. l'attestation CPU/GPU/runtime ;
5. la libération conditionnelle des clés ;
6. la révocation bornée ;
7. un registre d'audit append-only avec ancrage externe ;
8. des expérimentations reproductibles dans kind local ;
9. une future validation AKS privée, avec GPU réel testé plus tard ;
10. une UI privée pour gérer/configurer/auditer la plateforme.

CONTRAINTE ABSOLUE
==================
Ne rien inventer.
Ne pas créer de faux succès.
Ne pas masquer les erreurs.
Ne pas utiliser de tests vides.
Ne pas ajouter de "TODO test later" pour faire passer la CI.
Ne pas utiliser "|| true", "--validate=false", "skip if failing", ou des contournements similaires.
Ne pas désactiver des webhooks, des validations ou des contrôleurs juste pour que les tests passent.
Ne jamais prétendre que Confidential GPU est testé localement dans kind.
Ne jamais pousser vers un remote git.
Dans kind, si une preuve d'attestation, un GPU ou une RuntimeClass est simulé(e),
cela doit être explicitement marqué "simulated", interdit par défaut en mode
production, et visible dans les logs, CRDs, métriques et rapports.

Avant de coder :
1. inspecte le dépôt actuel ;
2. identifie le layout existant ;
3. identifie le groupe API existant ;
4. identifie le framework utilisé : kubebuilder/controller-runtime/Helm/Makefile/GitHub Actions ;
5. identifie la version Kubernetes effective (go.mod, image kind, CI) et note-la
   dans docs/architecture/version-matrix.md ;
6. conserve les fonctionnalités existantes : FinOps, souveraineté, routage, quality gate, reporting, shadow AI ;
7. ne casse pas les CRDs existantes ;
8. si une migration API est nécessaire, ajoute une stratégie claire de compatibilité.

Ne change pas le nom du groupe API sans raison. Réutilise le groupe existant si possible.

LIVRABLE FINAL
==============
Le dépôt doit devenir un monorepo modulaire avec plusieurs opérateurs/services,
une UI, des charts Helm, des tests kind, des scripts de release et une
documentation claire.

Architecture cible :

- governance-operator
- confidential-execution-operator
- trust-evidence-operator
- attestation-scheduler
- key-release-gateway
- platform-api
- platform-ui
- thesis-bench
- umbrella Helm chart
- kind test environment
- AKS private deployment values
- GPU/AKS future test plan

DÉCOUPAGE DES COMPOSANTS
========================

1. governance-operator
----------------------
Rôle :
- conserver et stabiliser l'opérateur existant ;
- gérer FinOps ;
- gérer souveraineté ;
- gérer budget ;
- gérer routing ;
- gérer quality gate ;
- gérer reports ;
- garder la compatibilité avec les CRDs existantes.

Actions :
- déplacer proprement le code existant vers cmd/governance-operator si le dépôt le permet ;
- extraire les libs communes dans pkg/ ;
- ne pas casser les manifests existants ;
- garder les tests actuels ;
- ajouter tests de non-régression.

Mise à niveau scientifique du calcul du quality gate (obligatoire) :
- remplacer les comparaisons brutes source/candidat par un test de
  NON-INFÉRIORITÉ avec marge de tolérance delta configurable dans la spec de
  l'AIQualityGate (test à deux proportions avec intervalle de confiance, ou
  approche bayésienne Beta-Binomiale : probabilité que le candidat soit pire
  que la source de plus de delta) ;
- calculer la taille d'échantillon minimale requise pour la puissance
  statistique visée : le verdict `insufficient-data` doit être fondé sur ce
  calcul, pas sur un seuil arbitraire ; exposer le nombre d'échantillons
  requis vs observés dans le status ;
- latence : utiliser p95/p99 et non la moyenne ; si la télémétrie le permet,
  normaliser par tokens de sortie ou séparer TTFT et débit inter-tokens ;
  si la télémétrie ne le permet pas, documenter la limite honnêtement,
  ne pas inventer de valeurs ;
- produire un SCORE COMPOSITE pondéré dans [0,1] (qualité golden dataset,
  taux d'erreur, latence, coût), pondérations déclarées dans la spec, EN PLUS
  des checks individuels qui restent visibles pour l'audit ;
- ajouter une HYSTÉRÉSIS : seuil d'entrée en `candidate-safe` différent du
  seuil de sortie, pour éviter le flapping des verdicts ;
- golden dataset : versionner et hasher le dataset ; inscrire version + hash
  dans le status du gate pour l'auditabilité ;
- option de test séquentiel (SPRT) pour le mode canary : arrêt anticipé
  (validation ou rollback) dès que l'évidence statistique suffit ;
  désactivé par défaut, activable dans la spec ;
- compatibilité : conserver les métriques existantes
  ai_finops_quality_gate_passed / ai_finops_quality_gate_failed_checks ;
  ajouter ai_quality_gate_score (gauge par gate) ;
- tests unitaires statistiques sur fixtures connues avec valeurs attendues
  calculées à la main ou par un outil de référence, pas approximées.

2. confidential-execution-operator
----------------------------------
Rôle :
- gérer les workloads IA sensibles ;
- appliquer ConfidentialInferencePolicy ;
- injecter RuntimeClass ;
- gérer schedulingGates ;
- préparer l'intégration DRA/GPU ;
- produire des décisions de placement attendues.

CRDs à ajouter :
- ConfidentialInferencePolicy
- AIComputeProfile
- ConfidentialWorkloadProfile si utile
- AttestationEvidence ou référence vers l'operator trust/evidence

Fonctions :
- admission webhook ;
- validation webhook ;
- defaulting webhook ;
- mutation des pods sensibles ;
- injection runtimeClassName ;
- injection schedulerName: ai-attestation-scheduler ;
- ajout de schedulingGate si les preuves sont absentes ;
- annotation avec policy hash ;
- annotation avec model digest ;
- annotation avec expected runtime ;
- annotation avec expected evidence age ;
- refus si policy invalide.

Stratégie RuntimeClass en kind (obligatoire) :
- kind/containerd ne possède PAS les handlers kata-qemu-tdx / kata-qemu-snp ;
  injecter ces runtimeClassName tels quels bloquerait les pods en Pending
  et ferait échouer les e2e pour une mauvaise raison ;
- en mode simulé (kind) : créer des objets RuntimeClass nommés
  simulated-kata-qemu-tdx / simulated-kata-qemu-snp, avec handler `runc`,
  et label ai.sovereign.io/simulated: "true" ;
- le webhook, en mode simulé, mappe la RuntimeClass exigée par la policy vers
  son équivalent simulé, et annote le pod avec la RuntimeClass réelle attendue
  ET le fait que l'exécution est simulée ;
- en mode production / aks-private : toute RuntimeClass portant le label
  simulated est INTERDITE ; le webhook refuse ;
- la simulation doit être visible partout : logs, annotations du pod, status
  des CRDs, métrique ai_simulated_runtimeclass_in_use ;
- les e2e kind doivent vérifier que le pod démarre réellement (Running) avec
  la RuntimeClass simulée.

Exemple de CRD à supporter :

apiVersion: <existing-group>/v1alpha1
kind: ConfidentialInferencePolicy
metadata:
  name: finance-confidential-llm
spec:
  target:
    namespaceSelector:
      matchLabels:
        ai.sovereign.io/sensitivity: high
    workloadSelector:
      matchLabels:
        app: risk-assistant
  requiredTEE:
    - TDX
    - SEV-SNP
  requireConfidentialContainers: true
  requireConfidentialGPU: false
  allowedRuntimeClasses:
    - kata-qemu-tdx
    - kata-qemu-snp
  maxEvidenceAgeSeconds: 300
  requireImageDigest: true
  requireModelDigest: true
  keyRelease:
    required: true
    ttlSeconds: 300
  audit:
    required: true
    level: full
  enforcementMode: enforce

Validation :
- enforcementMode doit être warn|enforce|audit.
- maxEvidenceAgeSeconds doit être > 0.
- requireConfidentialGPU=true doit exiger une section gpu.
- allowedRuntimeClasses ne doit pas être vide si requireConfidentialContainers=true.
- keyRelease.ttlSeconds doit être > 0 si keyRelease.required=true.

3. trust-evidence-operator
--------------------------
Rôle :
- gérer les preuves d'attestation ;
- gérer les révocations ;
- gérer le registre d'audit ;
- gérer l'ancrage externe de la chaîne ;
- gérer les métriques de confiance ;
- gérer les anomalies post-hoc.

CRDs à ajouter :
- AttestationEvidence
- AIKeyReleasePolicy
- AIKeyReleaseRequest
- AIPlacementDecision
- AIEvidenceRecord
- AIRevocationPolicy
- AITrustReport
- AIExperimentRun

AttestationEvidence doit contenir :
- nodeName ;
- teeType : NONE|TDX|SEV_SNP|NVIDIA_CC|SIMULATED ;
- runtimeClass ;
- gpuRequired ;
- gpuAttested ;
- gpuIdentity ;
- evidenceHash ;
- evidenceIssuer ;
- evidenceMode : real|simulated ;
- validFrom ;
- validUntil ;
- revoked ;
- revokedReason ;
- conditions.

AIEvidenceRecord doit être append-only logiquement.
Chaque entrée doit contenir :
- timestamp ;
- eventType ;
- namespace ;
- podName ;
- podUID ;
- nodeName ;
- runtimeClass ;
- gpuIdentity si applicable ;
- policyHash ;
- podSpecHash ;
- imageDigest ;
- modelDigest ;
- evidenceHash ;
- previousRecordHash ;
- recordHash ;
- signature ;
- decision ;
- reason ;
- componentName.

Event types obligatoires :
- policy_created
- pod_admitted
- attestation_verified
- node_selected
- gpu_allocated
- pod_bound
- key_release_requested
- key_released
- key_refused
- attestation_expired
- revocation_triggered
- pod_rescheduled
- audit_anomaly_detected

Le registre doit détecter :
- suppression d'entrée ;
- modification d'entrée ;
- previousRecordHash incohérent ;
- recordHash incohérent ;
- signature invalide ;
- key release avant attestation ;
- key release après révocation ;
- preuve expirée réutilisée ;
- pod déplacé sans nouvelle preuve ;
- réécriture de l'historique antérieure au dernier checkpoint ancré.

Ancrage externe de la chaîne (obligatoire) :
- l'immutabilité par webhook de validation ne résiste PAS à un cluster-admin
  ni à un accès etcd direct : le threat model doit le dire honnêtement ;
- pour borner la fenêtre de falsification, produire périodiquement un
  CHECKPOINT SIGNÉ (Ed25519) contenant : recordHash de tête, index/compteur,
  timestamp, identité du signataire ;
- exports du checkpoint : (a) CR/ConfigMap dédié dans le cluster,
  (b) fichier exportable, (c) interface AnchorBackend avec implémentations
  prêtes mais non fictives : memory et file en kind ; azure-blob / s3 /
  transparency-log en interfaces documentées "planned", sans faux appels ;
- le vérificateur de chaîne doit comparer la chaîne courante au dernier
  checkpoint ancré et détecter toute divergence ;
- métriques : ai_audit_checkpoints_total, ai_audit_last_checkpoint_age_seconds.

Volumétrie et rétention (obligatoire) :
- un CR par événement fera gonfler etcd pendant les runs thesis-bench ;
- implémenter le BATCHING : N événements par AIEvidenceRecord avec chaîne de
  hash interne au batch, la chaîne globale reliant les batches ;
- rétention configurable : purge des anciens segments UNIQUEMENT après
  checkpoint ancré couvrant ces segments ; la vérifiabilité doit être
  conservée à travers les segments via les checkpoints ;
- documenter l'impact etcd et les limites dans docs/security/audit-evidence-registry.md ;
- métrique : ai_evidence_records_pruned_total ;
- un run thesis-bench long ne doit pas dégrader le control plane kind.

4. attestation-scheduler
------------------------
Rôle :
- implémenter un vrai scheduler plugin Kubernetes, pas seulement un contrôleur.
- utiliser schedulerName: ai-attestation-scheduler.
- utiliser les points appropriés du Kubernetes Scheduling Framework si la version du projet le permet.

Contraintes d'implémentation (obligatoire) :
- suivre le pattern out-of-tree de sigs.k8s.io/scheduler-plugins (scaffold,
  imports, déploiement en second scheduler) ; ne pas toucher au scheduler
  par défaut du cluster ;
- ÉPINGLER la version Kubernetes : cohérence stricte entre go.mod, la version
  du Scheduling Framework, l'image de nœud kind et la CI ; documenter cette
  matrice dans docs/architecture/version-matrix.md ;
- viser Kubernetes >= 1.34 (DRA structured parameters GA) afin que la
  préparation Confidential GPU via DRA soit réelle et non décorative ;
  si le dépôt est sur une version plus ancienne, proposer la mise à jour dans
  le plan de la phase au lieu de bricoler des APIs incompatibles ;
- si l'API exacte du Scheduling Framework diffère selon la version utilisée,
  adapte proprement le code à la version du projet.

Fonctions minimales :
- PreFilter : charger ConfidentialInferencePolicy liée au pod ;
- Filter : rejeter les nœuds sans AttestationEvidence conforme ;
- Score : scorer selon confiance, fraîcheur, coût, disponibilité ;
- Reserve : créer/réserver une AIPlacementDecision ;
- Permit : attendre si l'attestation n'est pas encore fraîche ;
- PreBind : revérifier la conformité juste avant binding ;
- générer un placement token vérifiable après décision.

Ne remplace pas le scheduler plugin par un simple nodeSelector.
Ne remplace pas le scheduler plugin par un simple controller de labels.
Ne fais pas semblant que le plugin existe si ce n'est pas vrai.

AIPlacementDecision doit contenir :
- podUID ;
- podSpecHash ;
- namespace ;
- podName ;
- selectedNode ;
- runtimeClass ;
- policyHash ;
- evidenceHash ;
- imageDigest ;
- modelDigest ;
- gpuIdentity si applicable ;
- placementToken ;
- tokenSignature ;
- validUntil ;
- decisionReason ;
- schedulerVersion.

Le placement token doit lier :
- pod_uid ;
- pod_spec_hash ;
- image_digest ;
- model_digest ;
- node_identity ;
- gpu_identity ;
- runtimeClass ;
- evidence_hash ;
- policy_hash ;
- key_identifier si connu ;
- TTL.

Utiliser une signature Ed25519 ou une autre signature standard de la librairie Go.
Ne pas hardcoder les clés privées.
En kind, générer une clé dans un Secret Kubernetes dédié.
En production, prévoir l'intégration KMS/Vault.

5. key-release-gateway
----------------------
Rôle :
- service interne HTTP/gRPC pour demander une clé ;
- vérifier placement token ;
- vérifier AttestationEvidence ;
- vérifier ConfidentialInferencePolicy ;
- vérifier AIRevocationPolicy ;
- vérifier pod UID ;
- vérifier pod spec hash ;
- vérifier image digest ;
- vérifier model digest ;
- vérifier runtimeClass ;
- vérifier evidence freshness ;
- vérifier TTL ;
- refuser en cas de non-conformité.

Endpoints minimaux :
- POST /v1/key-release
- GET /healthz
- GET /readyz
- GET /metrics

Request minimale :
{
  "namespace": "...",
  "podName": "...",
  "podUID": "...",
  "keyID": "...",
  "modelDigest": "...",
  "imageDigest": "...",
  "placementToken": "...",
  "evidenceHash": "..."
}

Response allowed :
{
  "allowed": true,
  "keyMaterialRef": "...",
  "ttlSeconds": 300,
  "evidenceRecord": "..."
}

Response denied :
{
  "allowed": false,
  "reason": "EVIDENCE_EXPIRED"
}

Modes de backend :
- memory : kind uniquement ;
- kubernetes-secret : dev uniquement ;
- vault : interface prête ;
- trustee : interface prête ;
- azure-key-vault : interface prête, sans secrets hardcodés.

Aucun secret réel dans Git.
Aucune clé privée dans Git.
Aucune clé de test réutilisable en production.

6. platform-api
---------------
Rôle :
- backend de l'UI ;
- ne jamais donner un kubeconfig au navigateur ;
- lire/écrire les CRDs via Kubernetes API ;
- appliquer RBAC ;
- exposer une API simple pour la UI.

Endpoints :
- GET /api/overview
- GET /api/workloads
- GET /api/policies
- POST /api/policies/preview
- POST /api/policies/apply
- GET /api/attestations
- GET /api/key-releases
- GET /api/revocations
- GET /api/audit
- GET /api/experiments
- POST /api/experiments/run
- GET /api/reports/trust
- GET /healthz
- GET /readyz
- GET /metrics

Sécurité :
- pas de mode admin par défaut ;
- auth désactivée seulement en dev local avec flag explicite dev.insecureAuth=true ;
- en AKS privé, prévoir OIDC/Azure Entra ID configuration via Helm values ;
- NetworkPolicy obligatoire ;
- ServiceAccount minimal ;
- RBAC minimal.

7. platform-ui
--------------
Créer une UI web privée.

Stack recommandée :
- React + TypeScript + Vite ;
- composants simples ;
- pas besoin de design parfait ;
- priorité à l'utilité scientifique et produit.

Pages obligatoires :
1. Overview
   - workloads confidentiels ;
   - workloads non conformes ;
   - nœuds attestés ;
   - preuves expirées ;
   - key releases allowed/denied ;
   - révocations ;
   - trust score global.

2. Policy Wizard
   - créer ConfidentialInferencePolicy sans écrire YAML ;
   - choisir namespace/application ;
   - choisir sensibilité ;
   - Confidential Containers required ;
   - RuntimeClass ;
   - TDX/SEV-SNP/NVIDIA_CC ;
   - Confidential GPU required ou non ;
   - max evidence age ;
   - key release TTL ;
   - audit level ;
   - warn/enforce.

3. YAML Preview
   - afficher le YAML généré ;
   - bouton Apply ;
   - option future GitOps : "Create PR" mais ne pas l'implémenter faussement si non prêt.

4. Workloads
   - pod ;
   - namespace ;
   - policy ;
   - node ;
   - runtimeClass (avec indication claire si simulée) ;
   - evidence status ;
   - key release status ;
   - trust score ;
   - anomalies.

5. Attestation Evidence
   - node ;
   - TEE ;
   - runtime ;
   - GPU ;
   - evidence age ;
   - validUntil ;
   - revoked ;
   - evidenceMode real/simulated visible.

6. Key Release
   - demandes ;
   - allowed/denied ;
   - reason ;
   - keyID ;
   - TTL ;
   - evidenceHash.

7. Audit Timeline
   - liste chronologique des AIEvidenceRecord ;
   - chain hash status ;
   - statut du dernier checkpoint ancré ;
   - bouton "Verify Chain" ;
   - anomalies détectées.

8. Experiments
   - lancer les baselines ;
   - lancer les scénarios d'attaque ;
   - voir les résultats ;
   - exporter JSON/CSV/Markdown.

9. Reports
   - rapport TrustOps ;
   - rapport thèse ;
   - rapport startup/compliance.

La UI doit être privée dans AKS :
- Service type ClusterIP par défaut ;
- Ingress internal uniquement ;
- pas de LoadBalancer public ;
- annotations Azure internal load balancer si service interne ;
- valeurs Helm pour private AKS ;
- accès local via port-forward en kind.

8. thesis-bench
---------------
Créer un outil ou contrôleur d'expérimentation pour ma thèse.

But :
- exécuter les baselines ;
- exécuter les attaques ;
- collecter les métriques ;
- produire des résultats reproductibles.

Baselines à supporter :
B1 Kubernetes standard
B2 labels/nodeSelector
B3 RuntimeClass only
B4 Confidential Containers/Trustee sans scheduler spécialisé
B5 DRA sans attestation
B6 solution complète : scheduling + key release + révocation + audit

Dans kind, B4/B5 peuvent être partiellement simulées, mais cela doit être marqué "simulated".
Ne jamais afficher "real GPU confidential passed" dans kind.

Scénarios d'attaque à implémenter en e2e :
1. faux label confidential=true sur un nœud ;
2. pod IA sensible sans RuntimeClass confidentielle ;
3. preuve d'attestation expirée ;
4. replay d'ancienne preuve ;
5. key release depuis mauvais podUID ;
6. modelDigest différent ;
7. imageDigest différent ;
8. policy modifiée après admission avant key release ;
9. node révoqué après placement ;
10. pod reschedulé sans nouvelle preuve ;
11. tentative de modification d'un AIEvidenceRecord ;
12. demande de key release après expiration ;
13. GPU non confidentiel demandé alors que policy exige GPU confidentiel :
    à tester en simulation kind seulement, et réel plus tard en AKS ;
14. tentative de réécriture de l'historique d'audit antérieure au dernier
    checkpoint ancré : doit être détectée par le vérificateur.

Métriques de sortie :
- placement_non_compliant_blocked_total ;
- key_release_denied_total ;
- evidence_replay_blocked_total ;
- fake_label_attack_blocked_total ;
- expired_evidence_blocked_total ;
- revocation_delay_seconds ;
- audit_chain_valid ;
- audit_anomalies_detected_total ;
- scheduling_latency_seconds ;
- admission_to_binding_seconds ;
- attestation_to_key_release_seconds ;
- key_release_latency_seconds ;
- trust_score ;
- cost_per_secure_inference si l'opérateur FinOps le permet.

Sorties :
- results/thesis/latest/results.json
- results/thesis/latest/results.csv
- results/thesis/latest/report.md

Ne pas fabriquer de métriques.
Toutes les métriques doivent venir :
- des CRDs ;
- de Prometheus ;
- des events Kubernetes ;
- des logs des composants ;
- des timestamps réels.

LAYOUT CIBLE DU DÉPÔT
=====================

Proposer et appliquer un layout propre, par exemple :

/api
  /v1alpha1
/cmd
  /governance-operator
  /confidential-execution-operator
  /trust-evidence-operator
  /attestation-scheduler
  /key-release-gateway
  /platform-api
  /thesis-bench
/pkg
  /policy
  /attestation
  /evidence
  /scheduler
  /keyrelease
  /revocation
  /audit
  /anchoring
  /qualitystats
  /metrics
  /crypto
  /k8sutils
/operators
  optionnel si le projet préfère séparer par module
/ui
  /platform-ui
/charts
  /ai-confidential-governance-platform
  /governance-operator
  /confidential-execution-operator
  /trust-evidence-operator
  /attestation-scheduler
  /key-release-gateway
  /platform-api
  /platform-ui
/deploy
  /kind
  /aks-private
  /examples
/experiments
  /baselines
  /attacks
  /reports
/test
  /unit
  /envtest
  /e2e
/docs
  /architecture
  /thesis
  /startup
  /security
  /aks
  /kind

Si le dépôt existant a déjà une structure différente, fais une migration minimale et propre.
Ne déplace pas tout sans raison.
Garde l'historique logique.

HELM CHARTS
===========
Créer un umbrella chart :

charts/ai-confidential-governance-platform

Il doit permettre :

global:
  registry: ""
  imagePullSecrets: []
  environment: kind|aks-private|production
  mode: dev|thesis|startup
  simulatedEvidence:
    enabled: false
  runtimeClassSimulation:
    enabled: false
  audit:
    anchoring:
      mode: memory|file|azure-blob|s3
  gpu:
    enabled: false
    mode: disabled|simulated|nvidia-cc

modules:
  governanceOperator:
    enabled: true
  confidentialExecutionOperator:
    enabled: true
  trustEvidenceOperator:
    enabled: true
  attestationScheduler:
    enabled: true
  keyReleaseGateway:
    enabled: true
  platformApi:
    enabled: true
  platformUi:
    enabled: true
  thesisBench:
    enabled: true

kind values :
- simulatedEvidence.enabled=true ;
- runtimeClassSimulation.enabled=true (RuntimeClass simulated-* backées par runc) ;
- audit.anchoring.mode=file (ou memory) ;
- gpu.mode=simulated uniquement si tests GPU simulés explicitement ;
- UI ClusterIP ;
- accès par port-forward ;
- aucune dépendance cloud.

aks-private values :
- simulatedEvidence.enabled=false ;
- runtimeClassSimulation.enabled=false ;
- audit.anchoring.mode=azure-blob (placeholders, pas de secrets) ;
- UI private only ;
- Service ClusterIP ;
- internal ingress ;
- NetworkPolicies enabled ;
- OIDC ready ;
- Azure Key Vault placeholders ;
- ACR image registry ;
- GPU mode disabled par défaut jusqu'aux tests réels.

production values :
- simulatedEvidence.enabled=false ;
- runtimeClassSimulation.enabled=false ;
- dev.insecureAuth=false ;
- metrics TLS si possible ;
- NetworkPolicies enabled ;
- PodSecurityContext strict ;
- readOnlyRootFilesystem ;
- runAsNonRoot ;
- seccomp RuntimeDefault ;
- no public UI by default.

AKS PRIVÉ
=========
Ajouter deploy/aks-private avec :

1. values-aks-private.yaml
2. README-AKS-private.md
3. optionnel infra/azure/terraform minimal si le dépôt le permet

Exigences :
- UI non publique ;
- platform-api non publique ;
- ingress interne uniquement ;
- pas de LoadBalancer public ;
- NetworkPolicy pour limiter accès à platform-api ;
- readiness/liveness probes ;
- ServiceAccount dédié ;
- RBAC minimal ;
- secrets externes via Key Vault ou placeholders, jamais secrets dans Git ;
- documentation claire pour connecter depuis réseau privé, bastion, VPN ou port-forward via jumpbox.

GPU
===
La partie GPU réel sera testée ultérieurement avec AKS.

À faire maintenant :
- concevoir les champs CRD ;
- ajouter les templates Helm ;
- ajouter les validations ;
- ajouter les métriques ;
- ajouter les tests simulés kind clairement marqués ;
- ajouter docs "GPU real validation pending AKS" ;
- ajouter values-aks-gpu.yaml mais ne pas prétendre que c'est validé.

Ne pas faire :
- ne pas déclarer NVIDIA Confidential GPU validé dans kind ;
- ne pas utiliser de faux GPU en production ;
- ne pas mélanger GPU simulated et real ;
- ne pas cacher la simulation.

TESTS OBLIGATOIRES
==================

Unit tests :
- policy matching ;
- policy validation ;
- evidence freshness ;
- evidence revocation ;
- placement token generation ;
- placement token verification ;
- key release allow/deny ;
- audit chain hash ;
- tamper detection ;
- checkpoint anchoring : génération, signature, vérification, divergence détectée ;
- batching : chaîne de hash valide à travers plusieurs segments/batches ;
- rétention : purge refusée sans checkpoint couvrant ;
- quality gate : test de non-infériorité sur fixtures connues (verdicts attendus) ;
- quality gate : calcul de taille d'échantillon minimale ;
- quality gate : score composite et pondérations ;
- quality gate : hystérésis (pas de flapping autour du seuil) ;
- mapping RuntimeClass simulée (mode simulé) et refus (mode production) ;
- trust score calculation.

Envtest :
- CRD validation ;
- admission webhook ;
- mutation webhook ;
- ConfidentialInferencePolicy creation ;
- AttestationEvidence lifecycle ;
- AIRevocationPolicy effects ;
- AIEvidenceRecord immutability if implemented via webhook.

Scheduler tests :
- pod without policy uses default scheduler or is ignored ;
- sensitive pod without evidence is not scheduled ;
- sensitive pod with valid evidence is schedulable ;
- expired evidence rejected ;
- revoked node rejected ;
- fake label alone rejected ;
- score prefers freshest evidence.

Key release tests :
- valid placement token -> allowed ;
- wrong podUID -> denied ;
- wrong modelDigest -> denied ;
- expired evidence -> denied ;
- revoked node -> denied ;
- expired token -> denied ;
- missing audit requirement -> denied or warn according to policy.

E2E kind tests :
- create kind cluster ;
- build images ;
- load images into kind ;
- install chart ;
- wait for all controllers ready ;
- create simulated AttestationEvidence ;
- create ConfidentialInferencePolicy ;
- deploy sensitive workload ;
- verify RuntimeClass mutation (RuntimeClass simulée injectée, pod Running) ;
- verify schedulerName mutation ;
- verify schedulingGate behavior ;
- verify placement decision ;
- request key release ;
- verify allowed ;
- revoke node ;
- request key release again ;
- verify denied ;
- verify audit entries ;
- run audit chain verifier ;
- verify checkpoint créé et vérifiable ;
- tamper audit record ;
- verify anomaly detected ;
- tamper antérieur au checkpoint -> verify anomaly detected ;
- run thesis-bench attack suite ;
- generate report.

Helm tests :
- helm lint all charts ;
- helm template kind values ;
- helm template aks-private values ;
- kubeconform or equivalent validation ;
- verify no public LoadBalancer in aks-private values ;
- verify simulatedEvidence=false in aks-private values ;
- verify runtimeClassSimulation=false in aks-private values.

Security checks :
- go test ./... ;
- go vet ./... ;
- govulncheck ./... if available ;
- gosec ./... if available ;
- trivy fs . if available ;
- helm lint ;
- kubeconform ;
- npm/yarn/pnpm audit for UI if applicable ;
- no secrets committed.

Si un outil n'est pas installé, le script doit afficher clairement :
"tool missing: <tool>. Install it or run make bootstrap-tools."
Ne pas ignorer silencieusement.

MAKE TARGETS
============
Créer ou mettre à jour :

make bootstrap-tools
make generate
make manifests
make test
make test-unit
make test-envtest
make test-scheduler
make test-e2e-kind
make lint
make security-scan
make kind-up
make kind-down
make kind-load
make deploy-kind
make undeploy-kind
make build-images
make push-images
make helm-lint
make helm-template-kind
make helm-template-aks-private
make helm-package
make helm-push
make thesis-bench
make ci-local
make release

Variables :
REGISTRY
VERSION
IMAGE_TAG
CHART_VERSION
PUSH
HELM_REGISTRY

Comportement release :
- make release doit refuser de pousser si tests échouent ;
- make release doit refuser de pousser si REGISTRY vide ;
- make release doit refuser de pousser si VERSION vide ;
- make release doit construire toutes les images ;
- make release doit lancer ci-local ;
- make release doit tagger les images ;
- make release doit pousser seulement si PUSH=true ;
- make release doit packager les charts ;
- make release doit pousser les charts OCI seulement si HELM_REGISTRY défini ;
- mettre à jour Chart.yaml appVersion et values image tags.

Rappel : dans le cadre de ce prompt, Codex ne pousse rien (ni git, ni images,
ni charts). Les cibles push existent pour l'opérateur humain.

IMAGES À PRODUIRE
=================
- governance-operator
- confidential-execution-operator
- trust-evidence-operator
- attestation-scheduler
- key-release-gateway
- platform-api
- platform-ui
- thesis-bench

Chaque image :
- Dockerfile dédié ou Dockerfile multi-stage ;
- non-root ;
- minimal base image ;
- labels OCI ;
- version injectée ;
- commit SHA injecté ;
- build date injectée ;
- health endpoint si service ;
- metrics endpoint si service.

MÉTRIQUES PROMETHEUS
====================
Ajouter au minimum :

ai_confidential_policies_total
ai_confidential_workloads_total
ai_confidential_workloads_compliant_total
ai_confidential_workloads_noncompliant_total
ai_attestation_evidence_total
ai_attestation_evidence_valid
ai_attestation_evidence_age_seconds
ai_attestation_evidence_expired_total
ai_attestation_evidence_revoked_total
ai_scheduler_decisions_total
ai_scheduler_rejections_total
ai_scheduler_decision_latency_seconds
ai_placement_tokens_issued_total
ai_key_release_requests_total
ai_key_release_allowed_total
ai_key_release_denied_total
ai_key_release_latency_seconds
ai_revocations_total
ai_revocation_delay_seconds
ai_evidence_records_total
ai_evidence_records_pruned_total
ai_audit_chain_valid
ai_audit_anomalies_total
ai_audit_checkpoints_total
ai_audit_last_checkpoint_age_seconds
ai_trust_score
ai_quality_gate_score
ai_simulated_evidence_in_use
ai_simulated_runtimeclass_in_use

Important :
- ai_simulated_evidence_in_use doit être 1 si mode simulated actif.
- ai_simulated_runtimeclass_in_use doit être 1 si des RuntimeClass simulées sont utilisées.
- en production/aks-private, ces deux métriques doivent rester 0.

INDICATEURS QUE LE TRAVAIL EST BIEN FAIT
========================================
Codex doit vérifier et afficher les preuves suivantes dans son résumé final
(pour les phases effectivement exécutées).

1. Commandes réussies :
- make ci-local
- make test
- make test-e2e-kind
- make helm-lint
- make helm-template-kind
- make helm-template-aks-private

2. Cluster kind :
- tous les pods de la plateforme sont Running/Ready ;
- les CRDs sont installées ;
- les webhooks fonctionnent ;
- l'UI est accessible uniquement via port-forward ;
- aucune UI publique n'est exposée.

3. Scénario positif :
- ConfidentialInferencePolicy créée ;
- AttestationEvidence simulée valide créée ;
- workload sensible admis ;
- RuntimeClass simulée injectée et pod réellement Running ;
- schedulerName injecté ;
- pod placé sur nœud avec evidence valide ;
- AIPlacementDecision créée ;
- placement token généré ;
- key release autorisée ;
- AIEvidenceRecord contient pod_admitted, attestation_verified, node_selected, pod_bound, key_released ;
- audit chain valide ;
- checkpoint d'ancrage produit et vérifié.

4. Scénarios négatifs :
- faux label confidential=true ne suffit pas ;
- evidence expirée bloque placement ou key release ;
- evidence révoquée bloque key release ;
- mauvais podUID bloque key release ;
- mauvais modelDigest bloque key release ;
- key release après révocation bloquée ;
- audit record modifié détecté ;
- falsification antérieure au checkpoint détectée ;
- RuntimeClass simulée refusée en mode production ;
- GPU confidential exigé mais non réel dans kind -> refus ou simulation explicitement marquée selon mode.

5. Quality gate :
- verdict candidate-safe/candidate-risk fondé sur test de non-infériorité ;
- insufficient-data justifié par la taille d'échantillon requise ;
- score composite exposé (status + métrique) ;
- hystérésis vérifiée par test.

6. UI :
- créer policy via wizard ;
- voir YAML preview ;
- appliquer policy ;
- voir workloads ;
- voir attestation evidence (real/simulated visible) ;
- voir key releases ;
- voir audit timeline avec statut checkpoint ;
- vérifier audit chain ;
- exporter rapport.

7. Helm AKS privé :
- helm template aks-private ne contient pas de LoadBalancer public ;
- UI et API sont ClusterIP/internal ingress ;
- NetworkPolicies activées ;
- simulatedEvidence=false ;
- runtimeClassSimulation=false ;
- gpu.enabled=false par défaut ;
- aucun secret hardcodé.

8. Release :
- images construites localement ;
- images poussées seulement si PUSH=true et REGISTRY défini (par l'humain) ;
- charts packagés ;
- charts poussés seulement si HELM_REGISTRY défini (par l'humain) ;
- tags cohérents entre values.yaml, Chart.yaml et images.

DOCUMENTATION À AJOUTER
=======================
Créer/mettre à jour :

docs/architecture/platform.md
docs/architecture/operators-split.md
docs/architecture/version-matrix.md
docs/thesis/experimental-design.md
docs/thesis/baselines.md
docs/thesis/attack-scenarios.md
docs/thesis/metrics.md
docs/thesis/results-format.md
docs/thesis/quality-gate-statistics.md
docs/security/threat-model.md
docs/security/key-release-protocol.md
docs/security/audit-evidence-registry.md
docs/security/audit-anchoring.md
docs/security/revocation.md
docs/kind/local-testing.md
docs/kind/runtimeclass-simulation.md
docs/aks/private-deployment.md
docs/aks/gpu-future-validation.md
docs/startup/product-positioning.md
docs/startup/mvp.md
docs/ui/user-guide.md
docs/release/images-and-charts.md

Chaque doc doit être claire, pratique et vérifiable.
Ne pas écrire de promesses non implémentées.
Pour les parties futures, écrire clairement "planned" ou "future AKS GPU validation".
Le threat model doit dire honnêtement ce que le système NE protège PAS
(cluster-admin, accès etcd, side channels non couverts, GPU non validé).

DÉFINITION DE DONE
==================
Le travail est terminé seulement si :

1. les opérateurs sont découplés proprement ou un plan de migration partiel est documenté avec les parties réellement faites ;
2. les CRDs principales existent et sont testées ;
3. l'admission controller fonctionne ;
4. RuntimeClass/schedulerName/schedulingGate sont gérés ;
5. le scheduler attestation-aware fonctionne au moins dans kind avec evidence simulée ;
6. le key-release-gateway refuse/autorise correctement selon policy/evidence/token ;
7. la révocation bloque les nouvelles key releases ;
8. l'audit registry produit une chaîne vérifiable ;
9. la détection d'anomalie fonctionne ;
10. thesis-bench produit des résultats JSON/CSV/Markdown ;
11. l'UI permet de gérer policies, workloads, evidence, key releases, audit et experiments ;
12. le chart kind fonctionne ;
13. le chart aks-private rend une configuration privée ;
14. les images et charts peuvent être poussés avec les variables de release ;
15. aucun test n'est vide ou contourné ;
16. aucun secret n'est commité ;
17. la partie GPU réelle est clairement marquée future AKS validation ;
18. le quality gate produit des verdicts fondés statistiquement (non-infériorité,
    taille d'échantillon, score composite, hystérésis) avec tests unitaires ;
19. la RuntimeClass simulée fonctionne en kind (pods Running) et est refusée
    en mode production ;
20. les checkpoints d'ancrage sont produits, vérifiés, et une falsification
    antérieure au dernier checkpoint est détectée ;
21. la volumétrie du registre est maîtrisée (batching + rétention) sans casser
    la vérifiabilité de la chaîne ;
22. rien n'a été poussé vers un remote par Codex.

ORDRE DE TRAVAIL RECOMMANDÉ
===========================
Rappel : une session = uniquement les phases explicitement demandées.

Phase 0 — Audit du dépôt
- lire README, Makefile, charts, config, api, controllers, internal ;
- identifier la version Kubernetes effective et remplir docs/architecture/version-matrix.md ;
- produire un court plan dans docs/implementation-plan.md ;
- ne pas commencer par casser le layout.

Phase 1 — Foundation
- créer packages communs (dont pkg/crypto, pkg/qualitystats) ;
- créer CRDs ConfidentialInferencePolicy, AttestationEvidence, AIPlacementDecision, AIKeyReleasePolicy, AIEvidenceRecord, AIRevocationPolicy ;
- générer manifests ;
- ajouter validations ;
- implémenter la lib statistique du quality gate (non-infériorité, taille
  d'échantillon, score composite, hystérésis) avec tests unitaires ;
- ajouter tests unitaires.

Phase 2 — Operators split
- isoler governance-operator ;
- brancher le quality gate existant sur pkg/qualitystats sans casser les CRDs ;
- ajouter confidential-execution-operator ;
- ajouter trust-evidence-operator ;
- garder compatibilité.

Phase 3 — Admission + RuntimeClass
- webhook mutate/validate ;
- injection RuntimeClass ;
- stratégie RuntimeClass simulée kind (création des RuntimeClass simulated-*,
  mapping, refus en production) ;
- injection schedulerName ;
- schedulingGates ;
- tests envtest/e2e.

Phase 4 — Attestation evidence
- evidence controller ;
- simulated provider kind ;
- validation freshness/revocation ;
- metrics.

Phase 5 — Scheduler plugin
- attestation-scheduler (pattern scheduler-plugins, versions épinglées) ;
- Filter/Score/Reserve/Permit/PreBind si possible ;
- AIPlacementDecision ;
- placement token ;
- tests.

Phase 6 — Key release
- key-release-gateway ;
- verification token/evidence/policy ;
- allow/deny ;
- audit entries ;
- tests.

Phase 7 — Revocation + audit
- AIRevocationPolicy ;
- block new key releases ;
- TTL ;
- audit registry chain (batching + rétention) ;
- checkpoints d'ancrage signés + vérificateur ;
- anomaly detector.

Phase 8 — Thesis bench
- baselines ;
- attacks (dont scénario 14 anti-réécriture) ;
- metrics ;
- reports.

Phase 9 — UI + platform API
- backend ;
- frontend ;
- wizard ;
- dashboards ;
- audit timeline avec checkpoints ;
- experiments ;
- private mode.

Phase 10 — Helm/release/AKS
- umbrella chart ;
- kind values ;
- aks-private values ;
- release scripts ;
- docs.

RÉSUMÉ FINAL ATTENDU DE CODEX
=============================
À la fin de chaque session, donne un résumé structuré :

1. phases exécutées dans cette session ;
2. fichiers modifiés ;
3. composants ajoutés ;
4. CRDs ajoutées ;
5. commandes exécutées ;
6. résultats des tests (réels, avec sorties) ;
7. ce qui marche dans kind ;
8. ce qui est prévu pour AKS privé ;
9. ce qui est prévu pour GPU réel plus tard ;
10. tout élément simulé actuellement actif (evidence, RuntimeClass, GPU) ;
11. limitations honnêtes ;
12. ce qui reste pour les phases suivantes ;
13. commandes exactes pour reconstruire, tester, installer et pousser
    (le push restant réservé à l'opérateur humain).

Ne pas écrire "tout est terminé" si ce n'est pas vrai.
Écrire exactement ce qui a été fait et ce qui reste.
