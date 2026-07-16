# Décomposition en 3 opérateurs indépendants

Ce dossier décompose l'opérateur monolithique `ai-sovereign-finops-operator`
(19 CRDs / 18 controllers / 1 image) en **3 opérateurs spécialisés, indépendants à
l'installation comme à l'exécution**. Chaque opérateur a son propre manager, sa propre
image, son propre chart Helm (CRDs + RBAC scopés), sa documentation par CRD, son
automatisation kind avec applications de test et son dashboard Grafana.

| Opérateur | Spécialité | CRDs | Image | Dossier |
|---|---|---|---|---|
| **ai-finops-operator** | FinOps & souveraineté du trafic IA (coûts, budgets, résidence, break-even, quality gates, routage, approbations) | 11 | `finops-operator` | [`ai-finops-operator/`](ai-finops-operator/) |
| **ai-confidential-operator** | Attestation TEE & placement vérifiable (RATS SEV-SNP/TDX, key-release, révocation, audit chaîné, webhooks pods) | 7 | `confidential-operator` | [`ai-confidential-operator/`](ai-confidential-operator/) |
| **ai-govar-operator** | Admission gouvernée GOV-AR (identité workload, admission `ext_proc`, ledger PostgreSQL, calibration, approbation fail-closed) | 1 possédée + 5 lues | `govar-operator` | [`ai-govar-operator/`](ai-govar-operator/) |

Images publiées sur `ghcr.io/ihsenalaya/ai-sovereign-finops-operator/` (tags `0.1.0`).

## Principes de la décomposition

- **Groupe API conservé** : `aiops.imperium.io/v1alpha1` pour les trois — aucune migration
  de données ; les CRs existantes restent valides.
- **Un propriétaire par CRD** : chaque chart n'installe et ne réconcilie que ses CRDs. Le
  chart govar embarque une **copie** des CRDs catalogue qu'il lit (Helm saute les CRDs déjà
  présentes → coexistence sans conflit avec le FinOps).
- **Webhooks séparés par domaine** : pods (injection/validation confidentielle) →
  confidential ; `aichangerequests` (tampon d'identité d'approbation fail-closed) → govar.
  Le bootstrap accepte désormais un `Scope` (`Pods` / `ChangeRequests` / `All`), le monolithe
  gardant le comportement historique (`All`).
- **Code partagé, distribution séparée** : les trois managers vivent dans le module Go commun
  `operateur/` (`cmd/finops-manager`, `cmd/confidential-manager`, `cmd/govar-manager`) — pas
  de fork de code, pas de drift de schéma — mais chaque opérateur est **construit, versionné,
  installé et exploité indépendamment**.
- **Dégradation gracieuse** : chaque opérateur fonctionne seul ; les intégrations croisées
  (catalogue FinOps lu par GOV-AR, approbations) sont opt-in et documentées dans chaque README.

## Démarrage rapide

Chaque opérateur fournit un cluster kind complet en une commande (opérateur + Prometheus +
Grafana + applications de test + dashboard) :

```bash
cd ai-finops-operator/automatisation        && ./up.sh
cd ai-confidential-operator/automatisation  && ./up.sh
cd ai-govar-operator/automatisation         && ./up.sh
```

Les trois clusters kind (`finops-operator`, `confidential-operator`, `govar-operator`)
peuvent coexister sur la même machine. `./down.sh` supprime le cluster correspondant.

## Compatibilité avec le monolithe

Le chart historique `operateur/charts/ai-sovereign-finops-operator` (image `controller`)
reste fonctionnel et inchangé. **Ne pas mélanger** le monolithe et les opérateurs découpés
sur un même cluster : les mêmes CRDs seraient réconciliées deux fois et les webhooks
enregistrés en double. Choisir l'un ou l'autre par cluster.

## Matrice d'indépendance

| | FinOps absent | Confidential absent | GOV-AR absent |
|---|---|---|---|
| **FinOps** | — | aucun impact | workflow `reroute` intact ; pas de tampon d'identité reviewer |
| **Confidential** | aucun impact | — | aucun impact |
| **GOV-AR** | installer soi-même les CRs catalogue (CRDs incluses dans son chart) | aucun impact | — |
