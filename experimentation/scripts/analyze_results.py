#!/usr/bin/env python3
"""Analyze experiment results and produce publication-ready figures + summary.

Reads results/*.csv (produced by the Go runner), computes aggregates and 95%
confidence intervals (bootstrap on per-call latency), and writes figures/*.png
and results/summary.md.

Usage: python3 scripts/analyze_results.py [--results results] [--figures figures]
"""
import argparse
import os
import numpy as np
import pandas as pd
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

plt.rcParams.update({"figure.dpi": 150, "font.size": 10, "axes.grid": True, "grid.alpha": 0.3})


def boot_ci(x, n=2000, alpha=0.05, seed=42):
    x = np.asarray(x, dtype=float)
    if len(x) == 0:
        return (0.0, 0.0)
    rng = np.random.default_rng(seed)
    means = [rng.choice(x, size=len(x), replace=True).mean() for _ in range(n)]
    return (float(np.percentile(means, 100 * alpha / 2)), float(np.percentile(means, 100 * (1 - alpha / 2))))


def savefig(fig, path):
    fig.tight_layout()
    fig.savefig(path, bbox_inches="tight")
    plt.close(fig)
    print("wrote", path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--results", default="results")
    ap.add_argument("--figures", default="figures")
    args = ap.parse_args()
    R, F = args.results, args.figures
    os.makedirs(F, exist_ok=True)

    rq1 = pd.read_csv(f"{R}/rq1_cost.csv")
    rq2 = pd.read_csv(f"{R}/rq2_quality.csv")
    rq3 = pd.read_csv(f"{R}/rq3_latency.csv")
    rq4 = pd.read_csv(f"{R}/rq4_sovereignty.csv")
    rq5 = pd.read_csv(f"{R}/rq5_budget.csv")
    rq6 = pd.read_csv(f"{R}/rq6_breakeven.csv")
    calls = pd.read_csv(f"{R}/calls.csv")
    abl = pd.read_csv(f"{R}/rq8_ablation.csv") if os.path.exists(f"{R}/rq8_ablation.csv") else None

    summary = ["# Experiment summary\n"]

    # ---- Figure 2: total cost by strategy ----
    fig, ax = plt.subplots(figsize=(7, 4))
    ax.bar(rq1.strategy, rq1.total_cost_eur, color="#3b6ea5")
    for i, (s, c) in enumerate(zip(rq1.savings_vs_premium_pct, rq1.total_cost_eur)):
        ax.text(i, c, f"-{s:.0f}%", ha="center", va="bottom", fontsize=8)
    ax.set_ylabel("Total cost (EUR)")
    ax.set_title("Figure 2 — Total cost by routing strategy")
    ax.tick_params(axis="x", rotation=30)
    savefig(fig, f"{F}/fig2_cost_by_strategy.png")
    summary.append("## RQ1 Cost\n" + rq1.to_markdown(index=False) + "\n")

    # ---- Figure 3: quality vs cost ----
    m = rq1.merge(rq2, on="strategy")
    fig, ax = plt.subplots(figsize=(6, 5))
    ax.scatter(m.total_cost_eur, m.mean_quality_norm, s=60, color="#a53b3b")
    for _, r in m.iterrows():
        ax.annotate(r.strategy, (r.total_cost_eur, r.mean_quality_norm), fontsize=7,
                    xytext=(4, 4), textcoords="offset points")
    ax.set_xlabel("Total cost (EUR)")
    ax.set_ylabel("Mean quality (normalized 0-1)")
    ax.set_title("Figure 3 — Quality vs cost")
    savefig(fig, f"{F}/fig3_quality_vs_cost.png")
    summary.append("## RQ2 Quality\n" + rq2.to_markdown(index=False) + "\n")

    # ---- Figure 4: latency p95 with bootstrap CI ----
    served = calls[(calls.blocked == False) & (calls.latency_ms > 0)]
    strategies = list(rq3.strategy)
    p95s, los, his = [], [], []
    for s in strategies:
        lat = served[served.strategy == s].latency_ms.values
        p95s.append(np.percentile(lat, 95) if len(lat) else 0)
        lo, hi = boot_ci(lat)
        los.append(lo); his.append(hi)
    fig, ax = plt.subplots(figsize=(7, 4))
    ax.bar(strategies, p95s, color="#3b8f5a")
    # CI shown on the mean (bootstrap), annotated
    ax.errorbar(strategies, [np.mean(served[served.strategy == s].latency_ms) for s in strategies],
                yerr=[[np.mean(served[served.strategy == s].latency_ms) - lo for s, lo in zip(strategies, los)],
                      [hi - np.mean(served[served.strategy == s].latency_ms) for s, hi in zip(strategies, his)]],
                fmt="o", color="black", capsize=3, label="mean ±95% CI")
    ax.set_ylabel("Latency (ms)")
    ax.set_title("Figure 4 — Latency p95 (bars) and mean ±95% CI (points)")
    ax.tick_params(axis="x", rotation=30)
    ax.legend()
    savefig(fig, f"{F}/fig4_latency.png")
    summary.append("## RQ3 Latency\n" + rq3.to_markdown(index=False) + "\n")

    # ---- Figure 5: sovereignty impact ----
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(11, 4))
    piv_c = rq4.pivot(index="scenario", columns="strategy", values="total_cost_eur")
    piv_c.plot(kind="bar", ax=a1, color=["#a53b3b", "#3b6ea5"])
    a1.set_ylabel("Total cost (EUR)"); a1.set_title("Cost per sovereignty scenario")
    a1.tick_params(axis="x", rotation=20)
    piv_v = rq4.pivot(index="scenario", columns="strategy", values="violations")
    piv_v.plot(kind="bar", ax=a2, color=["#a53b3b", "#3b6ea5"])
    a2.set_ylabel("Sovereignty violations"); a2.set_title("Violations per scenario")
    a2.tick_params(axis="x", rotation=20)
    fig.suptitle("Figure 5 — Sovereignty: cost & violations (B1 vs Ours)")
    savefig(fig, f"{F}/fig5_sovereignty.png")
    summary.append("## RQ4 Sovereignty\n" + rq4.to_markdown(index=False) + "\n")

    # ---- Figure 6: budget hard block vs graceful ----
    fig, ax = plt.subplots(figsize=(7, 4))
    x = np.arange(len(rq5)); w = 0.4
    ax.bar(x - w/2, rq5.availability_pct, width=w, label="availability %", color="#3b8f5a")
    ax.bar(x + w/2, rq5.budget_overrun_pct, width=w, label="budget overrun %", color="#a53b3b")
    ax.set_xticks(x); ax.set_xticklabels(rq5.policy)
    ax.set_title("Figure 6 — Budget policy: availability vs overrun")
    ax.legend()
    savefig(fig, f"{F}/fig6_budget.png")
    summary.append("## RQ5 Budget\n" + rq5.to_markdown(index=False) + "\n")

    # ---- Figure 7: break-even curve ----
    fig, ax = plt.subplots(figsize=(7, 4))
    ax.plot(rq6.tokens_per_day / 1e6, rq6.managed_monthly_eur, marker="o", label="managed (API)")
    ax.plot(rq6.tokens_per_day / 1e6, rq6.selfhosted_monthly_eur, marker="s", label="self-hosted (modeled)")
    ax.set_xscale("log"); ax.set_xlabel("Tokens/day (millions, log)"); ax.set_ylabel("Monthly cost (EUR)")
    ax.set_title("Figure 7 — Managed vs self-hosted break-even")
    ax.legend()
    savefig(fig, f"{F}/fig7_breakeven.png")
    summary.append("## RQ6 Break-even (modeled)\n" + rq6.head(20).to_markdown(index=False) + "\n")

    # ---- Figure 8: ablation ----
    if abl is not None:
        fig, ax = plt.subplots(figsize=(7, 4))
        x = np.arange(len(abl)); w = 0.4
        ax.bar(x - w/2, abl.total_cost_eur, width=w, label="cost (EUR)", color="#3b6ea5")
        ax2 = ax.twinx()
        ax2.plot(x, abl.mean_quality_norm, "o-", color="#a53b3b", label="quality (norm)")
        ax.set_xticks(x); ax.set_xticklabels(abl.variant, rotation=20)
        ax.set_ylabel("cost (EUR)"); ax2.set_ylabel("quality (norm)")
        ax.set_title("Figure 8 — Ablation of scoring terms")
        savefig(fig, f"{F}/fig8_ablation.png")
        summary.append("## Ablation\n" + abl.to_markdown(index=False) + "\n")

    # ---- Headline numbers ----
    ours = rq1[rq1.strategy == "B6-ours"].iloc[0]
    ours_q = rq2[rq2.strategy == "B6-ours"].iloc[0]
    prem_q = rq2[rq2.strategy == "B1-premium-static"].iloc[0]
    qdrop = (prem_q.mean_quality_norm - ours_q.mean_quality_norm) / prem_q.mean_quality_norm * 100
    summary.insert(1, f"\n**Headline:** Ours reduces cost by **{ours.savings_vs_premium_pct:.1f}%** vs premium-static "
                      f"with a quality change of **{qdrop:.2f}%** (win-rate vs premium {ours_q.winrate_vs_premium_pct:.1f}%).\n")

    with open(f"{R}/summary.md", "w") as fh:
        fh.write("\n".join(summary))
    print("wrote", f"{R}/summary.md")


if __name__ == "__main__":
    main()
