#!/usr/bin/env python3
"""Statistical analysis scaffolding for the experiments.

IMPORTANT: this does NOT rerun experiments and does NOT modify any measured
values. It consumes results/calls.csv. With the current single deterministic pass
it reports per-strategy bootstrap CIs for latency and a clear notice that
between-strategy significance/effect-size tests require N>=30 repetitions
(produced by future multi-rep runs). When repetitions are present (a `rep`
column or multiple rows per (strategy, prompt)), it computes the full battery.

Tests provided (SciPy):
- bootstrap 95% CI (percentile)
- Mann-Whitney U (independent)
- Wilcoxon signed-rank (paired)
- Cliff's delta (non-parametric effect size)
- Cohen's d (parametric effect size)

Usage: python3 scripts/stats.py [--results results] [--metric latency_ms]
"""
import argparse
import itertools
import numpy as np
import pandas as pd
from scipy import stats


def bootstrap_ci(x, n=2000, alpha=0.05, seed=42):
    x = np.asarray(x, float)
    if len(x) == 0:
        return (float("nan"), float("nan"), float("nan"))
    rng = np.random.default_rng(seed)
    means = np.array([rng.choice(x, size=len(x), replace=True).mean() for _ in range(n)])
    return float(x.mean()), float(np.percentile(means, 100 * alpha / 2)), float(np.percentile(means, 100 * (1 - alpha / 2)))


def cliffs_delta(a, b):
    a, b = np.asarray(a, float), np.asarray(b, float)
    if len(a) == 0 or len(b) == 0:
        return float("nan")
    gt = sum((x > b).sum() for x in a)
    lt = sum((x < b).sum() for x in a)
    return (gt - lt) / (len(a) * len(b))


def cohens_d(a, b):
    a, b = np.asarray(a, float), np.asarray(b, float)
    if len(a) < 2 or len(b) < 2:
        return float("nan")
    na, nb = len(a), len(b)
    sp = np.sqrt(((na - 1) * a.var(ddof=1) + (nb - 1) * b.var(ddof=1)) / (na + nb - 2))
    return float((a.mean() - b.mean()) / sp) if sp > 0 else float("nan")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--results", default="results")
    ap.add_argument("--calls", default="", help="explicit calls CSV (default: <results>/calls.csv)")
    ap.add_argument("--metric", default="latency_ms", help="numeric column in the calls CSV")
    args = ap.parse_args()

    path = args.calls or f"{args.results}/calls.csv"
    calls = pd.read_csv(path)
    served = calls[(calls.get("blocked") == False)] if "blocked" in calls else calls
    metric = args.metric
    if metric not in served.columns:
        raise SystemExit(f"metric '{metric}' not in calls.csv columns: {list(served.columns)}")
    served = served[served[metric] > 0] if metric.endswith("_ms") else served

    # True repetitions are signalled by an explicit `rep` column (added by future
    # multi-repetition runs). A single deterministic pass has none, even though the
    # same (strategy, scenario, prompt) cell may appear twice via the response cache.
    has_reps = "rep" in served.columns

    print(f"# Statistical report — metric: {metric}")
    print(f"repetitions detected: {'YES' if has_reps else 'NO (single deterministic pass)'}\n")

    strategies = sorted(served.strategy.unique())
    print("## Per-strategy bootstrap 95% CI (mean)")
    samples = {}
    for s in strategies:
        x = served[served.strategy == s][metric].values
        samples[s] = x
        m, lo, hi = bootstrap_ci(x)
        print(f"  {s:24s} n={len(x):4d}  mean={m:10.3f}  CI95=[{lo:.3f}, {hi:.3f}]")

    if not has_reps:
        print("\n## Between-strategy significance & effect size")
        print("  SKIPPED — single deterministic pass. Between-strategy tests on a single")
        print("  pass would treat per-prompt values as the sample; for the camera-ready we")
        print("  run N>=30 repetitions (see ROADMAP_Q1.md) and enable the battery below.")
        print("  The intra-pass bootstrap CIs above are reported as a lower-bound on variance.")
        # Still demonstrate the machinery on the available per-call distributions,
        # clearly labeled as illustrative (not a validity claim).
        print("\n## Illustrative pairwise tests on per-call distributions (NOT a significance claim)")

    base = "B1-premium-static" if "B1-premium-static" in samples else strategies[0]
    for s in strategies:
        if s == base:
            continue
        a, b = samples[s], samples[base]
        try:
            u, pu = stats.mannwhitneyu(a, b, alternative="two-sided")
        except ValueError:
            u, pu = float("nan"), float("nan")
        d = cliffs_delta(a, b)
        cd = cohens_d(a, b)
        tag = "" if has_reps else "[illustrative]"
        print(f"  {s:24s} vs {base}: MWU p={pu:.4g}  Cliff_delta={d:+.3f}  Cohen_d={cd:+.3f} {tag}")

    print("\nNote: no measured values were altered; this script is read-only over results/calls.csv.")


if __name__ == "__main__":
    main()
