#!/usr/bin/env python3
"""Inter-judge agreement on the quality metric (results/judge_agreement.csv).

Computes Cohen's quadratic-weighted kappa, Krippendorff's alpha (ordinal),
Spearman correlation and exact/within-1 agreement between two judges' 1-5 scores.
Validates the LLM-as-judge as a quality proxy (addresses judge dependence).

Usage: python3 scripts/judge_agreement.py [--file results/judge_agreement.csv] [--results results]
"""
import argparse
import numpy as np
import pandas as pd
from scipy import stats


def weighted_kappa(a, b, k=5):
    a = np.asarray(a, int) - 1
    b = np.asarray(b, int) - 1
    O = np.zeros((k, k))
    for x, y in zip(a, b):
        O[x, y] += 1
    W = np.zeros((k, k))
    for i in range(k):
        for j in range(k):
            W[i, j] = ((i - j) ** 2) / ((k - 1) ** 2)
    act_a = O.sum(1); act_b = O.sum(0); n = O.sum()
    E = np.outer(act_a, act_b) / n
    num = (W * O).sum(); den = (W * E).sum()
    return 1 - num / den if den else float("nan")


def krippendorff_alpha_ordinal(a, b, k=5):
    # Two coders, complete data: alpha = 1 - Do/De with squared-difference metric.
    a = np.asarray(a, float); b = np.asarray(b, float)
    n = len(a)
    Do = np.mean((a - b) ** 2)
    allv = np.concatenate([a, b])
    De = np.mean([(x - y) ** 2 for x in allv for y in allv])
    return 1 - Do / De if De else float("nan")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--file", default="results/judge_agreement.csv")
    ap.add_argument("--results", default="results")
    args = ap.parse_args()
    df = pd.read_csv(args.file)
    cols = [c for c in df.columns if c.startswith("judge_")]
    if len(cols) < 2:
        raise SystemExit(f"need two judge columns, got {cols}")
    a, b = df[cols[0]].values, df[cols[1]].values
    kappa = weighted_kappa(a, b)
    alpha = krippendorff_alpha_ordinal(a, b)
    rho, p = stats.spearmanr(a, b)
    exact = np.mean(a == b) * 100
    within1 = np.mean(np.abs(a - b) <= 1) * 100
    lines = [
        f"# Inter-judge agreement — {cols[0]} vs {cols[1]} (n={len(df)})\n",
        f"- mean score: {cols[0]}={a.mean():.3f}, {cols[1]}={b.mean():.3f}",
        f"- Cohen's quadratic-weighted kappa: **{kappa:.3f}**",
        f"- Krippendorff's alpha (ordinal): **{alpha:.3f}**",
        f"- Spearman rho: **{rho:.3f}** (p={p:.2g})",
        f"- exact agreement: {exact:.1f}%  ·  within-1: {within1:.1f}%",
        "",
        "Interpretation: kappa/alpha > 0.6 = substantial agreement; this supports using the",
        "LLM judge as a quality proxy. Lower values would weaken quality claims (reported honestly).",
    ]
    out = f"{args.results}/judge_agreement_summary.md"
    open(out, "w").write("\n".join(lines) + "\n")
    print("\n".join(lines))
    print("wrote", out)


if __name__ == "__main__":
    main()
