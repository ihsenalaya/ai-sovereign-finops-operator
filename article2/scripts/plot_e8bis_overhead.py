#!/usr/bin/env python3
"""IEEE-style figures for M2 controller overhead. Reads processed_overhead.json."""
import glob
import json
import sys
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

matplotlib.rcParams.update({
    "font.size": 8, "axes.titlesize": 8, "axes.labelsize": 8,
    "xtick.labelsize": 7, "ytick.labelsize": 7, "legend.fontsize": 7,
    "figure.dpi": 300, "axes.grid": True, "grid.alpha": 0.3, "grid.linewidth": 0.4,
})
# Okabe-Ito
BLUE, ORANGE, GREEN, VERM = "#0072B2", "#E69F00", "#009E73", "#D55E00"

RUN = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(sorted(glob.glob(
    str(Path(__file__).resolve().parents[1] / "reports/M2_controller_overhead/*")))[-1])
data = json.loads((RUN / "processed_overhead.json").read_text())
tiers = data["tiers"]
x = [t["tier_policies_each_kind"] * 2 for t in tiers]  # total reconciled objects (both kinds)
p50 = [t["reconcile_p50_ms"] for t in tiers]
p95 = [t["reconcile_p95_ms"] for t in tiers]
cpu = [t["cpu_cores_avg"] for t in tiers]
mem = [t["memory_mib"] for t in tiers]
figdir = Path(__file__).resolve().parents[1] / "paper/figures"
figdir.mkdir(parents=True, exist_ok=True)

# Fig 1: reconcile latency vs reconciled-object count
fig, ax = plt.subplots(figsize=(3.3, 2.2))
ax.plot(x, p95, "-o", color=VERM, label="p95", markersize=4)
ax.plot(x, p50, "-s", color=BLUE, label="p50", markersize=4)
ax.set_xlabel("Reconciled policy objects")
ax.set_ylabel("Reconcile latency (ms)")
ax.set_ylim(bottom=0)
ax.legend(frameon=False)
fig.tight_layout(pad=0.3)
fig.savefig(figdir / "fig_e8bis_reconcile.pdf")

# Fig 2: CPU + memory vs reconciled-object count (dual axis)
fig, ax = plt.subplots(figsize=(3.3, 2.2))
ax.plot(x, cpu, "-o", color=GREEN, label="CPU", markersize=4)
ax.set_xlabel("Reconciled policy objects")
ax.set_ylabel("CPU (cores)", color=GREEN)
ax.tick_params(axis="y", labelcolor=GREEN)
ax.set_ylim(bottom=0)
ax2 = ax.twinx()
ax2.plot(x, mem, "-^", color=ORANGE, label="Memory", markersize=4)
ax2.set_ylabel("Memory (MiB)", color=ORANGE)
ax2.tick_params(axis="y", labelcolor=ORANGE)
ax2.grid(False)
fig.tight_layout(pad=0.3)
fig.savefig(figdir / "fig_e8bis_resources.pdf")
print("wrote fig_e8bis_reconcile.pdf and fig_e8bis_resources.pdf to", figdir)
