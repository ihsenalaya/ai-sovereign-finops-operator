#!/usr/bin/env python3
"""Analyze E8-bis controller-overhead snapshots into a per-tier table + figures.

All numbers come from the operator's controller-runtime Prometheus metrics
scraped on the live cluster; nothing is simulated. Per-tier windows use the
snapshot file mtimes as scrape timestamps.
"""
import glob
import json
import os
import re
import sys
from pathlib import Path

RUN = Path(sys.argv[1]) if len(sys.argv) > 1 else None
if RUN is None:
    runs = sorted(glob.glob(str(Path(__file__).resolve().parents[1] /
                                "reports/M2_controller_overhead/*")))
    RUN = Path(runs[-1])
SNAP = RUN / "snapshots"


def parse(path):
    """Return dict of metric->value; histograms as name->{le:count}, plus counters/gauges."""
    buckets = {}   # reconcile bucket: le(float)->count
    scalars = {}
    recon_sum = 0.0
    recon_count = 0.0
    for line in Path(path).read_text().splitlines():
        if line.startswith("#") or not line.strip():
            continue
        if line.startswith("controller_runtime_reconcile_time_seconds_bucket"):
            m = re.search(r'le="([^"]+)".*\}\s+([0-9.e+]+)$', line)
            if m:
                le = float(m.group(1)) if m.group(1) != "+Inf" else float("inf")
                buckets[le] = buckets.get(le, 0.0) + float(m.group(2))
        elif line.startswith("controller_runtime_reconcile_time_seconds_sum"):
            recon_sum += float(line.rsplit("}", 1)[-1] if "}" in line else line.split()[-1])
        elif line.startswith("controller_runtime_reconcile_time_seconds_count"):
            recon_count += float(line.rsplit("}", 1)[-1] if "}" in line else line.split()[-1])
        else:
            for key in ("process_cpu_seconds_total",
                        "process_resident_memory_bytes",
                        "workqueue_depth",
                        "workqueue_adds_total",
                        "rest_client_requests_total"):
                if line.startswith(key):
                    val = float(line.split()[-1])
                    scalars[key] = scalars.get(key, 0.0) + val
    return {"buckets": buckets, "recon_sum": recon_sum, "recon_count": recon_count, "scalars": scalars}


def hist_quantile(delta_buckets, q):
    if not delta_buckets:
        return None
    items = sorted((le, c) for le, c in delta_buckets.items())
    total = items[-1][1]
    if total <= 0:
        return None
    target = q * total
    prev_le, prev_c = 0.0, 0.0
    for le, c in items:
        if c >= target:
            if le == float("inf"):
                return prev_le
            if c == prev_c:
                return le
            return prev_le + (le - prev_le) * (target - prev_c) / (c - prev_c)
        prev_le, prev_c = le, c
    return items[-1][0]


def main():
    snaps = sorted(glob.glob(str(SNAP / "*.prom")))
    if len(snaps) < 2:
        sys.exit("need baseline + at least one tier snapshot")
    parsed = [(Path(s).name, os.path.getmtime(s), parse(s)) for s in snaps]

    rows = []
    for i in range(1, len(parsed)):
        name, mt, cur = parsed[i]
        _, mt0, prev = parsed[i - 1]
        window = max(mt - mt0, 1e-3)
        tier = re.sub(r"[^0-9]", "", name) or name
        # delta reconcile buckets
        dbuck = {le: cur["buckets"].get(le, 0) - prev["buckets"].get(le, 0)
                 for le in cur["buckets"]}
        dcount = cur["recon_count"] - prev["recon_count"]
        dsum = cur["recon_sum"] - prev["recon_sum"]
        d_rest = cur["scalars"].get("rest_client_requests_total", 0) - prev["scalars"].get("rest_client_requests_total", 0)
        d_adds = cur["scalars"].get("workqueue_adds_total", 0) - prev["scalars"].get("workqueue_adds_total", 0)
        d_cpu = cur["scalars"].get("process_cpu_seconds_total", 0) - prev["scalars"].get("process_cpu_seconds_total", 0)
        rows.append({
            "tier_policies_each_kind": int(tier) if tier.isdigit() else tier,
            "window_s": round(window, 1),
            "reconciles_in_window": int(dcount),
            "reconcile_mean_ms": round(dsum / dcount * 1000, 2) if dcount else None,
            "reconcile_p50_ms": round((hist_quantile(dbuck, 0.5) or 0) * 1000, 2),
            "reconcile_p95_ms": round((hist_quantile(dbuck, 0.95) or 0) * 1000, 2),
            "workqueue_depth_at_scrape": int(cur["scalars"].get("workqueue_depth", 0)),
            "workqueue_adds_in_window": int(d_adds),
            "api_writes_in_window": int(d_rest),
            "api_writes_per_s": round(d_rest / window, 2),
            "cpu_cores_avg": round(d_cpu / window, 4),
            "memory_mib": round(cur["scalars"].get("process_resident_memory_bytes", 0) / 1024 / 1024, 1),
        })

    out = {"run": RUN.name, "note": "controller-runtime metrics scraped live; per-tier deltas over snapshot mtime windows.", "tiers": rows}
    (RUN / "processed_overhead.json").write_text(json.dumps(out, indent=2))
    # markdown
    cols = ["tier_policies_each_kind", "reconciles_in_window", "reconcile_p50_ms", "reconcile_p95_ms",
            "workqueue_depth_at_scrape", "api_writes_per_s", "cpu_cores_avg", "memory_mib"]
    L = ["# M2 — Controller overhead vs reconciled-object count", "",
         f"Run: `{RUN.name}` (live AKS, controller-runtime metrics, mock load, no LLM calls).", "",
         "| tier (policies/kind) | reconciles | p50 ms | p95 ms | wq depth | API writes/s | CPU cores | mem MiB |",
         "|--:|--:|--:|--:|--:|--:|--:|--:|"]
    for r in rows:
        L.append("| " + " | ".join(str(r[c]) for c in cols) + " |")
    (RUN / "SUMMARY.md").write_text("\n".join(L) + "\n")
    print("\n".join(L))


if __name__ == "__main__":
    main()
