#!/usr/bin/env python3
from __future__ import annotations

import csv
import json
import math
import random
from pathlib import Path
from statistics import mean, median


ROOT = Path(__file__).resolve().parents[2]
PROCESSED = ROOT / "experiments" / "processed"
ANALYSIS = ROOT / "analysis"
TABLES = ROOT / "tables"
ARTIFACTS = ROOT / "artifacts"

RNG = random.Random(20260711)
BOOTSTRAP_SAMPLES = 5000


def load(name: str) -> dict:
    return json.loads((PROCESSED / name).read_text())


def percentile(sorted_values: list[float], p: float) -> float:
    if not sorted_values:
        return math.nan
    if len(sorted_values) == 1:
        return sorted_values[0]
    idx = (len(sorted_values) - 1) * p
    lo = math.floor(idx)
    hi = math.ceil(idx)
    if lo == hi:
        return sorted_values[lo]
    frac = idx - lo
    return sorted_values[lo] * (1 - frac) + sorted_values[hi] * frac


def bootstrap_mean_ci(values: list[float]) -> tuple[float, float]:
    if not values:
        return (math.nan, math.nan)
    samples = []
    for _ in range(BOOTSTRAP_SAMPLES):
        draw = [RNG.choice(values) for _ in values]
        samples.append(mean(draw))
    samples.sort()
    return percentile(samples, 0.025), percentile(samples, 0.975)


def paired_bootstrap_diff_ci(a: list[float], b: list[float]) -> tuple[float, float]:
    if len(a) != len(b) or not a:
        return (math.nan, math.nan)
    diffs = [x - y for x, y in zip(a, b)]
    samples = []
    for _ in range(BOOTSTRAP_SAMPLES):
        draw = [RNG.choice(diffs) for _ in diffs]
        samples.append(mean(draw))
    samples.sort()
    return percentile(samples, 0.025), percentile(samples, 0.975)


def sign_test_two_sided(diffs: list[float]) -> float:
    pos = sum(1 for d in diffs if d > 0)
    neg = sum(1 for d in diffs if d < 0)
    n = pos + neg
    if n == 0:
        return 1.0
    tail = 0.0
    k = min(pos, neg)
    for i in range(0, k + 1):
        tail += math.comb(n, i) * (0.5**n)
    return min(1.0, 2 * tail)


def summarize_campaign_pair(exp_id: str, left_name: str, right_name: str) -> list[dict[str, object]]:
    left = load(left_name)["run_metrics"]
    right = load(right_name)["run_metrics"]
    metrics = ["admitted_count", "queued_count", "settled_total", "slack_total", "overshoot_total"]
    rows: list[dict[str, object]] = []
    for metric in metrics:
        lvals = [float(run[metric]) for run in left]
        rvals = [float(run[metric]) for run in right]
        diffs = [l - r for l, r in zip(lvals, rvals)]
        left_ci = bootstrap_mean_ci(lvals)
        right_ci = bootstrap_mean_ci(rvals)
        diff_ci = paired_bootstrap_diff_ci(lvals, rvals)
        rows.append(
            {
                "experiment_id": exp_id,
                "metric": metric,
                "left_variant": left_name.replace("_campaign.json", ""),
                "right_variant": right_name.replace("_campaign.json", ""),
                "left_mean": mean(lvals),
                "right_mean": mean(rvals),
                "left_median": median(lvals),
                "right_median": median(rvals),
                "mean_diff_left_minus_right": mean(diffs),
                "left_ci_low": left_ci[0],
                "left_ci_high": left_ci[1],
                "right_ci_low": right_ci[0],
                "right_ci_high": right_ci[1],
                "diff_ci_low": diff_ci[0],
                "diff_ci_high": diff_ci[1],
                "sign_test_pvalue": sign_test_two_sided(diffs),
                "n_runs": len(lvals),
            }
        )
    return rows


def write_csv(path: Path, rows: list[dict[str, object]]) -> None:
    if not rows:
        return
    with path.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        writer.writeheader()
        writer.writerows(rows)


def write_report(path: Path, rows: list[dict[str, object]]) -> None:
    grouped: dict[str, list[dict[str, object]]] = {}
    for row in rows:
        grouped.setdefault(str(row["experiment_id"]), []).append(row)
    lines = ["# Statistical Summary", "", "This report is generated from processed campaign outputs only.", ""]
    for experiment_id, items in grouped.items():
        lines.append(f"## {experiment_id}")
        lines.append("")
        for row in items:
            lines.append(
                f"- `{row['metric']}`: left mean={row['left_mean']:.3f}, right mean={row['right_mean']:.3f}, "
                f"diff={row['mean_diff_left_minus_right']:.3f}, paired bootstrap 95% CI="
                f"[{row['diff_ci_low']:.3f}, {row['diff_ci_high']:.3f}], sign-test p={row['sign_test_pvalue']:.4f}"
            )
        lines.append("")
    path.write_text("\n".join(lines))


def main() -> None:
    ANALYSIS.mkdir(parents=True, exist_ok=True)
    TABLES.mkdir(parents=True, exist_ok=True)
    ARTIFACTS.mkdir(parents=True, exist_ok=True)

    rows = []
    rows.extend(summarize_campaign_pair("E1", "E1_quantile_campaign.json", "E1_mean_std_campaign.json"))
    rows.extend(summarize_campaign_pair("E2", "E2_quantile_campaign.json", "E2_mean_std_campaign.json"))

    csv_path = TABLES / "table_statistical_summary.csv"
    write_csv(csv_path, rows)
    report_path = ANALYSIS / "STATISTICAL_SUMMARY.md"
    write_report(report_path, rows)
    artifacts_path = ARTIFACTS / "STATISTICAL_SUMMARY.md"
    artifacts_path.write_text(report_path.read_text())
    print(f"wrote {csv_path}")
    print(f"wrote {report_path}")
    print(f"wrote {artifacts_path}")


if __name__ == "__main__":
    main()
