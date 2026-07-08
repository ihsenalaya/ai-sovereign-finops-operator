# Experiment Dashboard

_Last updated: 2026-07-08 06:55 UTC_

## Headline Results

- **Cost:** B6 costs 0.011353 EUR over 40 prompts versus 0.038930 EUR for premium-static (70.84% reduction under the committed catalog).
- **Open-ended quality proxy:** B6 judge mean is 0.912500 versus 0.900000 for premium-static.
- **Latency:** B6 mean latency is 1509.95 ms versus 1125.97 ms for premium-static.
- **Objective benchmark:** cost-aware accuracy is 47.20% at 0.007286 EUR.
- **Objective guardrail:** premium pinning restores 65.40% accuracy at premium cost.

## Evidence Files

| Evidence | Path |
|---|---|
| Main replay | `results/rq1_cost.csv`, `results/rq2_quality.csv`, `results/rq3_latency.csv` |
| Declared-policy scenarios | `results/rq4_sovereignty.csv` |
| Budget pressure | `results/rq5_budget.csv` |
| Objective benchmark | `results-bench/rq_benchmark.csv` |
| Objective guardrail | `results-bench/rq_guardrail.csv` |
| Prompt bootstrap | `results/bootstrap_items.csv` |
| Stability run | `results-stats/stats_summary.md` |
| Judge caveats | `results/judge_agreement_summary.md`, `results/judge_vs_truth_summary.md` |

## Submission Status

| Item | Status |
|---|:--:|
| Claims audit | done |
| Manuscript source | done |
| PDF build | done |
| Figures regenerated | done |
| Forbidden-phrase check on manuscript | pass |
| Go tests | pass |
| Python script compile | pass |

## Scope Notes

- `kind` integration is for CI, debug, and regression verification only.
- Declared-policy compliance is not legal certification.
- Judge-assessed quality is a bounded proxy; objective tasks use exact match.
- Production traces and stress tests are outside the current evidence.
