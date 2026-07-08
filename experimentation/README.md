# Governance-Aware LLM Routing Experiments

This directory contains the reproducible artifact for the revised networked LLM
gateway-control-plane paper.

The current paper scope is intentionally bounded:

- Kubernetes-native governance control plane for LLM gateway routing.
- Cost attribution by workload, namespace, team, provider, and model.
- Declared-residency policy filtering over catalog metadata.
- Budget-pressure behavior and guarded degradation scenarios.
- Envoy AI Gateway telemetry attribution.
- Tetragon/shadow-AI evidence ingestion.
- Evidence separation: `MEASURED`, `CACHED`, and `SIMULATED`.

The canonical manuscript source is:

```bash
paper/latex/main.tex
```

## Reproduce Analyses

```bash
cd experimentation
python3 scripts/analyze_results.py --results results --figures figures
python3 scripts/bootstrap_items.py --calls results/calls.csv
python3 scripts/objective_guardrail.py
python3 scripts/analyze_stats.py
python3 scripts/judge_agreement.py
python3 scripts/judge_vs_truth.py
```

The main CSV outputs are under `results/`, `results-stats/`, and
`results-bench/`. Provider credentials are not stored in this repository.

## Evidence Boundary

`kind` and local integration logs are used only for CI/debug/integration
verification. They are not presented as security-evaluation results.

The revised paper does not present unmeasured hosting-economics comparisons as a
main result. Legacy analytical code, if present, is documented in
`paper_claims_audit.md` and excluded from the manuscript claims.
