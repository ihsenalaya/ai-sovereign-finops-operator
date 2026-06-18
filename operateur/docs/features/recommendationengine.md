# Fonctionnalité — Recommendation engine (`internal/recommendationengine`)

Transforme l'usage observé, le catalogue de modèles et les constats de souveraineté en
**recommandations chiffrées et actionnables**. Moteur **pur** (aucune dépendance Kubernetes), calculant
les montants en EUR à partir des **mêmes volumes de tokens réels** et prix que le moteur de coût.

## Types de recommandations
- **`cost-saving`** — router le trafic d'une app vers un modèle moins cher, avec l'économie EUR estimée
  sur la fenêtre observée (seuil de bruit : ≥ 20 % d'économie).
- **`sovereignty`** — requêtes/coût partant vers un fournisseur non conforme (zone interdite) et le correctif.
- **`data-quality`** — usage de modèles sans prix (coût non attribuable).

## Souveraineté d'abord (correctif clé)
Un swap `cost-saving` n'est **jamais** proposé vers un modèle non conforme, si bon marché soit-il. Chaque
`Candidate` porte un flag `Compliant` (calculé par le controller via la zone du fournisseur et la policy
de souveraineté active) ; la sélection du moins cher **ignore** les candidats non conformes.

> Exemple réel : pour `marketing/content-writer` sur `mistral-large` (UE), l'ancienne version proposait
> `gpt-4o-mini` (US, **zone interdite**) — une contradiction pour un produit *sovereign FinOps*. Désormais
> la recommandation ne cible que le **modèle conforme le moins cher** (ou disparaît s'il n'en existe pas).

## Câblage & métriques
Le controller `AIFinOpsReport` agrège l'usage par (namespace, application, modèle), construit les
candidats avec leur conformité, appelle `Recommend(...)`, puis expose `ai_finops_recommendations`,
`ai_finops_cost_saving_eur`, `ai_finops_potential_savings_*`. Les séries sont purgées par report (tracker
par-UID + finalizer) — pas de série fantôme.

Lié : [costengine](costengine.md), [sovereigntyengine](sovereigntyengine.md),
[enforcementengine](enforcementengine.md), [metrics](metrics.md).
