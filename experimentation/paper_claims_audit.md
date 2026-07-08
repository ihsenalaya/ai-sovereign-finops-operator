# Paper Claims Audit for JNCA Resubmission

Date: 2026-07-08

Scope: `experimentation/` manuscript and experiment artifact, cross-checked against the operator implementation under `operateur/`. This audit was completed before editing the manuscript source.

## 1. Repository Structure

| Area | Path | Role |
|---|---|---|
| Manuscript sources | `experimentation/paper/latex/main.tex`, `experimentation/paper/paper.md`, `experimentation/paper/paper.tex` | Current paper drafts. The LaTeX source is the submission source. |
| Experiment harness | `experimentation/cmd/experiment`, `experimentation/internal/*` | Runs routing strategies, cached/replayed calls, public benchmark evaluation, and CSV generation. |
| Workloads | `experimentation/datasets`, `experimentation/datasets-public` | Four synthetic governance-motivated workloads plus GSM8K/MMLU benchmark subsets. |
| Results | `experimentation/results`, `experimentation/results-stats`, `experimentation/results-bench` | Main CSVs, cached responses, 30-repetition run, benchmark accuracy results, judge agreement results. |
| Analysis scripts | `experimentation/scripts` | Figure generation, statistics, sensitivity analysis, judge-vs-truth analysis. |
| Operator source | `operateur/internal`, `operateur/pkg`, `operateur/api/v1alpha1` | Production operator engines, CRDs, controllers, telemetry collectors, quality gate, and enforcement logic. |
| Legacy/future-study material | `experimentation/paper/GPU_SELF_HOSTING_VALIDATION_PLAN.md`, `experimentation/cmd/gpubench`, `experimentation/results/rq6_breakeven.csv` | Not valid as main-paper evidence because GPU/self-hosted serving was not measured. |

## 2. Manuscript Source Location

The canonical manuscript for resubmission is `experimentation/paper/latex/main.tex`. Supporting Markdown drafts (`paper.md`, `paper.tex`, `methodology.md`, `tables.md`, `threats_to_validity.md`) contain older claims and must not be treated as authoritative until synchronized.

## 3. Operator Modules and Responsibilities

| Module | Responsibility | Evidence/Status |
|---|---|---|
| `operateur/internal/costengine` | Computes EUR token cost from `collectors.UsageSample` and a model price book; aggregates by model, provider, namespace, team, and application. | Implemented and unit-tested. Used by reports and the experimentation harness for cost attribution. |
| `operateur/internal/budgetengine` | Maps projected spend to phases `WithinBudget`, `Warning`, `Critical`, `Exceeded` using configured warning/critical/hard-limit percentages and advisory actions. | Implemented and used by `AIBudgetPolicyReconciler`. |
| `operateur/internal/controller/aibudgetpolicy_controller.go` | Collects telemetry, computes projected monthly spend, evaluates budget phase, emits advisory action metrics, and may actuate a configured fallback model when enforcement and safety guardrails allow it. | Implemented; live paper evidence is integration verification, not broad production evaluation. |
| `operateur/internal/sovereigntyengine` | Evaluates declared provider zones, forbidden/allowed zones, and sensitive-data external-provider policy. Emits findings; does not certify legal residency. | Implemented and unit-tested. Claims must say declared-policy filtering, not legal sovereignty certification. |
| `operateur/internal/routingscore` | Runtime score per namespace/application/model/provider. Higher-is-better in [0,1]. Defaults: cost 0.40, quality 0.30, latency 0.20, reliability 0.10. Sovereignty is a hard gate with score 0 for non-compliant models. | Implemented and reported by `AIFinOpsReport` / `AIRoutingPolicy`. |
| `operateur/internal/qualityengine` | Pure composite quality score from golden evidence and real telemetry. Defaults: correctness 0.40, reliability 0.20, latency 0.15, semantic 0.15, judged 0.10, with judge weight disabled when no judge is used. Missing weighted signals produce `insufficient-data`. | Implemented; not fully evaluated as a paper result. Must be in implementation/status table. |
| `operateur/pkg/qualitystats` | Non-inferiority machinery, composite operational score, and hysteresis. Defaults: delta 0.05, confidence 95%, power 80%, baseline success 0.90; required samples per arm 446 under defaults. | Implemented and unit-tested; not a main experimental result. |
| `operateur/internal/qualityeval` | Runs golden prompts against the production gateway and serializes evidence for `qualityengine`. | Implemented; paper may describe as an implemented pipeline, not as a broad evaluated outcome unless logs/results are cited. |
| `operateur/internal/collectors/aigw` | Scrapes Envoy AI Gateway OpenTelemetry Prometheus metrics (`gen_ai_client_token_usage`, `gen_ai_server_request_duration_seconds`) and attributes usage to workloads via headers or AIModel catalog. | Implemented and unit-tested. Integration run verifies telemetry ingestion. |
| `operateur/internal/shadowengine` and `controller/shadow.go` | Ingests gateway-independent egress observations from a `shadow-egress` ConfigMap, classifies known LLM endpoints via catalog mapping, and emits shadow-AI findings. | Detection/reporting only; not prevention. |
| `operateur/internal/gatewayreroute` | Mutates Envoy AI Gateway route rules for reroute/block controls, preserving reversibility. | Implemented and unit-tested; paper should avoid claiming large-scale production failover evaluation. |
| `operateur/internal/breakevenengine` | Analytical managed-vs-self-hosted economics. | Implemented but not supported by measured GPU/vLLM experiments; remove from paper results. |

## 4. Experiment Scripts and Result Files

| File | Evidence Type | Use in Revised Paper |
|---|---|---|
| `experimentation/results/rq1_cost.csv` | CACHED replay of previously collected real API responses, with some old baselines contaminated by modeled self-hosted rows. | Keep B1, B4, B5, B6 values; avoid baselines that select `selfhosted-eu-llama` in main conclusions. |
| `experimentation/results/rq2_quality.csv` | CACHED LLM-as-judge quality for open-ended synthetic workloads. | Keep with scoped wording: open-ended judge-assessed quality proxy only. |
| `experimentation/results/rq3_latency.csv` | CACHED/MEASURED call latencies in the replay records. | Keep latency increase; do not hide mean/tail latency penalty. |
| `experimentation/results/rq4_sovereignty.csv` | SIMULATED declared-policy scenarios over catalog metadata and routing decisions. | Reframe as cost/availability/quality trade-offs under declared policy; zero configured-policy violations are by construction. Exclude self-hosted-only claims from main paper. |
| `experimentation/results/rq5_budget.csv` | SIMULATED budget pressure over existing workload rows. | Keep as a bounded simulated scenario. |
| `experimentation/results/rq6_breakeven.csv` | MODELED GPU/self-hosted economics. | Remove from main paper. |
| `experimentation/results/rq8_ablation.csv` | CACHED/SIMULATED ablation over the routing score terms. | Keep if clearly scoped and no break-even/self-hosted conclusion is attached. |
| `experimentation/results-stats/calls_stats.csv` and `stats_summary.md` | MEASURED 30-repetition uncached run over fixed workload matrix. | Keep as stability analysis; downplay p-values/effect sizes because workload and prices are fixed. |
| `experimentation/results-bench/rq_benchmark.csv` | CACHED exact-match benchmark on 250 GSM8K + 250 MMLU items. | Keep as objective accuracy/cost trade-off evidence. |
| `experimentation/results/judge_agreement_summary.md` | MEASURED/CACHED inter-judge agreement over 120 scored items. | Keep as moderate/weak agreement caveat; do not claim strong judge validity. |
| `experimentation/results/judge_vs_truth_summary.md` | MEASURED/CACHED judge-vs-ground-truth analysis over 300 objective items. | Keep as methodological warning that judge scores over-rate incorrect answers. |
| `experimentation/results-live-kind-*` | MEASURED ephemeral kind integration logs. | Use only as integration verification for telemetry, attribution, quality jobs, and shadow-AI ingestion; not as security/production evaluation. |
| `experimentation/cmd/gpubench` and `results-bench/gpubench.csv` | GPU/vLLM benchmarking utility/results placeholder. | Future work or artifact only; not a main result. |

## 5. Major Manuscript Claims and Evidence Decisions

| Claim | Evidence Type | Source | Decision |
|---|---|---|---|
| Kubernetes-native gateway control plane represents providers, models, budgets, policies, reports, routing and quality resources as CRDs. | MEASURED/IMPLEMENTED | `operateur/api/v1alpha1`, controllers | Keep as implementation claim. |
| The operator attributes cost by workload/team/provider/model from token usage. | MEASURED/IMPLEMENTED | `costengine`, `collectors`, `AIFinOpsReport`, `rq1_cost_by_team.csv` | Keep. |
| The proposed policy reduces cached replay cost by 70.8% vs premium-static. | CACHED | `results/rq1_cost.csv`, B6 savings 70.84% | Keep, bounded to the evaluated workload mix and committed catalog prices. |
| Open-ended quality is preserved generally. | UNSUPPORTED | Contradicted by `results-bench/rq_benchmark.csv` | Remove. Use scoped wording only: open-ended judge-assessed quality remained comparable in the evaluated prompt set. |
| Objective GSM8K/MMLU accuracy drops under cost-aware routing. | CACHED | `results-bench/rq_benchmark.csv`: B1 65.40%, B6 47.20% | Keep prominently. |
| 30-repetition run proves enterprise-scale statistical significance. | UNSUPPORTED/OVERCLAIMED | `results-stats/stats_summary.md` fixed workload matrix | Weaken. Treat as run-to-run stability under fixed matrix. |
| Declared-policy filtering prevents configured-policy violations. | SIMULATED/IMPLEMENTED | `sovereigntyengine`, `rq4_sovereignty.csv` | Keep as by-construction configured-policy compliance under declared metadata. Do not call it legal sovereignty. |
| The operator legally certifies sovereignty or data residency. | UNSUPPORTED | No code/legal verification | Remove. Add explicit non-certification sentence. |
| Budget graceful degradation dominates hard blocking. | SIMULATED | `rq5_budget.csv` | Weaken: better availability/overrun trade-off in one simulated pressure scenario. |
| Envoy AI Gateway telemetry is ingested and attributed by workload. | MEASURED/IMPLEMENTED | `collectors/aigw`, kind logs/metrics | Keep as integration verification. |
| Tetragon/shadow-AI prevents bypass traffic. | UNSUPPORTED | `shadowengine` only detects/reports | Rewrite: evidence ingestion/detection of direct egress to known LLM endpoints. |
| Production failover and large-scale enforcement were evaluated. | UNSUPPORTED | No production trace/load test | Remove or list as not evaluated. |
| Managed-vs-self-hosted GPU break-even was evaluated. | UNSUPPORTED/MODELED | `rq6_breakeven.csv`, `breakevenengine`, `cmd/gpubench` not used for measured vLLM serving | Delete from main paper. Future-work sentence only. |
| B7 is a RouteLLM/Hybrid LLM baseline. | UNSUPPORTED | `DifficultyRouter` is a heuristic using token length and minQuality, not public RouteLLM/Hybrid checkpoints. | Rename to difficulty-proxy heuristic or remove from main results. |

## 6. Text/Code Mismatches to Fix

1. The paper says “hard sovereignty” and “zero sovereignty violations”; the code enforces declared metadata filters and reports findings. Use “zero configured-policy violations under declared provider metadata.”
2. The paper describes a single scoring formula. The experimentation harness uses an argmin formula with alpha/beta/gamma/delta/epsilon, while the operator runtime uses `routingscore.Compute`, an argmax weighted score in [0,1] with cost/quality/latency/reliability and a hard sovereignty gate. The manuscript must state which is evaluated where.
3. The paper implies budget hard-block/graceful degradation is production-grade. The operator has advisory actions and guarded fallback actuation, while the paper results are a simulated budget pressure scenario. Scope must be clear.
4. The paper claims or implies live enforcement in some places. Integration evidence verifies telemetry, attribution, route mutation logic, and shadow evidence ingestion; it is not a broad production enforcement study.
5. `DifficultyRouter` is a transparent heuristic, not RouteLLM or Hybrid LLM.
6. Old RQ4 France-only/self-hosted-only rows depend on a stubbed self-hosted model. Do not use them as main-paper results.
7. Old RQ6 break-even is modeled economics and must be removed from the manuscript, figures, sensitivity tables, contributions, abstract, conclusion, and data availability.
8. The inter-judge summary currently says kappa/alpha support the judge; values around 0.40/0.39 are moderate/weak and must be presented as a limitation.

## 7. GPU/Self-Hosted/Break-Even Content to Delete From the Paper

Delete main-paper references to:

- managed-vs-self-hosted arbitrage;
- self-hosting break-even;
- GPU/vLLM measured or modeled economics;
- payback and tokens/day crossover;
- RQ6;
- Figure 7 break-even;
- break-even sensitivity rows;
- self-host recommendation language;
- claims that the artifact reproduces GPU economics as a result.

Allowed future-work sentence only:

“Future work will evaluate real self-hosted GPU serving and hosting economics.”

## 8. Revised Evidence Boundary

The paper should use only three main evidence labels:

| Label | Meaning |
|---|---|
| MEASURED | Produced by live calls, live integration logs, uncached API repetitions, or real telemetry ingestion. |
| CACHED | Reanalysis/replay of previously collected real API calls and judge responses. |
| SIMULATED | Deterministic policy/budget scenarios over declared metadata and existing result rows. |

`MODELED` remains in the repository for legacy GPU economics artifacts but must not appear as a main evidence class in the revised paper.
