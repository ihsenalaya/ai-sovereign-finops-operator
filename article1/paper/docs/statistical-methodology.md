# Statistical Methodology

Mesures principales:
- `admission_latency_ms`
- `scheduling_latency_ms`
- `filter_latency_ms`
- `score_latency_ms`
- `permit_wait_ms`
- `prebind_latency_ms`
- `pod_pending_duration_ms`
- `pod_admission_to_running_ms`

Résumé statistique:
- médiane
- moyenne seulement en complément
- p95 / p99
- IC 95 % par bootstrap si possible
- test de Mann-Whitney U pour comparaison de latence

Règles:
- ne jamais agréger simulation et réel sans colonne d'environnement
- ne jamais supprimer les runs défavorables
- conserver le warm-up dans les données brutes
