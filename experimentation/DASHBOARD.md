# Experiment dashboard

_Last updated: 2026-06-07 06:48 UTC_  ·  regenerate: `python3 scripts/dashboard.py`

## Test runs (status + duration)

| Run dir | Tests | PASS | FAIL | Duration | Groups |
|---|---:|---:|---:|---:|---|
| results | 46 | 46 | 0 | 4s | RQ-ablation,RQ1-3-matrix,RQ4-sovereignty,RQ5-bud |
| results-bench | 7 | 7 | 0 | 48s | BENCHMARK |
| results-stats | 30 | 30 | 0 | 36m20s | STATS-matrix |
| **TOTAL** | **83** | **83** | **0** | **37m12s** | — |

**Integrity:** 83/83 PASS, none, no skipped tests.

## Headline results

- **Cost** (Ours vs premium): −70.84% (B6 0.011353 vs B1 0.038930 EUR).
- **Quality** (judge, norm): Ours 0.912500 vs premium 0.900000 (win-rate 50.00%).
- **Sovereignty (EU-only)**: Ours served 40/40, 0 violations (blind baseline: 40).
- **Objective benchmark (exact-match)**: B1 premium static=70.00%; B6 ours=40.00%
- **Statistics**: see `results-stats/stats_summary.md` (N=30, CIs + Mann-Whitney + Cliff δ / Cohen d).
- **Inter-judge agreement**: see `results/judge_agreement_summary.md`.

## Q1 hardening progress

| Step | Status |
|---|:--:|
| Multi-provider (OpenAI + Mistral EU) | ✅ |
| Real EU sovereignty (RQ4) | ✅ |
| ≥30-rep statistics + effect sizes | ✅ |
| Multi-judge agreement | ✅ |
| Learned-style baseline (B7) | ✅ |
| Public benchmark + exact-match | ✅ |
| Feature-matrix positioning | ✅ |
| Scale-up (currently 70 prompts; target 1000s) | ⬜ |
| Live gateway enforcement under load | ⬜ |
| Human evaluation | ⬜ |
| Submission format (LaTeX) + artifact DOI | ⬜ |

_Datasets: 70 prompts across 5 files._
