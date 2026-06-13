#!/usr/bin/env python3
"""Statistical analysis of the multi-repetition run (calls_stats.csv).

Aggregates per (strategy, rep), then across reps computes mean ± 95% CI for cost,
quality and latency per strategy, and tests each strategy vs premium-static
(Mann-Whitney U on per-rep means, Cliff's delta, Cohen's d). Produces
publication-ready figures with error bars and results/stats_summary.md.

Usage: python3 scripts/analyze_stats.py [--calls results-stats/calls_stats.csv] [--out figures] [--results results-stats]
"""
import argparse
import os
import numpy as np
import pandas as pd
from scipy import stats
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
BASE = "B1-premium-static"
BLUE = "#2f6f9f"
RED = "#a83f3f"
GREEN = "#3d8b5a"


def compact_label(label):
    mapping = {
        "B1-premium-static": "Premium",
        "B2-round-robin": "Round robin",
        "B3-least-cost": "Least cost",
        "B4-static-policy": "Static",
        "B5-budget-hard-block": "Hard block",
        "B6-ours": "Ours",
    }
    return mapping.get(str(label), str(label))


def ci95(x):
    x = np.asarray(x, float)
    if len(x) < 2:
        return (float(np.mean(x)) if len(x) else 0.0, 0.0, 0.0)
    m = x.mean(); se = x.std(ddof=1) / np.sqrt(len(x))
    h = se * stats.t.ppf(0.975, len(x) - 1)
    return m, m - h, m + h


def cliffs_delta(a, b):
    a, b = np.asarray(a, float), np.asarray(b, float)
    gt = sum((x > b).sum() for x in a); lt = sum((x < b).sum() for x in a)
    return (gt - lt) / (len(a) * len(b)) if len(a) and len(b) else float("nan")


def cohens_d(a, b):
    a, b = np.asarray(a, float), np.asarray(b, float)
    if len(a) < 2 or len(b) < 2:
        return float("nan")
    sp = np.sqrt(((len(a)-1)*a.var(ddof=1) + (len(b)-1)*b.var(ddof=1)) / (len(a)+len(b)-2))
    return float((a.mean()-b.mean())/sp) if sp > 0 else float("nan")


def per_rep(df, metric, agg="sum"):
    """Return {strategy: [per-rep value]} for a metric (sum for cost, mean otherwise)."""
    g = df.groupby(["strategy", "rep"])[metric]
    s = g.sum() if agg == "sum" else g.mean()
    out = {}
    for (strat, _), v in s.items():
        out.setdefault(strat, []).append(v)
    return out


def barcih(ax, order, data, xlabel, color):
    means, los, his = [], [], []
    for s in order:
        m, lo, hi = ci95(data.get(s, [0]))
        means.append(m); los.append(m - lo); his.append(hi - m)
    y = np.arange(len(order))
    ax.barh(y, means, xerr=[los, his], capsize=3, color=color)
    ax.set_yticks(y)
    ax.set_yticklabels([compact_label(s) for s in order])
    ax.invert_yaxis()
    ax.set_xlabel(xlabel)
    ax.grid(axis="x", alpha=0.22)
    ax.grid(axis="y", visible=False)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--calls", default="results-stats/calls_stats.csv")
    ap.add_argument("--out", default="figures")
    ap.add_argument("--results", default="results-stats")
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)

    df = pd.read_csv(args.calls)
    df["quality_norm"] = (df["quality_1to5"] - 1) / 4
    reps = sorted(df["rep"].unique())
    order = [s for s in ["B1-premium-static", "B2-round-robin", "B3-least-cost",
                         "B4-static-policy", "B5-budget-hard-block", "B6-ours"] if s in set(df.strategy)]

    cost = per_rep(df, "cost_eur", "sum")          # total cost per rep
    qual = per_rep(df, "quality_norm", "mean")     # mean quality per rep
    lat = per_rep(df[df.latency_ms > 0], "latency_ms", "mean")  # mean latency per rep

    # Figures with 95% CI error bars.
    fig, ax = plt.subplots(figsize=(3.45, 2.25))
    barcih(ax, order, cost, "Cost per rep (EUR)", BLUE)
    fig.tight_layout(pad=0.4); fig.savefig(f"{args.out}/fig_stats_cost.png", bbox_inches="tight"); plt.close(fig)
    fig, ax = plt.subplots(figsize=(3.45, 2.25))
    barcih(ax, order, qual, "Mean quality", RED)
    ax.set_xlim(0.82, 0.98)
    fig.tight_layout(pad=0.4); fig.savefig(f"{args.out}/fig_stats_quality.png", bbox_inches="tight"); plt.close(fig)
    fig, ax = plt.subplots(figsize=(3.45, 2.25))
    barcih(ax, order, lat, "Latency per rep (ms)", GREEN)
    fig.tight_layout(pad=0.4); fig.savefig(f"{args.out}/fig_stats_latency.png", bbox_inches="tight"); plt.close(fig)

    # Significance table vs premium.
    lines = [f"# Statistical summary — N={len(reps)} repetitions (temperature>0)\n",
             "Per-rep aggregates (cost=sum, quality/latency=mean). Tests are vs B1-premium-static on",
             "the per-rep distributions (Mann-Whitney U, Cliff's delta, Cohen's d).\n"]
    for label, data, unit in [("Cost (EUR, total/rep)", cost, "EUR"),
                              ("Quality (norm, mean/rep)", qual, ""),
                              ("Latency (ms, mean/rep)", lat, "ms")]:
        lines.append(f"\n## {label}\n")
        lines.append("| strategy | mean | 95% CI | vs premium p (MWU) | Cliff δ | Cohen d |")
        lines.append("|---|---:|---|---:|---:|---:|")
        base = data.get(BASE, [])
        for s in order:
            m, lo, hi = ci95(data.get(s, [0]))
            if s == BASE or not base:
                p = d = cd = float("nan")
            else:
                try:
                    _, p = stats.mannwhitneyu(data[s], base, alternative="two-sided")
                except ValueError:
                    p = float("nan")
                cd = cohens_d(data[s], base); d = cliffs_delta(data[s], base)
            lines.append(f"| {s} | {m:.4f} | [{lo:.4f}, {hi:.4f}] | {p:.3g} | {d:+.3f} | {cd:+.3f} |")
    with open(f"{args.results}/stats_summary.md", "w") as fh:
        fh.write("\n".join(lines) + "\n")
    print("wrote figures fig_stats_{cost,quality,latency}.png and", f"{args.results}/stats_summary.md")
    print(f"N={len(reps)} reps")


if __name__ == "__main__":
    main()
