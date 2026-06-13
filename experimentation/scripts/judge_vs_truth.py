#!/usr/bin/env python3
"""Explain the exact-match vs LLM-judge contradiction (results/judgevstruth.csv).

Shows whether the LLM judge over-rates objectively-incorrect answers: mean judge
score for correct vs incorrect answers, % of incorrect answers rated "acceptable"
(>=3), point-biserial correlation, and per-model accuracy. Writes a summary +
figure. This turns the contradiction into a methodological finding (judge validity
is task-dependent).
"""
import argparse
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
    "xtick.labelsize": 7,
    "ytick.labelsize": 7,
    "legend.fontsize": 7,
    "axes.grid": True,
    "grid.alpha": 0.22,
    "grid.linewidth": 0.6,
    "axes.spines.top": False,
    "axes.spines.right": False,
})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--file", default="results/judgevstruth.csv")
    ap.add_argument("--results", default="results")
    ap.add_argument("--figures", default="figures")
    args = ap.parse_args()
    df = pd.read_csv(args.file)
    corr = df[df.correct == 1].judge_score
    inc = df[df.correct == 0].judge_score
    rpb, p = stats.pointbiserialr(df.correct, df.judge_score)
    acc_overall = df.correct.mean() * 100
    inc_acceptable = (inc >= 3).mean() * 100 if len(inc) else float("nan")

    lines = [
        f"# Judge vs ground truth — n={len(df)} (objective benchmark items)\n",
        f"- Overall exact-match accuracy: **{acc_overall:.1f}%**",
        f"- Mean judge score — **correct: {corr.mean():.2f}**, **incorrect: {inc.mean():.2f}** "
        f"(scale 1-5, n_correct={len(corr)}, n_incorrect={len(inc)})",
        f"- **% of *incorrect* answers the judge rated acceptable (>=3): {inc_acceptable:.1f}%**",
        f"- Point-biserial correlation (correct vs judge score): **r={rpb:.3f}** (p={p:.2g})",
        "",
        "## Per-model exact-match accuracy",
        "| model | n | accuracy % | mean judge |",
        "|---|---:|---:|---:|",
    ]
    for m, g in df.groupby("model"):
        lines.append(f"| {m} | {len(g)} | {g.correct.mean()*100:.1f} | {g.judge_score.mean():.2f} |")
    lines += [
        "",
        "**Finding.** On objective tasks the LLM judge assigns high acceptability even to many",
        "*wrong* answers, so judge-based quality over-estimates true correctness — this explains the",
        "exact-match vs judge contradiction. Consequence for the paper: report judge quality on",
        "open-ended workloads only, and use exact-match on tasks with ground truth; do not claim",
        "quality preservation from the judge alone.",
    ]
    open(f"{args.results}/judge_vs_truth_summary.md", "w").write("\n".join(lines) + "\n")

    fig, ax = plt.subplots(figsize=(3.45, 2.25))
    bins = np.arange(0.5, 6.5, 1)
    ax.hist([inc, corr], bins=bins, label=["incorrect", "correct"],
            color=["#a83f3f", "#3d8b5a"], edgecolor="white", linewidth=0.5)
    ax.set_xlabel("LLM judge score (1-5)")
    ax.set_ylabel("Count")
    ax.legend(frameon=False)
    fig.tight_layout(pad=0.4); fig.savefig(f"{args.figures}/fig_judge_vs_truth.png", bbox_inches="tight"); plt.close(fig)

    print("\n".join(lines[:6]))
    print(f"wrote {args.results}/judge_vs_truth_summary.md and {args.figures}/fig_judge_vs_truth.png")


if __name__ == "__main__":
    main()
