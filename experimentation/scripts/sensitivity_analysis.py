#!/usr/bin/env python3
"""Generate local-only sensitivity tables for the paper.

The script does not call any LLM provider and does not modify measured values.
It reads committed workloads/results and writes deterministic CSVs:

- scoring_weight_sensitivity.csv: SIMULATED routing decisions using the same
  catalog priors and scoring equation as B6.
- budget_threshold_sensitivity.csv: SIMULATED budget outcomes from the existing
  RQ5 spending rows.
- breakeven_sensitivity.csv: MODELED break-even scenarios around the RQ6 model.
"""
import argparse
import csv
import json
import math
from pathlib import Path


MODELS = {
    "gpt-4o": {
        "provider": "openai-us",
        "zone": "US",
        "managed": True,
        "in": 2.50,
        "out": 10.00,
        "quality": 1.00,
        "latency": 900.0,
    },
    "gpt-4o-mini": {
        "provider": "openai-us",
        "zone": "US",
        "managed": True,
        "in": 0.15,
        "out": 0.60,
        "quality": 0.88,
        "latency": 600.0,
    },
    "gpt-4.1-nano": {
        "provider": "openai-us",
        "zone": "US",
        "managed": True,
        "in": 0.10,
        "out": 0.40,
        "quality": 0.80,
        "latency": 450.0,
    },
    "mistral-large": {
        "provider": "mistral-eu",
        "zone": "EU",
        "managed": True,
        "in": 2.00,
        "out": 6.00,
        "quality": 0.96,
        "latency": 850.0,
    },
    "mistral-small": {
        "provider": "mistral-eu",
        "zone": "EU",
        "managed": True,
        "in": 0.20,
        "out": 0.60,
        "quality": 0.84,
        "latency": 550.0,
    },
    "selfhosted-eu-llama": {
        "provider": "onprem-fr",
        "zone": "FR",
        "managed": False,
        "in": 0.05,
        "out": 0.05,
        "quality": 0.74,
        "latency": 700.0,
    },
}


WEIGHT_SCENARIOS = [
    ("base", 1.0, 1.5, 0.3, 0.5, 1.0),
    ("cost_heavy", 2.0, 1.0, 0.3, 0.5, 1.5),
    ("quality_heavy", 0.7, 2.5, 0.3, 0.5, 0.5),
    ("latency_heavy", 1.0, 1.5, 1.2, 0.5, 1.0),
    ("budget_heavy", 1.0, 1.2, 0.3, 0.5, 2.5),
]


def est_tokens(text):
    return max(1, len(text) // 4)


def cost(model, input_tokens, output_tokens):
    return input_tokens / 1_000_000 * model["in"] + output_tokens / 1_000_000 * model["out"]


def choose(workload, prompt, weights, budget_used, budget_total):
    alpha, beta, gamma, delta, epsilon = weights
    allowed = [MODELS[m] | {"id": m} for m in workload["allowedModels"] if m in MODELS]
    exhausted = budget_total > 0 and budget_used >= budget_total
    pool = [m for m in allowed if m["quality"] >= workload["minQuality"] or exhausted]
    if not pool:
        pool = allowed
    premium = MODELS[workload["premiumModel"]]
    input_tokens = est_tokens((prompt.get("system") or "") + prompt["text"])
    output_tokens = 200
    premium_cost = max(cost(premium, input_tokens, output_tokens), 1e-9)
    premium_quality = max(premium["quality"], 1e-9)
    budget_pressure = min(1.0, budget_used / budget_total) if budget_total > 0 else 0.0
    max_latency = max([m["latency"] for m in pool] + [1.0])
    best = None
    best_score = float("inf")
    for m in pool:
        norm_cost = cost(m, input_tokens, output_tokens) / premium_cost
        quality_loss = max(0.0, (premium_quality - m["quality"]) / premium_quality)
        latency_penalty = m["latency"] / max_latency
        sov_risk = 0.0
        score = (
            alpha * norm_cost * (1 + epsilon * budget_pressure)
            + beta * quality_loss
            + gamma * latency_penalty
            + delta * sov_risk
        )
        if score < best_score:
            best_score = score
            best = m
    return best, input_tokens, output_tokens


def write_csv(path, header, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(header)
        writer.writerows(rows)
    print(f"wrote {path}")


def scoring_weight_sensitivity(repo, results):
    workloads = []
    for path in sorted((repo / "experimentation/datasets").glob("w*.json")):
        workloads.append(json.loads(path.read_text(encoding="utf-8")))
    rows = []
    for name, alpha, beta, gamma, delta, epsilon in WEIGHT_SCENARIOS:
        total_cost = 0.0
        quality_sum = 0.0
        served = 0
        premium_picks = 0
        cheapest_picks = 0
        for workload in workloads:
            budget_total = workload["monthlyBudgetEUR"]
            budget_used = 0.0
            for prompt in workload["prompts"]:
                model, input_tokens, output_tokens = choose(
                    workload, prompt, (alpha, beta, gamma, delta, epsilon), budget_used, budget_total
                )
                c = cost(model, input_tokens, output_tokens)
                budget_used += c
                total_cost += c
                quality_sum += model["quality"]
                served += 1
                if model["id"] == workload["premiumModel"]:
                    premium_picks += 1
                if model["id"] in ("gpt-4.1-nano", "selfhosted-eu-llama"):
                    cheapest_picks += 1
        rows.append(
            [
                name,
                f"{alpha:.2f}",
                f"{beta:.2f}",
                f"{gamma:.2f}",
                f"{delta:.2f}",
                f"{epsilon:.2f}",
                f"{total_cost:.6f}",
                f"{quality_sum / served:.4f}",
                served,
                premium_picks,
                cheapest_picks,
                "SIMULATED",
            ]
        )
    write_csv(
        results / "scoring_weight_sensitivity.csv",
        [
            "scenario",
            "alpha_cost",
            "beta_quality",
            "gamma_latency",
            "delta_sovereignty",
            "epsilon_budget",
            "estimated_cost_eur",
            "mean_quality_prior",
            "served",
            "premium_picks",
            "cheapest_picks",
            "evidence_label",
        ],
        rows,
    )


def budget_threshold_sensitivity(results):
    with (results / "calls.csv").open(newline="", encoding="utf-8") as f:
        calls = [row for row in csv.DictReader(f) if row["scenario"] == "global" and row["workload"] == "W4-Analytical-Agent"]
    def unique_by_prompt(strategy):
        seen = {}
        for row in calls:
            if row["strategy"] == strategy and row["prompt"] not in seen:
                seen[row["prompt"]] = row
        return [seen[k] for k in sorted(seen)]

    premium_calls = unique_by_prompt("B1-premium-static")
    ours_calls = unique_by_prompt("B6-ours")
    n_prompts = len(premium_calls)
    premium_total = sum(float(r["cost_eur"]) for r in premium_calls)
    rows = []
    for threshold in [0.30, 0.40, 0.50, 0.70, 1.00]:
        budget = premium_total * threshold
        policy_rows = []
        policy_rows.append(("alert-only", premium_calls, premium_total, len(premium_calls), 0))
        used, served, blocked = 0.0, 0, 0
        for r in premium_calls:
            if used >= budget:
                blocked += 1
                continue
            used += float(r["cost_eur"])
            served += 1
        policy_rows.append(("hard-block", premium_calls[:served], used, served, blocked))
        ours_used = sum(float(r["cost_eur"]) for r in ours_calls)
        policy_rows.append(("ours-graceful", ours_calls, ours_used, len(ours_calls), 0))
        for policy, source_rows, used, served, blocked in policy_rows:
            qualities = [float(r["quality_1to5"]) for r in source_rows if r["quality_1to5"]]
            quality = sum((q - 1) / 4 for q in qualities) / len(qualities) if qualities else 0.0
            overrun = max(0.0, (used - budget) / budget * 100)
            rows.append(
                [
                    f"{int(threshold * 100)}",
                    policy,
                    f"{budget:.6f}",
                    f"{used:.6f}",
                    served,
                    blocked,
            f"{served / n_prompts * 100:.2f}",
                    f"{overrun:.2f}",
                    f"{quality:.4f}",
                    "SIMULATED",
                ]
            )
    write_csv(
        results / "budget_threshold_sensitivity.csv",
        [
            "budget_as_pct_of_premium",
            "policy",
            "budget_eur",
            "used_eur",
            "served",
            "blocked",
            "availability_pct",
            "budget_overrun_pct",
            "mean_quality_norm",
            "evidence_label",
        ],
        rows,
    )


def breakeven_sensitivity(results):
    blended_per_m = 0.7 * MODELS["gpt-4o"]["in"] + 0.3 * MODELS["gpt-4o"]["out"]
    scenarios = [
        ("low_ops_high_util", 1200, 500, 2000, 0.80, 1.00),
        ("base", 1800, 700, 5000, 1.00, 1.00),
        ("high_ops_low_util", 3500, 1500, 10000, 0.40, 1.00),
        ("api_price_minus_30pct", 1800, 700, 5000, 1.00, 0.70),
        ("api_price_plus_30pct", 1800, 700, 5000, 1.00, 1.30),
    ]
    rows = []
    for name, gpu, ops, migration, utilization, api_mult in scenarios:
        effective_self = (gpu + ops) / max(utilization, 0.01)
        effective_price = blended_per_m * api_mult
        break_even = effective_self / (30 * effective_price) * 1_000_000
        managed_at_25m = 25_000_000 * 30 / 1_000_000 * effective_price
        savings = managed_at_25m - effective_self
        payback = "n/a" if savings <= 0 else f"{migration / savings:.2f}"
        rec = "keep-managed" if savings <= 0 else ("self-host" if float(payback) <= 6 else "investigate")
        rows.append(
            [
                name,
                f"{gpu:.2f}",
                f"{ops:.2f}",
                f"{migration:.2f}",
                f"{utilization:.2f}",
                f"{api_mult:.2f}",
                f"{effective_self:.2f}",
                f"{break_even:.2f}",
                f"{managed_at_25m:.2f}",
                f"{savings:.2f}",
                payback,
                rec,
                "MODELED",
            ]
        )
    write_csv(
        results / "breakeven_sensitivity.csv",
        [
            "scenario",
            "gpu_monthly_eur",
            "ops_monthly_eur",
            "migration_eur",
            "utilization",
            "managed_api_price_multiplier",
            "effective_selfhost_monthly_eur",
            "break_even_tokens_per_day",
            "managed_monthly_at_25m_tokens_day_eur",
            "monthly_savings_at_25m_tokens_day_eur",
            "payback_months_at_25m_tokens_day",
            "recommendation_at_25m_tokens_day",
            "evidence_label",
        ],
        rows,
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", default=".", help="repository root")
    parser.add_argument("--results", default="experimentation/results")
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    results = (repo / args.results).resolve()
    scoring_weight_sensitivity(repo, results)
    budget_threshold_sensitivity(results)
    breakeven_sensitivity(results)


if __name__ == "__main__":
    main()
