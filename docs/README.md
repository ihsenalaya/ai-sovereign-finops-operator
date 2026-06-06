# Documentation — AI Sovereign FinOps Operator

## Vue d'ensemble
- [Plan technique du MVP](MVP_PLAN.md) — architecture, CRDs, packages, interfaces, commandes.
- [Architecture (HLD)](ARCHITECTURE.md) — design, décisions, flux de réconciliation.
- [Roadmap](ROADMAP.md) — backlog sprint par sprint (statut).
- [Démo kind](DEMO_KIND.md) — installation et démonstration locale.
- [Prompt produit d'origine](PRODUCT_PROMPT.txt).

## CRDs (référence par ressource)
| Kind | Doc | Rôle |
|------|-----|------|
| AIGateway | [crds/aigateway.md](crds/aigateway.md) | Gateway IA observée |
| AIProvider | [crds/aiprovider.md](crds/aiprovider.md) | Fournisseur + pricing + conformité |
| AIModel | [crds/aimodel.md](crds/aimodel.md) | Catalogue de modèles |
| AIBudgetPolicy | [crds/aibudgetpolicy.md](crds/aibudgetpolicy.md) | Budgets & seuils |
| AISovereigntyPolicy | [crds/aisovereigntypolicy.md](crds/aisovereigntypolicy.md) | Règles de souveraineté |
| AIBreakEvenAnalysis | [crds/aibreakevenanalysis.md](crds/aibreakevenanalysis.md) | Point mort GPU |
| AIFinOpsReport | [crds/aifinopsreport.md](crds/aifinopsreport.md) | Rapport généré |

## Fonctionnalités (par package/moteur)
| Fonctionnalité | Doc |
|----------------|-----|
| Collecte de télémétrie | [features/collectors.md](features/collectors.md) |
| Moteur de coût | [features/costengine.md](features/costengine.md) |
| Moteur de budget | [features/budgetengine.md](features/budgetengine.md) |
| Moteur de souveraineté | [features/sovereigntyengine.md](features/sovereigntyengine.md) |
| Moteur break-even | [features/breakevenengine.md](features/breakevenengine.md) |
| Reporting (MD/JSON) | [features/reporting.md](features/reporting.md) |
| Observabilité (métriques) | [features/metrics.md](features/metrics.md) |

## Déploiement & exploitation
- [Helm chart](../charts/ai-sovereign-finops-operator) — packaging.
- [automatisation/](../automatisation) — kind + ArgoCD end-to-end, et chemin offline Helm.
- [Installation AKS](INSTALL_AKS.md).
- [Dashboard Grafana](../dashboards/ai-finops-overview.json).

## Guides
- [Développement](DEVELOPMENT.md).
- [Contribution](CONTRIBUTING.md).
- [Limites & hypothèses du MVP](LIMITATIONS.md).
