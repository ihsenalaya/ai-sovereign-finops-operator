# Mémoire — Décomposition en 3 opérateurs indépendants

> Fichier de suivi mis à jour régulièrement (~10 min) pendant le chantier.
> Dernière mise à jour : 2026-07-16 ~15h20 (démarrage)

## Objectif

Décomposer l'opérateur géant (19 CRDs / 18 controllers / 1 image) en **3 opérateurs
indépendants**, chacun installable et opérable seul :

| Opérateur | Spécialité | CRDs possédés |
|---|---|---|
| `ai-finops-operator` | FinOps & souveraineté du trafic IA | AIGateway, AIProvider, AIModel, AIBudgetPolicy, AISovereigntyPolicy, AIBreakEvenAnalysis, AIFinOpsReport, AIQualityGate, AIRoutingPolicy, AIRouteOverride, AIChangeRequest (11) |
| `ai-confidential-operator` | Attestation TEE & placement vérifiable | ConfidentialInferencePolicy, RawAttestationReport, AttestationEvidence, AIPlacementDecision, AIKeyReleasePolicy, AIRevocationPolicy, AIEvidenceRecord (7) |
| `ai-govar-operator` | Admission gouvernée GOV-AR | AIWorkloadBinding (1) + lit les CRDs catalogue FinOps (copie dans crds/ du chart, Helm saute les CRDs déjà présentes) |

## Décisions d'architecture

- **Groupe API conservé** : `aiops.imperium.io/v1alpha1` pour les 3 (aucune migration de données).
- **Code Go** : module commun existant `operateur/` (pas de fork de code) ; chaque opérateur a
  son **manager dédié** (`operateur/cmd/<op>-manager/main.go`) n'enregistrant que ses controllers,
  sa **propre image** et son **propre chart** avec CRDs + RBAC scopés.
- **Indépendance runtime** : 3 charts disjoints ; le chart govar embarque les CRDs catalogue
  qu'il lit (AIModel/AIProvider/AIRoutingPolicy/AIChangeRequest/AIBudgetPolicy) — Helm ignore
  les CRDs déjà installées → coexistence sans conflit.
- **Interdits** : ne pas toucher aux dossiers `article*` et `experimentation*`.
- **Livrables par opérateur** (dossier `operators/<nom>/`) :
  - `README.md` détaillé (installation + utilisation)
  - `docs/` : un fichier par CRD
  - `chart/` : Helm chart indépendant
  - `automatisation/` : kind + dépendances + apps de test + dashboard Grafana adapté

## Images (registry ghcr.io/ihsenalaya/ai-sovereign-finops-operator/)

- `finops-operator` (nouveau manager FinOps)
- `confidential-operator` (nouveau manager confidential) + images de service existantes
  (attestation-scheduler, central-verifier, node-attestation-agent, key-release-gateway)
- `govar-operator` (nouveau manager govar) + gov-ar-admission, gov-ar-calibration-producer (déjà poussées)

## Cartographie (faite)

- **Webhook mutating/validating** (`internal/webhook/bootstrap`) crée 2 règles : `pods`
  (injection sidecar → confidential) et `aichangerequests` (tampon identité reviewer GOV-AR
  → govar). Décision : ajouter une option `Scope` au bootstrap (Pods / ChangeRequests / All,
  défaut All = comportement monolithe inchangé).
- **Répartition controllers** : finops-manager = 11 (AIGateway, AIProvider, AIModel,
  AIBudgetPolicy, AISovereigntyPolicy, AIBreakEvenAnalysis, AIFinOpsReport, AIQualityGate,
  AIRouteOverride, AIRoutingPolicy, AIChangeRequest) + sous-commande quality-eval ;
  confidential-manager = 6 (AttestationEvidence*, AIRevocationPolicy, RawAttestationReport*,
  AIEvidenceRecord, AIPlacementDecision, AIKeyReleasePolicy ; * = env-gated comme aujourd'hui)
  + webhooks podinjector + runtimeclasses simulées ; govar-manager = AIWorkloadBinding +
  webhooks changeapproval (chemins dédiés) ; services gov-ar-admission /
  gov-ar-calibration-producer inchangés (déjà des binaires/images séparés).
- **Docs CRD existantes** (operateur/docs/crds/) : 9 réutilisables (aiprovider, aimodel,
  aigateway, aibudgetpolicy, aisovereigntypolicy, aibreakevenanalysis, aifinopsreport,
  aiqualitygate, aiworkloadbinding + govar-safety-fields). À écrire : airoutingpolicy,
  airouteoverride, aichangerequest + les 7 confidential.
- **Dashboard existant** : operateur/dashboards/ai-finops-overview.json (source à adapter ×3).
- **Métriques** : famille `ai_finops_*` (+ `ai_quality_gate_score`,
  `ai_simulated_runtimeclass_in_use`) ; famille `govar_*` (documentée dans
  docs/gov-ar-operations.md). Confidential : `ai_simulated_runtimeclass_in_use` + events.
- **seed-usage** : génère de la télémétrie réelle (nécessite clés API) → pour les apps de test
  kind, préférer des ConfigMaps d'usage statiques (collector `configmap`).
- **Scripts kind réutilisables** : automatisation/scripts/01-create-cluster.sh, 02-build-load-image.sh…

## Avancement

- [x] Fichier mémoire créé
- [x] Cartographie des dépendances controllers/paquets
- [x] 3 managers Go + bootstrap scopable (`Scope`: Pods/ChangeRequests/All) + build + tests webhook OK
- [x] 3 Dockerfiles (`operateur/Dockerfile.{finops,confidential,govar}-operator`) + images
      construites ET poussées sur ghcr (tags 0.1.0 + latest) :
      finops-operator sha256:5f5f6d1f…, confidential-operator sha256:f93fb659…,
      govar-operator sha256:6f2d40ef… (~60-70 MB chacune)
- [x] 3 Helm charts (helm lint + render OK) :
      operators/ai-finops-operator/chart (11 CRDs, RBAC scopé, pas de webhook) ;
      operators/ai-confidential-operator/chart (7 CRDs, webhooks pods, runtimeclasses) ;
      operators/ai-govar-operator/chart (aiworkloadbindings + 5 CRDs catalogue en copie,
      templates gov-ar-admission/calibration copiés, digests gov-ar épinglés,
      garde-fous fail-closed conservés : softwareSHA256, identity.masterExistingSecret,
      postgres.existingSecret requis quand admission activée)
- [x] dashboards Grafana ×3 générés depuis operateur/dashboards/ai-finops-overview.json :
      finops-overview.json (23 panels : split FinOps + rangée Optimisation ajoutée),
      confidential-overview.json (13 panels : split confidential + runtimeclass simulée),
      govar-overview.json (15 panels : décisions/deny ratio/p95-p99, ledger, réconciliation)
- [x] automatisation kind finops : common.sh / up.sh / down.sh (kind + kube-prometheus-stack
      + chart + test-apps + import dashboard via ConfigMap grafana_dashboard=1)
- [x] apps de test finops : 01-catalog (2 providers EU/US + 3 modèles), 02-gateway-usage
      (AIGateway telemetry configmap + greenops-usage usage.json statique 3 apps),
      03-policies-report (budget/souveraineté/breakeven/report) — **validées contre les
      schémas OpenAPI des CRDs (0 erreur)**
- [x] automatisation kind confidential : common.sh / up.sh / down.sh (mêmes étapes)
- [x] apps de test confidential : 01-policy (CIP warn), 02-evidence (RawAttestationReport
      provider=simulator + AttestationEvidence SEV-SNP simulée + AIEvidenceRecord),
      03-keyrelease-revocation (+ pod démo runtimeClass simulated-kata-qemu-snp) — validées
- [x] automatisation kind govar : common.sh / up.sh / down.sh (secret identité openssl,
      digest admission épinglé, devInMemory explicite, smoke test Job /healthz + /readyz +
      métriques govar_*) ; test-apps validées contre les schémas CRD (0 erreur)
- [x] 3 README détaillés (installation + utilisation + intégrations + avertissement
      monolithe) + operators/README.md (principes, matrice d'indépendance)
- [x] docs/ par CRD : 8 copies finops + 3 écrites (airoutingpolicy, airouteoverride,
      aichangerequest) ; 7 écrites confidential ; 4 copies govar (aiworkloadbinding,
      govar-safety-fields, gov-ar-operations, gov-ar-approval-migration)
- [x] workflow release : 3 images finops/confidential/govar-operator ajoutées
- [x] pointeur "Décomposition en 3 opérateurs" dans le README racine
- [x] validation finale : go build OK, helm lint 3/3 OK, dashboards JSON valides,
      scripts bash -n OK, 9 test-apps YAML + release.yaml valides
- [x] commit + push

## Journal

- 15h20 — Démarrage. Création mémoire + plan.
- 15h35 — Cartographie faite (webhooks pods vs aichangerequests identifiés comme point dur).
- 15h50 — bootstrap.Scope ajouté ; 3 mains créés ; build + tests OK.
- 16h05 — 3 Dockerfiles ; images construites en fond ; chart finops lint OK.
- 16h20 — chart confidential lint OK ; images poussées sur ghcr ; chart govar rendu complet
  validé (13 objets) avec tous les garde-fous.
- 16h35 — dashboards ×3 générés ; automatisation + test-apps finops écrites et validées
  contre les schémas CRD.
- 16h50 — automatisation + test-apps confidential écrites et validées (fix provider=simulator).
  Pause demandée puis annulée par l'utilisateur.
- 18h30 — automatisation govar + smoke test ; 10 docs CRD écrites + 12 copiées ; README ×3 +
  operators/README.md ; workflow release + pointeur README racine ; validation finale complète.
  CHANTIER TERMINÉ — commit + push effectués.

## Vérification de bout en bout (2026-07-19)

Passe de vérification complète : build Go 3 managers OK, tests webhook OK, helm lint +
template 3/3 OK, scripts bash -n OK, test kind réel des 3 opérateurs.

- **finops kind** : PASS complet — CRs réconciliés (budget Exceeded 697 %, souveraineté
  3 findings, rapport Markdown généré, gateway Ready), 52 métriques `ai_finops_*`,
  monitoring + dashboard importés.
- **confidential kind** : PASS — runtimeclasses simulées bootstrappées, webhooks pods
  (mutate/validate HTTP 200), chaîne d'attestation simulée appliquée, pod démo sur
  `simulated-kata-qemu-snp`.
- **govar kind** : DEUX bugs trouvés et corrigés dans `automatisation/up.sh`.
  1. *ImagePullBackOff* — le chart épinglait l'image admission **par digest**, mais
     `kind load` ne préserve pas les manifest digests du registre. **Fix** :
     `govArAdmission.image.digest=""` (référence par tag en kind).
  2. *Readiness crash-loop* — **bug réel dans le code source** (pas seulement dans
     l'image publiée : première hypothèse fausse, corrigée après reproduction).
     `/readyz` panique en mode `devInMemory` : **typed-nil interface** Go.
     - `main.go:145` : `var workers *durableWorkerManager` (pointeur concret, reste
       nil sans PostgreSQL) ;
     - `main.go:59` : le champ `server.workers` est de type **interface** `workerHealth` ;
     - `main.go:180` : assigner le pointeur nil dans le champ interface produit une
       **interface non-nil contenant un pointeur nil** ;
     - `main.go:340` : le garde `if s.workers != nil` est donc **vrai** → appel de
       `Healthy()` sur récepteur nil → panic sur `m.mu.RLock()`
       (`reconciliation_worker.go:377`).
     Conséquence : pod jamais Ready → `helm --wait` timeout → up.sh échoue avant les
     test-apps et le smoke test. Explique aussi le crash de l'image publiée.
     **Fix (2 niveaux)** :
     - `main.go` : n'assigner `srv.workers` que si `workers != nil` (cause racine) ;
     - `reconciliation_worker.go` : `Healthy()` tolère un récepteur nil (défense en
       profondeur) ;
     - 2 tests de régression dans `main_test.go`
       (`TestReadyzWithoutDurableWorkersStaysReady`,
       `TestReadyzSurvivesTypedNilDurableWorker`) — vérifiés : ils **paniquent sans le
       correctif**, passent avec.
     `up.sh` construit désormais l'image d'admission depuis la source
     (`Dockerfile.gov-ar-admission`, `pullPolicy=Never`) au lieu de tirer le tag publié.
- **Dashboards Grafana — corrections** (demande utilisateur) :
  - `confidential-overview.json` : 12 des 13 panels référençaient des métriques
    `aiops_*` qui n'existent nulle part dans le code → réécrit sur les métriques réelles
    du manager (controller_runtime_*, workqueue_* scopé au job de l'opérateur,
    ai_simulated_runtimeclass_in_use) ; validé live contre Prometheus (toutes les
    requêtes renvoient des séries).
  - `govar-overview.json` : `decision!="admit"` → `decision!="ADMIT"` (valeurs réelles
    ADMIT/QUEUE/REJECT/ABSTAIN/REQUIRE_APPROVAL) ; `govar_ledger_transitions_total` →
    `govar_transition_total{from,to,reason}` ; rangée réconciliation → métriques worker
    réelles (`govar_worker_backlog`, `govar_worker_heartbeat_age_seconds`,
    `govar_worker_claims_total`, `govar_worker_oldest_age_seconds`).
  - `finops-overview.json` : vérifié, toutes les métriques/labels existent — inchangé.
- **README ×3 enrichis** (demande utilisateur) : sections « Fonctionnement » (flux de
  réconciliation/décision) et « Fonctionnalités » ajoutées ; descriptions des dashboards
  alignées sur les panels corrigés.

### Bug n°3 govar — métriques `govar_*` jamais scrapées

Le ServiceMonitor du manager sélectionne bien les deux services (matchLabels par
sous-ensemble) mais son endpoint cible un port **nommé `metrics`** ; or le service
d'admission expose son port sous le nom **`http`** (8084, qui sert aussi `/metrics`).
Résultat : 0 cible pour l'admission → **toutes** les familles `govar_*` échappaient à
Prometheus (le dashboard entier aurait été vide en production).
**Fix** : ServiceMonitor dédié à l'admission dans `gov-ar-admission-service.yaml`
(`port: http`, `path: /metrics`). Vérifié en live : cible `up`, 102 séries `govar_*`.

### Bug n°4 govar — NetworkPolicy de production bloque le scrape

`enforcement.networkPolicy.enabled=true` (exigé en production par les garde-fous) pose un
ingress fail-closed n'autorisant que la gateway sur le port `ext_proc` → Prometheus ne peut
pas atteindre le port 8084. **Fix** : nouvelle valeur opt-in
`govArAdmission.enforcement.networkPolicy.monitoringNamespaceSelector` ajoutant une règle
d'ingress explicite pour le namespace de supervision (l'API y authentifie chaque requête,
donc aucun privilège d'admission accordé). Documenté dans le README govar.

### Validation live des dashboards

- confidential : toutes les requêtes renvoient des séries.
- govar : 3 panels HTTP alimentés ; 9 panels (décisions/ledger/worker) légitimement vides —
  la démo kind n'émet aucun trafic d'admission et `devInMemory` n'a pas de worker durable.
  Les compteurs Prometheus n'existent qu'après première incrémentation. Documenté.

### Publication

- `gov-ar-admission:0.5.13-article3.20260720` **poussée** sur ghcr avec le correctif
  `/readyz` — digest `sha256:bdf526715de019907a4cc290b09980555ea7775e19211a310f721e50850252b5`.
  Chart `values.yaml` réépinglé sur ce digest ; chart govar passé en `version: 0.1.1`.
- Images `finops/confidential/govar-operator:0.1.0` + `latest` reconstruites depuis la
  source courante et repoussées.
