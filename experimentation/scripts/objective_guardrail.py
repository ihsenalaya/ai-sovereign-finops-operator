#!/usr/bin/env python3
"""Build an objective-task guardrail table from measured benchmark rows.

The benchmark file already contains measured exact-match outcomes for premium
static and cost-aware routing. For an objective-task guardrail that pins
GSM8K/MMLU to the premium model, the measured outcome is exactly the premium
row over the same items. This script makes that recomposition explicit.
"""

import argparse
import csv
import os


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--benchmark", default="results-bench/rq_benchmark.csv")
    parser.add_argument("--out", default="results-bench/rq_guardrail.csv")
    args = parser.parse_args()

    with open(args.benchmark, newline="", encoding="utf-8") as fh:
        rows = {row["strategy"]: row for row in csv.DictReader(fh)}

    premium = rows["B1-premium-static"]
    ours = rows["B6-ours"]
    out_rows = [
        {
            "policy": "premium-static",
            "accuracy_pct": premium["accuracy_pct"],
            "total_cost_eur": premium["total_cost_eur"],
            "latency_p95_ms": premium["latency_p95_ms"],
            "evidence_label": "CACHED",
            "interpretation": "Measured premium baseline on GSM8K/MMLU.",
        },
        {
            "policy": "cost-aware",
            "accuracy_pct": ours["accuracy_pct"],
            "total_cost_eur": ours["total_cost_eur"],
            "latency_p95_ms": ours["latency_p95_ms"],
            "evidence_label": "CACHED",
            "interpretation": "Measured cost-aware policy on GSM8K/MMLU.",
        },
        {
            "policy": "objective-premium-guardrail",
            "accuracy_pct": premium["accuracy_pct"],
            "total_cost_eur": premium["total_cost_eur"],
            "latency_p95_ms": premium["latency_p95_ms"],
            "evidence_label": "CACHED-RECOMPOSED",
            "interpretation": "Objective tasks pinned to premium; recomposed from the measured premium row over the same items.",
        },
    ]

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with open(args.out, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=list(out_rows[0].keys()))
        writer.writeheader()
        writer.writerows(out_rows)
    print(f"wrote {args.out}")


if __name__ == "__main__":
    main()
