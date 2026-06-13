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

plt.rcParams.update({
    "figure.dpi": 200,
    "savefig.dpi": 300,
    "font.family": "DejaVu Sans",
    "font.size": 8,
    "axes.labelsize": 8,
    "axes.titlesize": 8,
    "xtick.labelsize": 7,
    "ytick.labelsize": 7,
    "legend.fontsize": 7,
    "axes.grid": True,
    "grid.alpha": 0.22,
    "grid.linewidth": 0.6,
    "axes.spines.top": False,
    "axes.spines.right": False,
})

BLUE = "#2f6f9f"
RED = "#a83f3f"
GREEN = "#3d8b5a"
ORANGE = "#d98c2b"


def compact_label(label):
    mapping = {
        "B1-premium-static": "Premium",
        "B2-round-robin": "Round robin",
        "B3-least-cost": "Least cost",
        "B4-static-policy": "Static",
        "B5-budget-hard-block": "Hard block",
        "B6-ours": "Ours",
        "full-system": "Full",
        "no-cost-term": "No cost",
        "no-quality-term": "No quality",
        "no-latency-term": "No latency",
    }
    return mapping.get(str(label), str(label))


def boot_ci(x, n=2000, alpha=0.05, seed=42):
    x = np.asarray(x, dtype=float)
    if len(x) == 0:
        return (0.0, 0.0)
    rng = np.random.default_rng(seed)
    means = [rng.choice(x, size=len(x), replace=True).mean() for _ in range(n)]
    return (float(np.percentile(means, 100 * alpha / 2)), float(np.percentile(means, 100 * (1 - alpha / 2))))


def savefig(fig, path):
    fig.tight_layout(pad=0.4)
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
    fig, ax = plt.subplots(figsize=(3.45, 2.35))
    labels = [compact_label(s) for s in rq1.strategy]
    ax.bar(labels, rq1.total_cost_eur, color=BLUE)
    for i, (s, c) in enumerate(zip(rq1.savings_vs_premium_pct, rq1.total_cost_eur)):
        ax.text(i, c, f"-{s:.0f}%", ha="center", va="bottom", fontsize=8)
    ax.set_ylabel("Total cost (EUR)")
    ax.tick_params(axis="x", rotation=25)
    savefig(fig, f"{F}/fig2_cost_by_strategy.png")
    summary.append("## RQ1 Cost\n" + rq1.to_markdown(index=False) + "\n")

    # ---- Figure 3: quality vs cost ----
    m = rq1.merge(rq2, on="strategy")
    fig, ax = plt.subplots(figsize=(3.45, 2.55))
    ax.scatter(m.total_cost_eur, m.mean_quality_norm, s=28, color=RED)
    for _, r in m.iterrows():
        ax.annotate(compact_label(r.strategy), (r.total_cost_eur, r.mean_quality_norm), fontsize=6.5,
                    xytext=(4, 4), textcoords="offset points")
    ax.set_xlabel("Total cost (EUR)")
    ax.set_ylabel("Mean quality")
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
    fig, ax = plt.subplots(figsize=(3.45, 2.35))
    labels = [compact_label(s) for s in strategies]
    ax.bar(labels, p95s, color=GREEN)
    # CI shown on the mean (bootstrap), annotated
    ax.errorbar(labels, [np.mean(served[served.strategy == s].latency_ms) for s in strategies],
                yerr=[[np.mean(served[served.strategy == s].latency_ms) - lo for s, lo in zip(strategies, los)],
                      [hi - np.mean(served[served.strategy == s].latency_ms) for s, hi in zip(strategies, his)]],
                fmt="o", color="black", capsize=3, label="mean ±95% CI")
    ax.set_ylabel("Latency (ms)")
    ax.tick_params(axis="x", rotation=25)
    ax.legend(frameon=False)
    savefig(fig, f"{F}/fig4_latency.png")
    summary.append("## RQ3 Latency\n" + rq3.to_markdown(index=False) + "\n")

    # ---- Figure 5: sovereignty impact ----
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(6.9, 2.4))
    piv_c = rq4.pivot(index="scenario", columns="strategy", values="total_cost_eur")
    piv_c.rename(columns=compact_label).plot(kind="bar", ax=a1, color=[RED, BLUE])
    a1.set_ylabel("Total cost (EUR)"); a1.set_title("Cost")
    a1.tick_params(axis="x", rotation=20)
    piv_v = rq4.pivot(index="scenario", columns="strategy", values="violations")
    piv_v.rename(columns=compact_label).plot(kind="bar", ax=a2, color=[RED, BLUE])
    a2.set_ylabel("Violations"); a2.set_title("Violations")
    a2.tick_params(axis="x", rotation=20)
    for axx in (a1, a2):
        axx.legend(frameon=False)
    savefig(fig, f"{F}/fig5_sovereignty.png")
    summary.append("## RQ4 Sovereignty\n" + rq4.to_markdown(index=False) + "\n")

    # ---- Figure 6: budget hard block vs graceful ----
    fig, ax = plt.subplots(figsize=(3.45, 2.35))
    x = np.arange(len(rq5)); w = 0.4
    ax.bar(x - w/2, rq5.availability_pct, width=w, label="availability", color=GREEN)
    ax.bar(x + w/2, rq5.budget_overrun_pct, width=w, label="overrun", color=RED)
    ax.set_xticks(x); ax.set_xticklabels(rq5.policy)
    ax.set_ylabel("%")
    ax.legend(frameon=False)
    savefig(fig, f"{F}/fig6_budget.png")
    summary.append("## RQ5 Budget\n" + rq5.to_markdown(index=False) + "\n")

    # ---- Figure 7: break-even curve ----
    fig, ax = plt.subplots(figsize=(3.45, 2.35))
    x = rq6.tokens_per_day / 1e6
    ax.plot(x, rq6.managed_monthly_eur, marker="o", markersize=3.5, linewidth=1.4,
            color=BLUE, label="Managed API")
    ax.plot(x, rq6.selfhosted_monthly_eur, marker="s", markersize=3.2, linewidth=1.4,
            color=ORANGE, label="Self-hosted model")
    modeled_break_even = float(rq6.selfhosted_monthly_eur.iloc[0] / rq6.managed_monthly_eur.iloc[0])
    ax.axvline(modeled_break_even, color="0.35", linestyle="--", linewidth=0.9)
    ax.annotate("break-even\n~18M/day", xy=(modeled_break_even, rq6.selfhosted_monthly_eur.iloc[0]),
                xytext=(10, 10), textcoords="offset points", fontsize=6.5,
                arrowprops={"arrowstyle": "-", "lw": 0.6, "color": "0.35"})
    ax.set_xscale("log")
    ax.set_xlabel("Tokens/day (M, log)")
    ax.set_ylabel("Monthly cost (EUR)")
    ax.legend(frameon=False, loc="upper left")
    savefig(fig, f"{F}/fig7_breakeven.png")
    summary.append("## RQ6 Break-even (modeled)\n" + rq6.head(20).to_markdown(index=False) + "\n")

    # ---- Figure 8: ablation ----
    if abl is not None:
        fig, ax = plt.subplots(figsize=(3.45, 2.35))
        x = np.arange(len(abl)); w = 0.4
        labels = [compact_label(v) for v in abl.variant]
        ax.bar(x - w/2, abl.total_cost_eur, width=w, label="cost", color=BLUE)
        ax2 = ax.twinx()
        ax2.plot(x, abl.mean_quality_norm, "o-", color=RED, linewidth=1.4,
                 markersize=3.5, label="quality")
        ax.set_xticks(x); ax.set_xticklabels(labels, rotation=20)
        ax.set_ylabel("Cost (EUR)"); ax2.set_ylabel("Quality")
        lines, labels1 = ax.get_legend_handles_labels()
        lines2, labels2 = ax2.get_legend_handles_labels()
        ax.legend(lines + lines2, labels1 + labels2, frameon=False, loc="upper center",
                  ncol=2, bbox_to_anchor=(0.52, 1.05))
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
