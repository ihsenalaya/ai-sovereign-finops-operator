# Pre-outcome statistical plan

This plan is completed with pilot-derived margins and sample sizes before `frozen_protocol.yaml` is signed. Frozen-test outcomes may not be inspected while choosing estimands, margins, stopping rules, or claim decisions.

## Primary estimands

1. Tenant budget-window overshoot probability and magnitude under the frozen charge/carryover rule.
2. Budget utilization at a matched frozen budget-window risk target.
3. Served deterministic quality at matched risk.

Request under-reservation and fixed-cohort/selected-outstanding-set coverage are distinct secondary calibration estimands, not substitutes for the primary budget-window event.

## Secondary estimands

- admitted/queued/rejected/abstained/approval counts and false refusal;
- reservation slack and unresolved-liability backlog;
- per-tenant isolation, starvation, and noisy-neighbor impact;
- drift detection/fallback/revalidation delay and coverage gap;
- effective duplicate/late/correction outcomes and invariant failures;
- p50/p95/p99 decision latency, throughput, CPU, memory, and database contention;
- provider-window usage coverage, retries, failures, latency, and estimated token cost.

## Independent units and hierarchy

- Trace experiments: matched seed/stream blocks; tenant-window summaries are nested within a stream and requests are nested observations.
- Multi-tenant experiments: matched seed/scenario streams with prespecified tenant-window contrasts.
- Fault experiments: independently reset Kind trials, with trial ID as the unit.
- Performance: independent Kind cluster recreation, with interval samples nested within recreation.
- Azure: sampling window/deployment clusters, with requests nested within the window. Independence across windows or deployments is not presumed; the frozen analysis must justify its covariance structure and use cluster-robust or hierarchical uncertainty when dependence remains.

Individual requests are never treated as independent replicates for population-level method comparisons.

## Required comparators

- no control and settled-spend-only;
- strict provider/max reservation, mean, fixed margin, fixed quantile, adaptive quantile;
- faithful R55/R56 fixed envelope plus safety multiplier with atomic pending spend and its documented expiry semantics;
- R01-style locked adaptive estimator;
- adaptive quantile without joint routing and expected-cost budgeted routing;
- strong compatible routing policies including RACER/CONCUR/SDR or a documented topology/objective exclusion, RouteLLM/HybridLLM/PILOT-class methods, cheapest compliant, current operator score, fixed premium/cheap, and evaluation-only oracles;
- full GOV-AR and every frozen ablation.

All methods receive identical stream, hidden outcome table, prices, budgets, delays, faults, policies, and seed. Matching uses explicit stream/config/outcome hashes, never positional list order.

## Operational falsifier and decision rule

Before freeze, the development pilot sets scientifically justified equivalence/noninferiority margins for risk, utilization, quality, refusal, and availability, plus a minimal practically relevant effect. The protocol freezes:

- the primary risk target/event and matched-risk interpolation procedure;
- whether the decision uses Pareto dominance or a prespecified scalar utility;
- superiority and equivalence/noninferiority margins;
- which regimes may support an advantage claim;
- the exact claim-removal rule when a practical envelope or adaptive quantile is equivalent/Pareto-nondominated.

Failure to reject a difference is not equivalence. Equivalence requires the frozen interval/test and margin.

## Intervals, tests, and multiplicity

- paired hierarchical/block bootstrap intervals over independent units;
- exact or Wilson binomial intervals where their independence model is valid;
- exact one-sided upper bounds for zero observed events;
- paired permutation, nonparametric, or prespecified mixed/model-based comparisons as pilot diagnostics justify;
- bootstrap uncertainty for median/p95/p99 clustered by recreation/window;
- calibration curves and coverage gaps for request, fixed-cohort, and selected-set events separately;
- Holm correction within each frozen primary/secondary hypothesis family;
- effect sizes and intervals reported regardless of p-value.

Rare-event sequential extension changes request blocks for every matched method together and stops only at the frozen event target or maximum. It does not inflate the number of independent seeds. The final report distinguishes attempted, retried, failed, excluded, and analyzed observations.

## Missingness and invalid runs

Hash mismatch, missing events, corrupted ledger totals, incomplete diagnostics, cross-method stream mismatch, test-split access, or implementation bugs invalidate affected runs. Missing/late telemetry is a treatment/fault outcome when intentionally injected, not an exclusion. Exclusions and censoring rules are frozen and reported by method/condition.

## Null and negative results

If GOV-AR is equivalent, worse, or operationally unavailable relative to a faithful practical envelope or strong router, the result remains in every table and the contribution is narrowed. Frozen-test tuning and selective condition removal are prohibited.
