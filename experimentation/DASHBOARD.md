# Experiment dashboard

_Last updated: 2026-06-07 07:51 UTC_  ·  regenerate: `python3 scripts/dashboard.py`

## Test runs (status + duration)

| Run dir | Tests | PASS | FAIL | Duration | Groups |
|---|---:|---:|---:|---:|---|
| results | 46 | 46 | 0 | 4s | RQ-ablation,RQ1-3-matrix,RQ4-sovereignty,RQ5-bud |
| results-bench | 1 | 1 | 0 | 13s | gpubench |
| results-stats | 30 | 30 | 0 | 36m20s | STATS-matrix |
| **TOTAL** | **77** | **77** | **0** | **36m37s** | — |

**Integrity:** 77/77 PASS, none, no skipped tests.

## Headline results

- **Cost** (Ours vs premium): −70.84% (B6 0.011353 vs B1 0.038930 EUR).
- **Quality** (judge, norm): Ours 0.912500 vs premium 0.900000 (win-rate 50.00%).
- **Sovereignty (EU-only)**: Ours served 40/40, 0 violations (blind baseline: 40).
- **Objective benchmark (exact-match)**: B1 premium static=65.40%; B6 ours=47.20%
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
| Scale-up (540 prompts; target >=500) | ✅ |
| Envoy data-path overhead under load | ✅ |
| Human evaluation (package ready) | ⬜ |
| Submission format (LaTeX) ready | ✅ |
| Artifact metadata (Zenodo/CITATION) | ✅ |

_Datasets: 540 prompts across 6 files._

## Recommended next actions (status)

| # | Action | Status | Note |
|--:|---|:--:|---|
| 1 | Investigate exact-match vs judge contradiction | ✅ | RESOLVED — judge rates 81% of *wrong* answers acceptable (r=0.12); see results/judge_vs_truth_summary.md |
| 2 | Human evaluation on 100 examples | ⬜ | PACKAGE READY (100 blind items) — awaiting evaluators; see human_eval/README.md |
| 3 | Increase dataset to >=500 prompts | ✅ | 540 prompts (40 synthetic + 500 public GSM8K/MMLU) |
| 4 | Live gateway routing benchmark under load | ✅ | DONE (local real Envoy): data-path overhead ~+3%, 0 errors @ concurrency 8; see results-bench/envoy_overhead_summary.md |
| 5 | LaTeX + GitHub/Zenodo packaging | ✅ | LaTeX (paper/paper.tex) + .zenodo.json + CITATION.cff READY; mint DOI via a GitHub Release (see paper/RELEASE_ARTIFACT.md) |

## Critical gaps

1. **Quality contradiction** — RESOLVED: LLM-judge over-rates wrong answers on objective tasks; we now report exact-match where ground truth exists.
2. **Dataset scale** — 540 prompts (>=500 OK).
3. **Human validation** — still missing (needs evaluators).
4. **Live gateway enforcement** — control-plane validated; data-path under load pending.
