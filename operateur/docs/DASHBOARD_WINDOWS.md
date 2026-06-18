# Descriptions courtes des fenêtres du dashboard

Ce fichier décrit rapidement chaque fenêtre du dashboard Grafana
[`AI FinOps & Sovereign Operator`](../dashboards/ai-finops-overview.json).

## Fenêtres principales

| Fenêtre | Description courte |
|---|---|
| Total cost (EUR) — observed | Affiche le coût réel total observé en euros, calculé à partir des tokens mesurés par la gateway et des prix des modèles. |
| Tokens (total in + out) | Montre le volume total de tokens d'entrée et de sortie consommés par les applications. |
| Requests (total) | Compte le nombre total de requêtes LLM passées par la gateway et prises en compte par l'opérateur. |
| Budget usage (%) by policy | Indique le pourcentage d'utilisation prévu de chaque budget mensuel défini par une `AIBudgetPolicy`. |
| Cost (EUR) by namespace | Suit l'évolution du coût réel par namespace Kubernetes. |
| Tokens by namespace (input / output) | Compare les tokens d'entrée et de sortie consommés par namespace. |
| Requests violating sovereignty (by app) | Liste les applications qui envoient des requêtes vers un fournisseur non conforme à la politique de souveraineté. |
| Spend by sovereignty zone (EUR) | Répartit la dépense par zone de résidence des données, par exemple EU ou US. |
| Enforcement actions (sovereignty) | Montre les décisions prises par l'opérateur face aux violations de souveraineté : report, warn, reroute ou block. |
| Shadow-AI egress details | Détaille les connexions LLM détectées hors gateway, avec l'application, le fournisseur, la zone et la sévérité. |
| Shadow-AI hotspots by workload | Met en évidence les workloads qui contournent le plus la gateway avec des connexions directes vers des LLM. |
| Observed latency telemetry | Affiche les latences réellement mesurées quand elles existent, ou signale explicitement l'absence de télémétrie. |

## Routage et souveraineté

| Fenêtre | Description courte |
|---|---|
| Routing Score & Souveraineté | Section qui regroupe les vues liées au score de routage et à l'impact des règles de souveraineté. |
| Score de routage par application / modèle | Affiche le score final de chaque couple application/modèle, basé sur le coût, la qualité, la latence, la fiabilité et la souveraineté. |
| Évolution du score de routage dans le temps | Suit l'évolution du score de routage pour repérer les dégradations ou les violations de souveraineté. |

## Radar modèle

| Fenêtre | Description courte |
|---|---|
| Radar — Comparaison multi-dimensionnelle des modèles | Section qui compare les modèles selon plusieurs dimensions de décision. |
| Radar — Score par dimension pour chaque modèle | Affiche la comparaison multi-dimensionnelle sous forme radar. |
