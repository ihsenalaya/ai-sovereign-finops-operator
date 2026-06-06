# Tables

Numbers are generated, not hand-copied: `results/summary.md` (auto-built by
`scripts/analyze_results.py`) always reflects the latest run, and the raw data lives in
`results/*.csv`. Below are the LaTeX skeletons to drop into the paper; fill cells from the CSVs.

## Table 1 — Cost by routing strategy (RQ1)
Source: `results/rq1_cost.csv`
```latex
\begin{tabular}{lrrrr}
\toprule
Strategy & Total cost (EUR) & Cost/req & Cost/token & Savings vs premium \\
\midrule
B1 premium-static & ... & ... & ... & 0\% \\
B2 round-robin    & ... & ... & ... & ...\% \\
B3 least-cost     & ... & ... & ... & ...\% \\
B4 static-policy  & ... & ... & ... & ...\% \\
B5 budget-block   & ... & ... & ... & ...\% \\
B6 ours           & ... & ... & ... & \textbf{...\%} \\
\bottomrule
\end{tabular}
```

## Table 2 — Quality preservation (RQ2)
Source: `results/rq2_quality.csv` — mean quality (norm), acceptable-rate, win-rate vs premium.

## Table 3 — Latency & routing overhead (RQ3)
Source: `results/rq3_latency.csv` — p50/p95/p99 (ms) + routing decision (µs); CIs from
`scripts/analyze_results.py` (bootstrap).

## Table 4 — Sovereignty scenarios (RQ4)
Source: `results/rq4_sovereignty.csv` — cost, served, blocked, violations, reroutes, quality for
B1 (sovereignty-blind) vs B6 (Ours) across {global, eu-only, france-only, no-external-sensitive,
self-hosted-only}.

## Table 5 — Budget policies (RQ5)
Source: `results/rq5_budget.csv` — availability, overrun, cost, quality for {alert-only, hard-block,
ours-graceful}.

## Table 6 — Break-even (RQ6, modeled)
Source: `results/rq6_breakeven.csv` — managed vs self-hosted monthly cost, savings, payback,
recommendation across daily token volumes.

## Table 7 — Ablation
Source: `results/rq8_ablation.csv` — cost & quality for {full, no-cost-term, no-quality-term,
no-latency-term}.
