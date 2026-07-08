# Q1 Readiness Checklist — Article 1

Updated: 2026-07-06

Evidence rule: main-paper empirical claims must come from real AKS SEV-SNP.
`kind` artifacts are CI/debug/regression only.

| Check | Status | Evidence |
|---|---|---|
| Real MAA SEV-SNP evidence present? | yes | `article1/results/raw/aks/attestation-real-summary.json` |
| A1-A10 evaluated on AKS? | yes | `security_attacks_A1_A11.csv`: 300/300 blocked |
| A2/A3/A6/A7/A9 AKS rerun complete? | yes | `security_attacks_missing_A2_A3_A6_A7_A9.csv`: each 30/30 |
| A10 trust-chain separation passed? | yes | `security_attacks_A1_A11.csv`, A10 30/30 |
| A11 GPU fail-closed scoped honestly? | yes | A11 is 1/1 fail-closed and not counted as GPU CC evaluation |
| Performance N>=30 with p50/p95/p99/CI95? | yes | `scheduling_latency.csv`, `results/tables/performance.csv` |
| B4 vs B5 strong baseline? | yes | `b4_vs_b5.csv`, `b4_vs_b5_stats.csv` |
| Identity binding verified? | yes | `identity_binding.csv`: 6/6 |
| Figures generated from scripts? | yes | `article1/paper/figures/` |
| GHCR used, not ACR? | yes | charts/scripts default to `ghcr.io/...`, tag `0.5.11` |
| Limitations honest? | yes | node-level SEV-SNP only; no pod-level, GPU CC, or TDX claims |

Verdict: technical evidence is ready for the claimed scope. Complete final
editorial/IP review before submission.
