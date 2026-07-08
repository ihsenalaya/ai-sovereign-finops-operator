#!/usr/bin/env python3
"""Prompt-level bootstrap over the cached main workload matrix.

This script addresses the fixed-matrix statistics caveat: instead of treating
the 30 repeated runs as independent workloads, it resamples prompt items within
the committed cached run and reports uncertainty for total cost, mean quality,
and mean latency. It does not call providers and does not alter measured values.
"""

import argparse
import csv
import os
from collections import OrderedDict

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np


MAIN_STRATEGIES = ["B1-premium-static", "B4-static-policy", "B5-budget-hard-block", "B6-ours"]
LABELS = {
    "B1-premium-static": "Premium",
    "B4-static-policy": "Static",
    "B5-budget-hard-block": "Hard block",
    "B6-ours": "Ours",
}


def load_unique_calls(path):
    rows = []
    seen = set()
    with open(path, newline="", encoding="utf-8") as fh:
        for row in csv.DictReader(fh):
            if row["strategy"] not in MAIN_STRATEGIES or row["scenario"] != "global":
                continue
            key = (row["strategy"], row["workload"], row["prompt"])
            if key in seen:
                continue
            seen.add(key)
            row["cost_eur"] = float(row["cost_eur"])
            row["latency_ms"] = float(row["latency_ms"])
            row["quality_norm"] = (float(row["quality_1to5"]) - 1.0) / 4.0
            rows.append(row)
    return rows


def bootstrap(rows, draws, seed):
    by_strategy = OrderedDict((s, [r for r in rows if r["strategy"] == s]) for s in MAIN_STRATEGIES)
    rng = np.random.default_rng(seed)
    out = []
    for strategy, items in by_strategy.items():
        if not items:
            continue
        costs, qualities, latencies = [], [], []
        n = len(items)
        for _ in range(draws):
            sample = rng.choice(items, size=n, replace=True)
            costs.append(sum(r["cost_eur"] for r in sample))
            qualities.append(float(np.mean([r["quality_norm"] for r in sample])))
            latencies.append(float(np.mean([r["latency_ms"] for r in sample])))
        out.append(
            {
                "strategy": strategy,
                "items": n,
                "cost_mean": float(np.mean(costs)),
                "cost_lo": float(np.percentile(costs, 2.5)),
                "cost_hi": float(np.percentile(costs, 97.5)),
                "quality_mean": float(np.mean(qualities)),
                "quality_lo": float(np.percentile(qualities, 2.5)),
                "quality_hi": float(np.percentile(qualities, 97.5)),
                "latency_mean": float(np.mean(latencies)),
                "latency_lo": float(np.percentile(latencies, 2.5)),
                "latency_hi": float(np.percentile(latencies, 97.5)),
            }
        )
    return out


def write_csv(path, rows):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    fields = [
        "strategy",
        "items",
        "cost_mean",
        "cost_95ci_low",
        "cost_95ci_high",
        "quality_mean",
        "quality_95ci_low",
        "quality_95ci_high",
        "latency_mean_ms",
        "latency_95ci_low_ms",
        "latency_95ci_high_ms",
        "evidence_label",
    ]
    with open(path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        for row in rows:
            writer.writerow(
                {
                    "strategy": row["strategy"],
                    "items": row["items"],
                    "cost_mean": f"{row['cost_mean']:.6f}",
                    "cost_95ci_low": f"{row['cost_lo']:.6f}",
                    "cost_95ci_high": f"{row['cost_hi']:.6f}",
                    "quality_mean": f"{row['quality_mean']:.6f}",
                    "quality_95ci_low": f"{row['quality_lo']:.6f}",
                    "quality_95ci_high": f"{row['quality_hi']:.6f}",
                    "latency_mean_ms": f"{row['latency_mean']:.2f}",
                    "latency_95ci_low_ms": f"{row['latency_lo']:.2f}",
                    "latency_95ci_high_ms": f"{row['latency_hi']:.2f}",
                    "evidence_label": "CACHED",
                }
            )


def plot(path, rows):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    labels = [LABELS[r["strategy"]] for r in rows]
    x = np.arange(len(rows))
    fig, axes = plt.subplots(1, 3, figsize=(7.2, 2.25))
    metrics = [
        ("cost", "Cost (EUR)", "#2f6f9f"),
        ("quality", "Quality", "#a83f3f"),
        ("latency", "Latency (ms)", "#3d8b5a"),
    ]
    for ax, (name, ylabel, color) in zip(axes, metrics):
        means = [r[f"{name}_mean"] for r in rows]
        los = [r[f"{name}_mean"] - r[f"{name}_lo"] for r in rows]
        his = [r[f"{name}_hi"] - r[f"{name}_mean"] for r in rows]
        ax.bar(x, means, color=color)
        ax.errorbar(x, means, yerr=[los, his], fmt="none", ecolor="black", capsize=2, linewidth=0.8)
        ax.set_xticks(x)
        ax.set_xticklabels(labels, rotation=25, ha="right")
        ax.set_ylabel(ylabel)
        ax.grid(axis="y", alpha=0.22)
        ax.spines["top"].set_visible(False)
        ax.spines["right"].set_visible(False)
    fig.tight_layout(pad=0.4)
    fig.savefig(path, bbox_inches="tight", dpi=300)
    plt.close(fig)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--calls", default="results/calls.csv")
    parser.add_argument("--out", default="results/bootstrap_items.csv")
    parser.add_argument("--fig", default="figures/fig_bootstrap_items.png")
    parser.add_argument("--draws", type=int, default=5000)
    parser.add_argument("--seed", type=int, default=20260708)
    args = parser.parse_args()
    rows = load_unique_calls(args.calls)
    boot = bootstrap(rows, args.draws, args.seed)
    write_csv(args.out, boot)
    plot(args.fig, boot)
    print(f"wrote {args.out} and {args.fig}")


if __name__ == "__main__":
    main()
