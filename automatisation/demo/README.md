# Démonstration end-to-end de l'opérateur

Une seule commande pour voir **toutes les fonctionnalités** de l'AI Sovereign FinOps Operator sur un
cluster kind local, avec **Prometheus + Grafana** déployés pour visualiser les métriques.

## Prérequis
- Un cluster **kind** nommé `greenops` (contexte `kind-greenops`) — créé via `automatisation/` (`make local`).
- `kubectl`, et `helm` si l'opérateur n'est pas déjà installé (le script l'installe sinon).
- L'image `ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.2.1` (publique sur GHCR).

> Contexte différent ? `KCTX=<mon-context> ./demo.sh up`

## Lancer

```bash
cd automatisation/demo
./demo.sh up        # déploie tout, affiche le tour guidé, ouvre Grafana
```

`./demo.sh up` enchaîne :
1. **Opérateur** — vérifie/installe le déploiement (Helm, image 0.2.0).
2. **CRs** — applique `config/samples/` + `demo-extra.yaml` (catalogue, providers FR/US, policy de
   souveraineté, budgets, break-even, rapports) et déclenche les réconciliations.
3. **Observabilité** — déploie un **Prometheus** (scrape l'opérateur) + une **Grafana** légers, avec le
   dashboard `dashboards/ai-finops-overview.json` auto-provisionné.
4. **Tour guidé** — imprime, moteur par moteur, les résultats réels (voir ci-dessous).
5. **Grafana** — port-forward sur `http://localhost:3000` (admin anonyme) et Prometheus sur `:9090`.

```bash
./demo.sh tour      # ré-imprime seulement le tour guidé (état courant)
./demo.sh down      # retire Prometheus/Grafana + CRs de démo (garde l'opérateur)
```

## Ce que la démo montre (les 7 fonctionnalités)

| # | Fonctionnalité | Où le voir |
|---|---|---|
| 1 | **Catalogue & gateway** | `AIGateway` / `AIProvider` (FR + US) / `AIModel` |
| 2 | **Attribution des coûts** | `AIFinOpsReport.status` : coût total, tokens, top modèles |
| 3 | **Souveraineté par flux** | findings attribués `namespace/app → modèle@fournisseur (zone) ×requêtes` — le flux `finance/risk-assistant → openai-us` est **critical** (zone US interdite) |
| 4 | **Budget & dégradation gracieuse** | `rh-tight-budget` à **120 %** → phase `Exceeded` (vs `rh-budget` 12 % `WithinBudget`) |
| 5 | **Break-even managé vs auto-hébergé** | `AIBreakEvenAnalysis.status` : recommandation + économie |
| 6 | **Reporting** | ConfigMap `<report>-report` (clés `report.md` / `report.json`) |
| 7 | **Observabilité** | métriques `ai_finops_*` dans Prometheus + dashboard Grafana (9 panels, dont **« Sovereignty findings by application »**) |

## Dans Grafana
Ouvrir `http://localhost:3000` → dashboard **« AI Sovereign FinOps — Overview »** :
coût total, requêtes, jauge budget, findings critiques, coût/tokens par namespace, économies break-even,
recommandations, et **violations de souveraineté par application** (nouveau panel).

## Notes
- Stack volontairement **légère** (faibles requests CPU/mémoire) pour tenir sur un petit cluster kind.
- La télémétrie provient du **fake collector** (données de démo) — aucune clé API requise pour la démo.
- Tout est **reportOnly** : l'opérateur n'altère jamais le trafic.
