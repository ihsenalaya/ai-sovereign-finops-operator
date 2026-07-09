#!/usr/bin/env python3
"""M3: significance analysis of the E2 objective quality axis (existing data only).

Wilson 95% CIs on per-baseline exact-match; pairwise McNemar tests (paired by
item, exact binomial on discordant pairs) with Holm multiplicity correction;
per-baseline mean latency with 95% CI. No simulation: reads the committed E2 run.
"""
import glob
import json
import math
from collections import defaultdict
from itertools import combinations
from pathlib import Path

from scipy import stats

ROOT = Path(__file__).resolve().parents[1]
RUN = Path(sorted(glob.glob(str(ROOT / "experiments/runs/E2_routing_tradeoffs/*/raw/request_results.jsonl")))[-1]).parents[1]
RAW = RUN / "raw/request_results.jsonl"
OUT = RUN / "processed"


def wilson(k, n, z=1.96):
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    d = 1 + z * z / n
    c = (p + z * z / (2 * n)) / d
    h = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n)) / d
    return (c - h, c + h)


def main():
    rows = [json.loads(l) for l in open(RAW)]
    obj = [r for r in rows if r.get("task_family") == "objective"]
    # per baseline: item-key -> exact_match ; and latency list
    em = defaultdict(dict)
    lat = defaultdict(list)
    for r in obj:
        key = (r["item_id"], r["window"])
        em[r["baseline"]][key] = int(r["exact_match"])
        if r.get("latency_ms"):
            lat[r["baseline"]].append(float(r["latency_ms"]))
    baselines = sorted(em)

    per = {}
    for b in baselines:
        vals = em[b]
        n = len(vals)
        k = sum(vals.values())
        lo, hi = wilson(k, n)
        la = lat[b]
        lmean = sum(la) / len(la)
        lsd = (sum((x - lmean) ** 2 for x in la) / (len(la) - 1)) ** 0.5
        lci = 1.96 * lsd / math.sqrt(len(la))
        per[b] = {"n": n, "correct": k, "exact_match": round(k / n, 4),
                  "wilson95": [round(lo, 4), round(hi, 4)],
                  "latency_mean_ms": round(lmean, 1),
                  "latency_ci95_ms": [round(lmean - lci, 1), round(lmean + lci, 1)]}

    # pairwise McNemar (exact) on discordant pairs, Holm-corrected
    tests = []
    for a, b in combinations(baselines, 2):
        keys = set(em[a]) & set(em[b])
        b_disc = sum(1 for kk in keys if em[a][kk] == 1 and em[b][kk] == 0)  # a right, b wrong
        c_disc = sum(1 for kk in keys if em[a][kk] == 0 and em[b][kk] == 1)  # a wrong, b right
        nd = b_disc + c_disc
        # exact McNemar = two-sided binomial test on min(b,c) with p=0.5
        pval = stats.binomtest(min(b_disc, c_disc), nd, 0.5).pvalue if nd > 0 else 1.0
        tests.append({"pair": f"{a} vs {b}", "a_only_correct": b_disc,
                      "b_only_correct": c_disc, "discordant": nd, "p_raw": pval})
    # Holm
    tests.sort(key=lambda t: t["p_raw"])
    m = len(tests)
    prev = 0.0
    for i, t in enumerate(tests):
        adj = min(1.0, (m - i) * t["p_raw"])
        adj = max(adj, prev)  # enforce monotonicity
        prev = adj
        t["p_holm"] = round(float(adj), 4)
        t["p_raw"] = round(float(t["p_raw"]), 4)
        t["significant_0.05"] = bool(adj < 0.05)

    any_sig = any(t["significant_0.05"] for t in tests)
    ems = [per[b]["exact_match"] for b in baselines]
    summary = {
        "run": RUN.name, "n_objective_per_baseline": per[baselines[0]]["n"],
        "note": "Wilson 95% CIs; exact McNemar paired by (item,window); Holm-corrected across 28 pairs. Measured E2 data only.",
        "exact_match_range": [min(ems), max(ems)],
        "any_pair_significant_after_holm": any_sig,
        "branch": ("some_pairs_significant" if any_sig else "baselines_indistinguishable_in_objective_quality"),
        "per_baseline": per,
        "pairwise_mcnemar": tests,
    }
    OUT.mkdir(exist_ok=True)
    (OUT / "e2_significance.json").write_text(json.dumps(summary, indent=2))

    print(f"N objective/baseline = {summary['n_objective_per_baseline']}")
    print(f"exact-match range = {summary['exact_match_range']}")
    print("per-baseline exact-match [Wilson 95% CI]:")
    for b in baselines:
        p = per[b]
        print(f"  {b:36s} {p['exact_match']:.3f} [{p['wilson95'][0]:.3f}, {p['wilson95'][1]:.3f}]  lat {p['latency_mean_ms']:.0f} ms {p['latency_ci95_ms']}")
    nsig = sum(t["significant_0.05"] for t in tests)
    print(f"\nMcNemar pairs significant after Holm: {nsig} / {m}")
    for t in sorted(tests, key=lambda x: x["p_holm"])[:5]:
        print(f"  {t['pair']:52s} p_raw={t['p_raw']:.3f} p_holm={t['p_holm']:.3f} sig={t['significant_0.05']}")
    print(f"\nBRANCH: {summary['branch']}")


if __name__ == "__main__":
    main()
