#!/usr/bin/env python3
"""Regenerate Figure 4: E2 cost/quality trade-off with 95% CI error bars.

x-axis: normalized cost per request (premium-like baseline = 1.0), dimensionless
(no absolute currency). y-axis: objective exact-match with Wilson 95% CI. All
numbers from the committed E2 run and its significance analysis.
"""
import glob
import json
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

matplotlib.rcParams.update({
    "font.size": 8, "axes.labelsize": 8, "xtick.labelsize": 7,
    "ytick.labelsize": 7, "legend.fontsize": 7, "figure.dpi": 300,
    "axes.grid": True, "grid.alpha": 0.3, "grid.linewidth": 0.4,
})
BLUE, ORANGE = "#0072B2", "#D55E00"

ROOT = Path(__file__).resolve().parents[1]
RUN = Path(sorted(glob.glob(str(ROOT / "experiments/runs/E2_routing_tradeoffs/*/processed/e2_significance.json")))[-1]).parents[1]
sig = json.loads((RUN / "processed/e2_significance.json").read_text())
summ = json.loads((RUN / "processed/summary.json").read_text())

cpr = {b: m["cost_per_request_eur"] for b, m in summ["baselines"].items()}
mx = max(cpr.values())
norm = {b: cpr[b] / mx for b in cpr}

short = {b: "B" + b.split("_")[0][1:] for b in sig["per_baseline"]}
premium = {"B1_premium_static", "B3_static_namespace", "B4_policy_only_premium"}

fig, ax = plt.subplots(figsize=(3.4, 2.5))
for b, p in sig["per_baseline"].items():
    x = norm[b]
    y = p["exact_match"]
    lo, hi = p["wilson95"]
    color = ORANGE if b in premium else BLUE
    ax.errorbar(x, y, yerr=[[y - lo], [hi - y]], fmt="o", color=color,
                markersize=5, capsize=2, elinewidth=0.9, zorder=3)

# label offsets to avoid overlap (dx, dy) per baseline
off = {
    "B1_premium_static": (6, 10), "B3_static_namespace": (6, -14),
    "B4_policy_only_premium": (-22, 12), "B2_least_cost": (8, 12),
    "B5_policy_only_least_cost": (-26, 14), "B6_routing_score_no_gate": (-30, -16),
    "B7_routing_score_with_gate": (8, -14), "B8_routing_score_gate_change_request": (8, 10),
}
for b, p in sig["per_baseline"].items():
    dx, dy = off.get(b, (6, 6))
    ax.annotate(short[b], (norm[b], p["exact_match"]),
                textcoords="offset points", xytext=(dx, dy), fontsize=6.5,
                arrowprops=dict(arrowstyle="-", lw=0.4, color="0.5"))

ax.axhline(0.5, ls=":", lw=0.6, color="0.6")
ax.text(0.66, 0.503, "chance", fontsize=6, color="0.5")
ax.set_xlabel("Normalized cost per request (premium = 1.0)")
ax.set_ylabel("Objective exact-match")
ax.set_xlim(0.6, 1.05)
ax.set_ylim(0.44, 0.66)
# legend proxies
from matplotlib.lines import Line2D
ax.legend([Line2D([], [], marker="o", ls="", color=ORANGE),
           Line2D([], [], marker="o", ls="", color=BLUE)],
          ["premium-like", "routed"], frameon=False, loc="lower left")
fig.tight_layout(pad=0.3)
out = ROOT / "paper/figures/fig_e2_cost_quality_tradeoff.pdf"
fig.savefig(out)
fig.savefig(str(out).replace(".pdf", ".png"))
print("wrote", out, "(error bars = Wilson 95% CI; cost normalized, no currency)")
