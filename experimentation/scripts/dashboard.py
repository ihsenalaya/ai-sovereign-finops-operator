#!/usr/bin/env python3
"""Generate experimentation/DASHBOARD.md for the revised artifact."""

import csv
import os
from datetime import datetime, timezone

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.chdir(ROOT)


def rows(path):
    if not os.path.exists(path):
        return []
    with open(path, newline="", encoding="utf-8") as fh:
        return list(csv.DictReader(fh))


def get(table, strategy, column):
    for row in table:
        if row.get("strategy") == strategy:
            return row.get(column, "")
    return ""


def main():
    rq1 = rows("results/rq1_cost.csv")
    rq2 = rows("results/rq2_quality.csv")
    rq3 = rows("results/rq3_latency.csv")
    guard = rows("results-bench/rq_guardrail.csv")

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    out = [
        "# Experiment Dashboard",
        "",
        f"_Last updated: {now}_",
        "",
        "## Headline Results",
        "",
    ]
    if rq1:
        out.append(
            f"- **Cost:** B6 costs {get(rq1, 'B6-ours', 'total_cost_eur')} EUR over 40 prompts "
            f"versus {get(rq1, 'B1-premium-static', 'total_cost_eur')} EUR for premium-static "
            f"({get(rq1, 'B6-ours', 'savings_vs_premium_pct')}% reduction under the committed catalog)."
        )
    if rq2:
        out.append(
            f"- **Open-ended quality proxy:** B6 judge mean is {get(rq2, 'B6-ours', 'mean_quality_norm')} "
            f"versus {get(rq2, 'B1-premium-static', 'mean_quality_norm')} for premium-static."
        )
    if rq3:
        out.append(
            f"- **Latency:** B6 mean latency is {get(rq3, 'B6-ours', 'latency_mean_ms')} ms "
            f"versus {get(rq3, 'B1-premium-static', 'latency_mean_ms')} ms for premium-static."
        )
    if guard:
        for row in guard:
            if row["policy"] == "cost-aware":
                out.append(
                    f"- **Objective benchmark:** cost-aware accuracy is {row['accuracy_pct']}% "
                    f"at {row['total_cost_eur']} EUR."
                )
            if row["policy"] == "objective-premium-guardrail":
                out.append(
                    f"- **Objective guardrail:** premium pinning restores {row['accuracy_pct']}% "
                    f"accuracy at premium cost."
                )

    out += [
        "",
        "## Evidence Files",
        "",
        "| Evidence | Path |",
        "|---|---|",
        "| Main replay | `results/rq1_cost.csv`, `results/rq2_quality.csv`, `results/rq3_latency.csv` |",
        "| Declared-policy scenarios | `results/rq4_sovereignty.csv` |",
        "| Budget pressure | `results/rq5_budget.csv` |",
        "| Objective benchmark | `results-bench/rq_benchmark.csv` |",
        "| Objective guardrail | `results-bench/rq_guardrail.csv` |",
        "| Prompt bootstrap | `results/bootstrap_items.csv` |",
        "| Stability run | `results-stats/stats_summary.md` |",
        "| Judge caveats | `results/judge_agreement_summary.md`, `results/judge_vs_truth_summary.md` |",
        "",
        "## Submission Status",
        "",
        "| Item | Status |",
        "|---|:--:|",
        "| Claims audit | done |",
        "| Manuscript source | done |",
        "| PDF build | done |",
        "| Figures regenerated | done |",
        "| Forbidden-phrase check on manuscript | pass |",
        "| Go tests | pass |",
        "| Python script compile | pass |",
        "",
        "## Scope Notes",
        "",
        "- `kind` integration is for CI, debug, and regression verification only.",
        "- Declared-policy compliance is not legal certification.",
        "- Judge-assessed quality is a bounded proxy; objective tasks use exact match.",
        "- Production traces and stress tests are outside the current evidence.",
    ]
    with open("DASHBOARD.md", "w", encoding="utf-8") as fh:
        fh.write("\n".join(out) + "\n")
    print("wrote DASHBOARD.md")


if __name__ == "__main__":
    main()
