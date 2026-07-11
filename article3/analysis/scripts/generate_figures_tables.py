#!/usr/bin/env python3
from __future__ import annotations

import csv
import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt


ROOT = Path(__file__).resolve().parents[2]
PROCESSED = ROOT / "experiments" / "processed"
FIGURES = ROOT / "figures"
TABLES = ROOT / "tables"


def load(name: str) -> dict:
    return json.loads((PROCESSED / name).read_text())


def ensure_dirs() -> None:
    FIGURES.mkdir(parents=True, exist_ok=True)
    TABLES.mkdir(parents=True, exist_ok=True)


def write_csv(path: Path, header: list[str], rows: list[list[object]]) -> None:
    with path.open("w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(header)
        writer.writerows(rows)


def save_plot(fig: plt.Figure, stem: str) -> None:
    fig.tight_layout()
    fig.savefig(FIGURES / f"{stem}.pdf")
    fig.savefig(FIGURES / f"{stem}.png", dpi=300)
    plt.close(fig)


def fig_budget_risk() -> None:
    e1 = load("E1_comparison.json")["comparisons"]
    rows = []
    labels, overshoot, slack = [], [], []
    for comp in e1:
        labels.extend([f"{comp['artifact']}:quantile", f"{comp['artifact']}:mean_std"])
        overshoot.extend([comp["quantile_overshoot"], comp["mean_std_overshoot"]])
        slack.extend([comp["quantile_slack"], comp["mean_std_slack"]])
        rows.extend(
            [
                [comp["artifact"], "quantile", comp["quantile_overshoot"], comp["quantile_slack"]],
                [comp["artifact"], "mean_std", comp["mean_std_overshoot"], comp["mean_std_slack"]],
            ]
        )
    write_csv(TABLES / "table_e1_budget_risk.csv", ["artifact", "variant", "overshoot_total", "slack_total"], rows)
    fig, ax = plt.subplots(figsize=(8, 4))
    x = range(len(labels))
    ax.bar([i - 0.2 for i in x], overshoot, width=0.4, label="overshoot")
    ax.bar([i + 0.2 for i in x], slack, width=0.4, label="slack")
    ax.set_xticks(list(x))
    ax.set_xticklabels(labels, rotation=25, ha="right")
    ax.set_ylabel("Cost units")
    ax.set_title("E1 Budget Risk Versus Reservation Slack")
    ax.legend()
    save_plot(fig, "fig_e1_budget_risk_vs_utilization")


def fig_e1_heatmap() -> None:
    rows = load("E1_matrix.json")["rows"]
    budgets = sorted({r["primary_budget"] for r in rows})
    variants = ["quantile", "mean_std"]
    data = {v: [] for v in variants}
    csv_rows = []
    for budget in budgets:
        for variant in variants:
            matched = [r for r in rows if r["primary_budget"] == budget and r["variant"] == variant]
            avg = sum(r["metrics"]["overshoot_total"] for r in matched) / max(len(matched), 1)
            data[variant].append(avg)
            csv_rows.append([budget, variant, avg])
    write_csv(TABLES / "table_e1_matrix_overshoot.csv", ["primary_budget", "variant", "mean_overshoot"], csv_rows)
    fig, ax = plt.subplots(figsize=(7, 3.5))
    image = ax.imshow([data["quantile"], data["mean_std"]], cmap="Greys", aspect="auto")
    ax.set_yticks([0, 1])
    ax.set_yticklabels(["quantile", "mean_std"])
    ax.set_xticks(range(len(budgets)))
    ax.set_xticklabels([str(b) for b in budgets])
    ax.set_xlabel("Primary budget")
    ax.set_title("E1 Overshoot Heatmap Across Budget Sweep")
    fig.colorbar(image, ax=ax, label="Mean overshoot")
    save_plot(fig, "fig_e1_heatmap_budget_sweep")


def fig_e2_isolation() -> None:
    summary = load("E2_quantile_summary.json")
    rows = [[tenant, cost] for tenant, cost in summary["tenant_settled_total"].items()]
    write_csv(TABLES / "table_e2_tenant_settled.csv", ["tenant", "settled_total"], rows)
    fig, ax = plt.subplots(figsize=(5, 4))
    ax.bar([r[0] for r in rows], [r[1] for r in rows], color=["#444444", "#999999"])
    ax.set_ylabel("Settled cost")
    ax.set_title("E2 Quantile Tenant Isolation Snapshot")
    save_plot(fig, "fig_e2_tenant_isolation")


def fig_e3_drift() -> None:
    drift = load("E3_drift_summary.json")
    rows = [
        ["reference_mean", drift["drift"]["ReferenceMean"]],
        ["recent_mean", drift["drift"]["RecentMean"]],
        ["ratio", drift["drift"]["Ratio"]],
    ]
    write_csv(TABLES / "table_e3_drift_summary.csv", ["metric", "value"], rows)
    fig, ax = plt.subplots(figsize=(5, 4))
    ax.bar(["reference", "recent"], [drift["drift"]["ReferenceMean"], drift["drift"]["RecentMean"]], color=["#777777", "#222222"])
    ax.set_ylabel("Mean output-length proxy")
    ax.set_title("E3 Drift Reaction")
    save_plot(fig, "fig_e3_drift_reaction")


def fig_e4_faults() -> None:
    faults = load("E4_faults_summary.json")
    rows = [
        ["duplicate_settlement_first_accepted", int(faults["duplicate_settlement"]["first_accepted"])],
        ["duplicate_settlement_second_distinct_rejected", int(faults["duplicate_settlement"]["second_distinct_rejected"])],
        ["reservation_expired", int(faults["reservation_expiry"]["expired"])],
        ["telemetry_fault_abstained", int(faults["telemetry_fault"]["abstained"])],
    ]
    write_csv(TABLES / "table_e4_fault_outcomes.csv", ["check", "value"], rows)
    fig, ax = plt.subplots(figsize=(7, 4))
    ax.bar([r[0] for r in rows], [r[1] for r in rows], color="#555555")
    ax.set_ylim(0, 1.2)
    ax.set_ylabel("Boolean outcome")
    ax.set_title("E4 Fault-Handling Outcomes")
    ax.tick_params(axis="x", rotation=20)
    save_plot(fig, "fig_e4_fault_outcomes")


def fig_e5_scalability() -> None:
    rows = load("E5_scalability_summary.json")["rows"]
    csv_rows = [[r["scenario"], r["request_count"], r["requests_per_sec"]] for r in rows]
    write_csv(TABLES / "table_e5_throughput.csv", ["scenario", "request_count", "requests_per_sec"], csv_rows)
    fig, ax = plt.subplots(figsize=(6, 4))
    for scenario in sorted({r["scenario"] for r in rows}):
        subset = [r for r in rows if r["scenario"] == scenario]
        ax.plot([r["request_count"] for r in subset], [r["requests_per_sec"] for r in subset], marker="o", label=scenario)
    ax.set_xlabel("Request count")
    ax.set_ylabel("Requests per second")
    ax.set_title("E5 Replay Scalability")
    ax.legend()
    save_plot(fig, "fig_e5_scalability")


def fig_e6_live() -> None:
    attempts = load("E6_azure_live_summary.json")["attempts"]
    rows = []
    labels = []
    latencies = []
    for name, payload in attempts.items():
        usage = payload.get("usage", {})
        latency = usage.get("latency_checkpoint", {}).get("service_ttlt_ms")
        if latency is not None:
            rows.append([name, latency])
            labels.append(name)
            latencies.append(latency)
    write_csv(TABLES / "table_e6_live_latency.csv", ["attempt", "service_ttlt_ms"], rows)
    if not rows:
        return
    fig, ax = plt.subplots(figsize=(8, 4))
    ax.bar(labels, latencies, color="#666666")
    ax.set_ylabel("ms")
    ax.set_title("E6 Live Azure Validation Latency Snapshot")
    ax.tick_params(axis="x", rotation=25)
    save_plot(fig, "fig_e6_live_latency")


def fig_e7_ablation() -> None:
    rows = load("E7_ablation_summary.json")["rows"]
    budget_delay = [r for r in rows if r["scenario"] == "budget_delay"]
    csv_rows = [[r["variant"]["name"], r["metrics"]["overshoot_total"], r["metrics"]["slack_total"]] for r in budget_delay]
    write_csv(TABLES / "table_e7_ablation_budget_delay.csv", ["variant", "overshoot_total", "slack_total"], csv_rows)
    fig, ax = plt.subplots(figsize=(7, 4))
    ax.bar([r[0] for r in csv_rows], [r[1] for r in csv_rows], color="#333333")
    ax.set_ylabel("Overshoot total")
    ax.set_title("E7 Ablation Overshoot in Budget-Delay Scenario")
    ax.tick_params(axis="x", rotation=20)
    save_plot(fig, "fig_e7_ablation_budget_delay")


def write_summary_tables() -> None:
    summary_rows = []
    for name in [
        "E1_quantile_summary.json",
        "E1_mean_std_summary.json",
        "E2_quantile_summary.json",
        "E2_mean_std_summary.json",
    ]:
        payload = load(name)
        summary_rows.append(
            [
                name.removesuffix("_summary.json"),
                payload["admitted_count"],
                payload["queued_count"],
                payload["settled_total"],
                payload["slack_total"],
                payload["overshoot_total"],
            ]
        )
    write_csv(
        TABLES / "table_core_summaries.csv",
        ["artifact", "admitted_count", "queued_count", "settled_total", "slack_total", "overshoot_total"],
        summary_rows,
    )


def main() -> None:
    ensure_dirs()
    write_summary_tables()
    fig_budget_risk()
    fig_e1_heatmap()
    fig_e2_isolation()
    fig_e3_drift()
    fig_e4_faults()
    fig_e5_scalability()
    fig_e6_live()
    fig_e7_ablation()
    print("generated figures and tables in", FIGURES, TABLES)


if __name__ == "__main__":
    main()
