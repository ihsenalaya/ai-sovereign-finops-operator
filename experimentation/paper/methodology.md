# Methodology

## Testbed
- Control plane: AI Sovereign FinOps Operator (this repo); its pure engines (costengine,
  budgetengine, sovereigntyengine, breakevenengine) are reused directly by the experiment harness, so
  measured cost/sovereignty/break-even logic is exactly the operator's, not a reimplementation.
- LLM provider (current phase): **OpenAI only**. The harness is provider-agnostic (`internal/llm`
  `Client` interface + model catalog); a second provider plugs in without changing the experiments.
- Self-hosted model: **modeled** (no GPU in the local testbed). Its cost is computed from declared
  parameters and its responses come from a deterministic stub. GPU/vLLM real runs are deferred to a
  later Azure phase. Real and modeled rows are tagged (`real`, `modeled`) in `calls.csv`.

## Models (catalog)
| Tier | Model (OpenAI) | EUR/1M in | EUR/1M out | Zone | Managed |
|------|----------------|-----------|------------|------|---------|
| premium | gpt-4o | 2.50 | 10.00 | US | yes |
| medium | gpt-4o-mini | 0.15 | 0.60 | US | yes |
| cheap | gpt-4.1-nano | 0.10 | 0.40 | US | yes |
| self-hosted (modeled) | selfhosted-eu-llama | 0.05 | 0.05 | FR | no |

Prices are approximate public EUR rates at experiment time; they are configuration, not results, and
are isolated in `internal/catalog` for easy update / sensitivity analysis.

## Workloads (W1-W4)
HR chatbot (short, sensitive), RAG docs (long context, quality-sensitive), Dev assistant (high
quality), Analytical agent (high volume, cost-explosion risk). Each carries team, namespace,
sensitivity, monthly budget, premium model, allowed models, and a minimum quality threshold. Datasets
in `datasets/*.json`.

## Baselines & approach (B1-B6)
B1 premium-static, B2 round-robin, B3 least-cost, B4 static namespace policy, B5 budget hard-block,
B6 Ours (economic-aware, sovereignty-constrained, budget-aware). Defined in `internal/router`.

## Algorithm (B6)
Hard constraint: reject any model violating the sovereignty scenario (forbidden zone; outside allowed
zones, where EU covers EU member states; managed-external provider for sensitive data when
disallowed). Among remaining candidates meeting the minimum quality (relaxed only when the budget is
exhausted, enabling graceful degradation), choose argmin of:

```
score(m) = α · norm_cost(m) · (1 + ε · budget_pressure)
         + β · quality_loss(m)
         + γ · latency_penalty(m)
         + δ · sovereignty_risk(m)
norm_cost      = est_cost(m) / est_cost(premium)
quality_loss   = max(0, (q_premium − q_m) / q_premium)
latency_penalty= latency_prior(m) / max_latency
budget_pressure= used / total
```
Default weights: α=1.0, β=1.5, γ=0.3, δ=0.5, ε=1.0 (`router.DefaultWeights`).

## Metrics
- Cost: total EUR, per request, per token, per team/namespace (via the operator costengine),
  relative savings vs premium.
- Quality: LLM-as-judge acceptability (1-5 → normalized 0-1), acceptable-rate (≥3), pairwise
  win-rate vs the premium reference answer.
- Latency: per-call p50/p95/p99 and mean (real API latency); routing-decision overhead (µs).
- Sovereignty: violations, reroutes, blocked, cost, quality per scenario.
- Budget: availability, served/blocked, budget overrun, quality.
- Break-even: managed vs self-hosted monthly cost, savings, payback months, recommendation.

## Execution environment and protocol
- **Software:** Go 1.25 harness; Python 3.10 analysis (numpy, pandas, scipy, matplotlib). OpenAI Chat
  Completions API. Versions and prices are configuration, not results.
- **Model snapshots:** model *aliases* are used; the exact dated snapshots returned at run time should
  be pinned for camera-ready (TODO — record the served `model` field per call). Prices are public EUR
  rates at experiment time (`internal/catalog`).
- **Determinism / seeds:** temperature 0 for all answer and judge calls; bootstrap seed fixed (42).
  Baselines are deterministic except round-robin (per-strategy state).
- **Warm-up vs measurement:** premium reference answers are pre-warmed and cached before the measured
  matrix so pairwise comparisons have a stable reference; the measured pass reads cache for repeated
  (model, prompt) pairs, isolating new calls.
- **Repetition status:** the committed run is **one deterministic pass**. The protocol and the stats
  scaffold (`scripts/stats.py`) target **N ≥ 30** repetitions; that run is future work and is NOT
  reflected in current numbers.

## Measured vs modeled (do not conflate)
| Component | Status | How |
|-----------|--------|-----|
| Answer cost (OpenAI tiers) | **measured** | real token usage × catalog price |
| Quality (acceptability, win-rate) | **measured** | LLM-as-judge (gpt-4o), 1 pass |
| Latency (end-to-end, routing µs) | **measured** | real API timing; 1 pass |
| Sovereignty violations/reroutes | **measured** | real routing decisions over scenarios |
| Self-hosted answers/quality | **modeled** | deterministic stub; quality = prior |
| Self-hosted & GPU cost (RQ6) | **modeled** | declared GPU/ops parameters |
| Per-team cost aggregation | **measured** | operator `costengine` over real usage |

## Statistics & reproducibility
- LLM calls use temperature 0 for determinism of cost/quality; latency retains natural variance.
- 95% confidence intervals for latency via bootstrap (2000 resamples) on per-call data
  (`scripts/analyze_results.py`).
- All LLM responses and judge scores are cached to `results/cache.json`, making the full pipeline
  reproducible and re-runs free (no re-billing). Re-running yields identical cost/quality.
- Current scale: 1 measured pass over the full matrix (deterministic). The protocol supports ≥30
  repetitions per scenario (config) for the camera-ready; see threats_to_validity.md.
- One-command run: `scripts/run_experiment.sh`; analysis: `scripts/analyze_results.py`.
