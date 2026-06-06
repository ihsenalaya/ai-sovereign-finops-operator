# AI Sovereign FinOps Operator

> Plan de contrôle **FinOps & souveraineté** pour l'IA d'entreprise, packagé comme opérateur Kubernetes.

L'opérateur se branche en **lecture seule** sur une gateway IA existante (LiteLLM, Envoy, Kong,
Gateway API, ou custom) et gouverne les appels LLM d'une organisation : coût, attribution,
fournisseurs/pays, budgets, souveraineté des données, et arbitrage *API managée vs auto-hébergement GPU*.

Cible : entreprises **françaises et européennes régulées** (santé, finance, assurance, secteur
public, industrie critique) sensibles à la souveraineté des données.

⚠️ **Le produit ne promet pas la conformité juridique.** Il produit une **traçabilité exploitable
et un dossier de préparation à l'audit** (RGPD, AI Act, politiques internes). En MVP il est
**non-bloquant** (`reportOnly`) et ne modifie jamais la gateway.

---

## État du projet — MVP complet (Sprints 1→6) ✅

**Validé de bout en bout sur un cluster kind via l'image déployée par Helm** :

- **7 CRDs** (`api/v1alpha1/`) : schémas OpenAPI, validations, status + conditions standard, print-columns.
- **7 controllers** (`internal/controller/`) : reconcile réel, `observedGeneration`, events, logs structurés ; watch `AIModel`→`AIProvider`.
- **Moteurs purs** : [`costengine`](docs/features/costengine.md) (95 %), [`budgetengine`](docs/features/budgetengine.md) (100 %), [`sovereigntyengine`](docs/features/sovereigntyengine.md) (80 %), [`breakevenengine`](docs/features/breakevenengine.md) (87 %), [`reporting`](docs/features/reporting.md) (87 %).
- **Collecte** : [`fake`, `prometheus`](docs/features/collectors.md) opérationnels, `litellm` préparé.
- **Observabilité** : métriques `ai_finops_*` + [dashboard Grafana](dashboards/ai-finops-overview.json).
- **Reporting** Markdown/JSON écrit dans un ConfigMap `<report>-report`.
- **Helm chart** + **image** + [`automatisation/`](automatisation) (kind + ArgoCD GitOps, et chemin offline Helm).
- **Tests** : unitaires (moteurs) + envtest (controllers), tous verts.

Détails : [docs/ROADMAP.md](docs/ROADMAP.md) · plan technique : [docs/MVP_PLAN.md](docs/MVP_PLAN.md) · **toute la doc** : [docs/README.md](docs/README.md).

| CRD | Rôle | shortName |
|-----|------|-----------|
| `AIGateway` | Gateway IA observée + mode de télémétrie | `aigw` |
| `AIProvider` | Fournisseur (managé/auto-hébergé), pricing, conformité | `aiprov` |
| `AIModel` | Modèle catalogué, lié à un provider | — |
| `AIBudgetPolicy` | Budget par namespace/équipe/app + seuils | `aibudget` |
| `AISovereigntyPolicy` | Règles de résidence/souveraineté (reportOnly) | `aisov` |
| `AIBreakEvenAnalysis` | Point mort API managée vs GPU auto-hébergé | `aibreakeven` |
| `AIFinOpsReport` | Rapport généré (résultats en `.status`) | `aireport` |

---

## Démarrage rapide (kind)

```bash
# 1. CRDs dans le cluster courant
make install

# 2. Lancer le manager localement contre le cluster (pas de build d'image)
make run            # Ctrl-C pour arrêter

# 3. Dans un autre terminal : appliquer les exemples
kubectl apply -k config/samples/

# 4. Observer
kubectl get aigw,aiprov,aimodel,aibudget,aisov,aibreakeven,aireport
kubectl describe aimodel mistral-small
```

Guide de démo détaillé : [docs/DEMO_KIND.md](docs/DEMO_KIND.md).

## Développement

```bash
make manifests generate   # régénère CRDs, RBAC, deepcopy
make test                 # tests envtest
make build                # binaire manager
make docker-build IMG=…   # image (Sprint 6 pour le Helm chart)
```

> Toolchain testée : Go 1.25, Kubebuilder 3.14, controller-gen v0.18.0, kind 0.31 (k8s 1.35), envtest k8s 1.31.

## Stack & principes

Tout le stack vise des briques **open-source / CNCF** : Kubernetes, controller-runtime, Prometheus,
Grafana, Helm, Kustomize, Envoy (intégration gateway). Voir [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Licence

Apache 2.0.
