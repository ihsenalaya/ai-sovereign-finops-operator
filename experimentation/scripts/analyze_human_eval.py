#!/usr/bin/env python3
"""Analyze filled human-evaluation sheets against the hidden key.

Computes, per evaluator and averaged: human accuracy, human-vs-exact-match
agreement, human-vs-LLM-judge agreement (Cohen's quadratic-weighted kappa) and
inter-human agreement. This validates or tempers the LLM judge as a quality proxy.

Run only AFTER evaluators fill human_eval/sheet_*.csv. No scores are invented.
Usage: python3 scripts/analyze_human_eval.py --sheets "human_eval/sheet_*.csv"
"""
import argparse
import glob
import numpy as np
import pandas as pd


def weighted_kappa(a, b, k=5):
    a = np.asarray(a, int) - 1; b = np.asarray(b, int) - 1
    O = np.zeros((k, k))
    for x, y in zip(a, b):
        if 0 <= x < k and 0 <= y < k:
            O[x, y] += 1
    W = np.array([[((i - j) ** 2) / ((k - 1) ** 2) for j in range(k)] for i in range(k)])
    n = O.sum()
    if n == 0:
        return float("nan")
    E = np.outer(O.sum(1), O.sum(0)) / n
    return 1 - (W * O).sum() / (W * E).sum() if (W * E).sum() else float("nan")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--sheets", default="human_eval/sheet_*.csv")
    ap.add_argument("--key", default="human_eval/key.csv")
    ap.add_argument("--out", default="human_eval/human_eval_summary.md")
    args = ap.parse_args()
    files = [f for f in glob.glob(args.sheets) if "sheet.csv" not in f]
    if not files:
        raise SystemExit("no filled evaluator sheets found (human_eval/sheet_<name>.csv). Fill them first.")
    key = pd.read_csv(args.key).set_index("eval_id")

    lines = [f"# Human evaluation results ({len(files)} evaluators)\n"]
    hcols = {}
    for f in files:
        df = pd.read_csv(f).dropna(subset=["human_score_1to5"]).set_index("eval_id")
        j = df.join(key, how="inner")
        name = f.split("sheet_")[-1].replace(".csv", "")
        hcols[name] = j["human_score_1to5"].astype(int)
        acc = (j["human_correct_0or1"].astype(int) == 1).mean() * 100
        agree_em = (j["human_correct_0or1"].astype(int) == j["exact_match"].astype(int)).mean() * 100
        k_judge = weighted_kappa(j["human_score_1to5"].astype(int), j["judge_score"].astype(int))
        lines += [f"## Evaluator {name} (n={len(j)})",
                  f"- human-judged accuracy: {acc:.1f}%",
                  f"- human-correct vs exact-match agreement: {agree_em:.1f}%",
                  f"- human-score vs LLM-judge (weighted kappa): {k_judge:.3f}", ""]
    if len(hcols) >= 2:
        names = list(hcols)
        common = hcols[names[0]].index.intersection(hcols[names[1]].index)
        kk = weighted_kappa(hcols[names[0]].loc[common], hcols[names[1]].loc[common])
        lines += [f"## Inter-human agreement ({names[0]} vs {names[1]}): weighted kappa = {kk:.3f}", ""]
    open(args.out, "w").write("\n".join(lines) + "\n")
    print("\n".join(lines))
    print("wrote", args.out)


if __name__ == "__main__":
    main()
