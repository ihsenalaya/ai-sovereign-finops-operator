# Paper outline

**Working title:** Economic-Aware and Sovereignty-Constrained Routing for Enterprise LLM Gateways

**Target:** Q1 venue (systems / cloud / services). The Kubernetes operator (AI Sovereign FinOps
Operator) is the experimental artifact that implements and validates the approach; the contribution
is the *method*, not the tool.

## Abstract
Enterprises adopting LLMs lack a principled way to control cost, attribute spend, respect data
sovereignty, and decide between managed APIs and self-hosting. We present an economic-aware,
sovereignty-constrained, budget-aware routing control plane placed at the gateway. We formalize the
routing decision as a constrained scoring problem and evaluate it on a reproducible testbed with
real LLM calls across four enterprise workloads and six routing strategies. Our approach reduces
operational cost by X% relative to premium-static routing while preserving response quality within
the target degradation threshold, enforces declarative sovereignty constraints with zero violations,
sustains availability under budget pressure via graceful degradation, and predicts managed-vs-self-
hosted break-even from runtime telemetry.

## 1. Introduction
- Motivation: cost opacity, sovereignty/compliance, managed-vs-self-hosted arbitrage.
- Gap: existing tooling measures/report; lacks an actionable, constraint-aware control plane.
- Contributions:
  1. A constrained scoring formulation for LLM routing (cost, quality, latency, sovereignty, budget).
  2. A Kubernetes/Envoy control-plane realization (the operator) with FinOps + sovereignty engines.
  3. A reproducible evaluation methodology and testbed with real LLM telemetry.
  4. Empirical evidence across RQ1-RQ6 (cost, quality, latency, sovereignty, budget, break-even).

## 2. Background & Related Work
- LLM gateways (Envoy AI Gateway, LiteLLM), model routing / cascades, FinOps, data sovereignty,
  serving economics (managed APIs vs vLLM/TGI self-hosting). Position vs prior routing work.

## 3. Problem Formulation & Algorithm
- Inputs: request metadata, model/provider catalog, pricing, sovereignty policy, budget state,
  quality/latency/availability priors.
- Hard constraints: reject any model violating sovereignty (forbidden/allowed zones, sensitive-data
  rules). Output: selected model+provider, reason, expected cost, findings, fallback.
- Scoring: `score(m) = α·norm_cost·(1+ε·budget_pressure) + β·quality_loss + γ·latency_penalty + δ·sov_risk`.
- Pseudocode (see methodology.md §Algorithm).

## 4. System Design
- Envoy AI gateway in the data path; operator as control plane with Cost/Budget/Sovereignty/
  Break-even/Report engines (CRDs). Figure 1 (HLD).

## 5. Experimental Methodology
- Testbed, workloads W1-W4, baselines B1-B6, metrics, repetitions, statistics. See methodology.md.

## 6. Results
- RQ1 Cost (Fig 2), RQ2 Quality (Fig 3), RQ3 Latency/overhead (Fig 4), RQ4 Sovereignty (Fig 5),
  RQ5 Budget degradation (Fig 6), RQ6 Break-even (Fig 7), Ablation (Fig 8). Numbers from
  `results/summary.md` and `results/*.csv`.

## 7. Discussion
- When economic routing helps most; sovereignty cost; budget strategy trade-offs; self-hosting
  break-even sensitivity.

## 8. Threats to Validity
See threats_to_validity.md.

## 9. Conclusion & Future Work
- Second provider (cross-provider), GPU self-hosting on Azure (real vLLM), live Envoy data-path
  enforcement, larger-scale repetitions.

## Artifacts / Reproducibility
- Operator repo + `experimentation/` (datasets, harness, scripts, results, figures). All LLM calls
  cached for reproducibility; one-command rerun.
