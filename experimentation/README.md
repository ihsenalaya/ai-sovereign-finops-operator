# experimentation/ — Q1 article experimental package

Reproducible experimental framework for the paper **"Economic-Aware and Sovereignty-Constrained
Routing for Enterprise LLM Gateways"**. The AI Sovereign FinOps Operator (this repo) is the
artifact; this folder evaluates the approach (RQ1–RQ6) with **real LLM calls** and the operator's
**actual engines** (no mocks for cost/sovereignty/break-even).

> Status: single provider (**OpenAI**) phase. Architecture is provider-agnostic — a second provider
> plugs in via `internal/llm` + `internal/catalog`. GPU/self-hosted (vLLM) is **modeled** for now;
> real GPU runs are deferred to an Azure phase. Real vs modeled data is tagged in `results/calls.csv`.

## What it measures
- **RQ1 Cost** — economic-aware routing vs static baselines (savings, cost/req, cost/token, per team).
- **RQ2 Quality** — LLM-as-judge acceptability + win-rate vs premium (quality preservation).
- **RQ3 Latency** — p50/p95/p99 + routing-decision overhead (µs), with bootstrap 95% CIs.
- **RQ4 Sovereignty** — cost/quality/violations across 5 declarative scenarios.
- **RQ5 Budget** — graceful degradation vs hard-block vs alert-only (availability, overrun).
- **RQ6 Break-even** — managed API vs self-hosted GPU (modeled) across token volumes.
- **Ablation** — contribution of the scoring terms (cost/quality/latency).

## Layout
```
cmd/experiment/      Go CLI (runs everything, journals every test)
internal/
  llm/               provider-agnostic chat client (OpenAI); key from file/env, never logged
  catalog/           model/provider catalog -> operator costengine + sovereigntyengine
  router/            6 strategies (B1-B6) incl. the scoring algorithm (Ours)
  quality/           LLM-as-judge (acceptability + pairwise)
  workload/          dataset loader
  journal/           test journal (JSONL + TEST_STATUS.md)
  runner/            RQ1-RQ6 + ablation, CSV export, response cache
datasets/            W1-W4 synthetic prompt sets
scripts/             run_experiment.sh, analyze_results.py, export_results.sh, collect_metrics.sh
results/             CSVs, summary.md, TEST_STATUS.md, journal.jsonl, cache.json
figures/             fig2..fig8 (publication-ready PNG)
paper/               outline, methodology, threats_to_validity, tables
manifests/           live-platform integration notes (Envoy + operator + Prometheus)
```

## Run it
```bash
# 1) real experiments (RQ1-RQ6 + ablation); ~10-15 min, responses cached to results/cache.json
scripts/run_experiment.sh            # uses ../docs/openaikey.txt, judge=gpt-4o

# 2) analysis + figures + summary
python3 scripts/analyze_results.py   # needs pandas matplotlib scipy tabulate

# 3) (optional) snapshot
scripts/export_results.sh
```
Re-runs are **free and identical**: `results/cache.json` holds every LLM response and judge score.
Delete it to force fresh calls.

## Testing standard
Every test is journaled (status, **duration**, details) in `results/TEST_STATUS.md` +
`results/journal.jsonl`. **No test is skipped**: any API error aborts the run, so results are never
silently missing. Unit tests cover the router/engines (`go test ./experimentation/...`).

## Headline result
Two-provider run (OpenAI US + **Mistral EU** via Azure AI Foundry, DataZone EU):
- **Cost −70.8%** vs premium; a 30-rep run confirms it (p≈3×10⁻¹¹, Cohen d≈−43, non-overlapping CIs).
- **Quality ≥ comparable** (judge-dependent; two-judge agreement κ=0.40, ρ=0.61, within-1 94%).
- **RQ4 sovereignty real**: under EU-only, **100% availability via Mistral EU, 0 violations** vs 40 for
  the sovereignty-blind baseline.
- **Latency** significantly higher for Ours (real trade-off, d≈+3.7).
- vs a literature-style **difficulty router (B7)**: B7 saves 73% at quality 0.866; Ours 71% at 0.91 +
  governance. See `results/summary.md`, `results-stats/stats_summary.md`, `results/judge_agreement_summary.md`.

## Scripts
- `scripts/run_experiment.sh` — deterministic RQ1–RQ6 + ablation.
- `cmd/experiment -stats-reps 30 -stats-temp 0.7` — multi-repetition statistical run.
- `cmd/judgeagree` + `scripts/judge_agreement.py` — two-judge agreement (κ/α/ρ).
- `scripts/analyze_results.py`, `scripts/analyze_stats.py`, `scripts/stats.py`, `scripts/md_to_pdf.py`.
- `cmd/gpubench` — GPU/vLLM bench (OpenAI-compatible; deferred real-GPU phase).

## Adding the second provider (next phase)
1. Add a `llm.Client` implementation (or reuse `OpenAI` with a different base URL / provider label).
2. Add the provider's models to `internal/catalog` (pricing, zone, managed, quality prior).
3. Re-run — strategies, journaling, analysis and figures work unchanged.

## Plugging the real operator (later)
The experiment already imports the operator's pure engines. For live, in-cluster runs, deploy the
operator (`../charts`, `../automatisation`), route through Envoy, and feed the Prometheus
`ai_finops_*` metrics into `scripts/collect_metrics.sh`. See `manifests/README.md`.
