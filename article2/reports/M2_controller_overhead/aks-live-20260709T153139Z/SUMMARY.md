# M2 — Controller overhead vs reconciled-object count

Run: `aks-live-20260709T153139Z` (live AKS, controller-runtime metrics, mock load, no LLM calls).

| tier (policies/kind) | reconciles | p50 ms | p95 ms | wq depth | API writes/s | CPU cores | mem MiB |
|--:|--:|--:|--:|--:|--:|--:|--:|
| 20 | 172 | 21.73 | 91.33 | 0 | 2.26 | 0.0124 | 53.9 |
| 60 | 555 | 20.56 | 81.91 | 0 | 4.05 | 0.018 | 53.6 |
| 120 | 1260 | 19.82 | 66.18 | 0 | 5.52 | 0.0198 | 52.7 |
