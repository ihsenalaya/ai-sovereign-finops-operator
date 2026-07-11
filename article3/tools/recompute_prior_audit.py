#!/usr/bin/env python3
"""Recompute forensic facts about the quarantined Codex-era Article 3 output."""

from __future__ import annotations

import json
import re
import subprocess
from collections import Counter
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
A3 = ROOT / "article3"
OLD = A3 / "archive" / "codex_20260711"
OUT = A3 / "audit"


def load(path: Path):
    return json.loads(path.read_text(encoding="utf-8"))


def command(*args: str) -> str:
    p = subprocess.run(args, cwd=ROOT, text=True, stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, check=False)
    return p.stdout


def pdf_fact(path: Path) -> dict:
    info = command("pdfinfo", str(path))
    pages = int(re.search(r"(?m)^Pages:\s+(\d+)", info).group(1))
    text = command("pdftotext", str(path), "-")
    return {"path": str(path.relative_to(ROOT)), "pages": pages,
            "words": len(re.findall(r"\b\w+\b", text)), "bytes": path.stat().st_size}


def request_ids(doc: dict) -> set[str]:
    return {str(e["RequestID"]) for e in doc.get("events", []) if e.get("RequestID")}


def metric_exposure(run: dict) -> int:
    m = run.get("metrics", run)
    return int(m.get("admitted_count", 0) + m.get("queued_count", 0)
               + m.get("rejected_count", 0) + m.get("abstained_count", 0))


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    raw = OLD / "experiments" / "raw"
    processed = OLD / "experiments" / "processed"

    pdfs = [pdf_fact(p) for p in sorted({*OLD.glob("artifacts/*.pdf"), *OLD.glob("overleaf/*.pdf")})]
    bibs = []
    for path in sorted(OLD.rglob("*.bib")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        bibs.append({"path": str(path.relative_to(ROOT)),
                     "entries": len(re.findall(r"(?m)^\s*@\w+\s*\{", text))})
    screening_path = OLD / "literature" / "screening.csv"
    screening_rows = max(0, len(screening_path.read_text(encoding="utf-8").splitlines()) - 1) if screening_path.exists() else 0

    initial = {}
    for exp in ("E1", "E2", "E3"):
        docs = [load(p) for p in sorted(raw.glob(f"{exp}_*.json"))]
        initial[exp] = [{"variant": d.get("variant"), "unique_requests": len(request_ids(d)),
                         "tenants": sorted({str(e.get("TenantID")) for e in d.get("events", []) if e.get("TenantID")}),
                         "decision_events": metric_exposure(d.get("metrics", {}))} for d in docs]

    campaign = {}
    for exp, per_run in (("E1", 12), ("E2", 14)):
        rows = []
        for path in sorted(processed.glob(f"{exp}_*_campaign.json")):
            doc = load(path)
            rows.append({"variant": doc.get("variant"), "runs": len(doc.get("run_metrics", [])),
                         "requests_per_run_from_generator": per_run,
                         "method_request_exposures": len(doc.get("run_metrics", [])) * per_run})
        matrix = load(processed / f"{exp}_matrix.json")
        matrix_requests = 12 if exp == "E1" else 14
        campaign[exp] = {"campaigns": rows, "matrix_rows": len(matrix.get("rows", [])),
                         "matrix_seeds": sorted({r.get("seed") for r in matrix.get("rows", [])}),
                         "matrix_method_request_exposures": len(matrix.get("rows", [])) * matrix_requests}

    e4 = load(raw / "E4_faults.json")
    e5 = load(raw / "E5_scalability.json")
    e6 = load(raw / "E6_azure_live.json")
    e7 = load(raw / "E7_ablation.json")
    live_attempts = e6.get("attempts", {})
    live = {name: {"http_status": int(v.get("status_code", 0)),
                   "has_usage": isinstance(v.get("usage"), dict),
                   "input_tokens": (v.get("usage") or {}).get("prompt_tokens"),
                   "output_tokens": (v.get("usage") or {}).get("completion_tokens")}
            for name, v in live_attempts.items()}

    nodes = []
    workloads = []
    jobs = []
    for run_dir in sorted((OLD / "experiments" / "logs" / "kind-diagnostics").glob("*")):
        node_lines = (run_dir / "nodes.txt").read_text(encoding="utf-8").splitlines() if (run_dir / "nodes.txt").exists() else []
        work_text = (run_dir / "workloads.txt").read_text(encoding="utf-8").strip() if (run_dir / "workloads.txt").exists() else ""
        job_text = (run_dir / "job-logs.txt").read_text(encoding="utf-8").strip() if (run_dir / "job-logs.txt").exists() else ""
        nodes.append({"diagnostic": run_dir.name, "node_count": max(0, len(node_lines) - 1),
                      "node_names": [x.split()[0] for x in node_lines[1:] if x.split()]})
        workloads.append({"diagnostic": run_dir.name, "nonempty": bool(work_text)})
        jobs.append({"diagnostic": run_dir.name, "text": job_text})

    tests = (OLD / "artifacts" / "TEST_REPORT.md").read_text(encoding="utf-8")
    passed_claims = re.findall(r"^- `([^`]+)`", tests, re.M)
    remaining = re.findall(r"^- (.+)$", tests.split("## Remaining validation not completed", 1)[-1], re.M)

    result = {
        "generated_from": str(OLD.relative_to(ROOT)),
        "pdfs": pdfs,
        "bibliography": {"files": bibs, "screening_rows": screening_rows,
                         "verified_reference_count": 0,
                         "reason": "No retained row contains primary-source verification evidence sufficient for the new protocol."},
        "experiments": {
            "initial": initial,
            "campaign_and_matrix": campaign,
            "E4": {"simulated_checks": len([k for k in e4 if k not in {"experiment_id", "variant"}]), "repetitions_per_check": 1},
            "E5": {"rows": len(e5.get("rows", [])), "total_requests": sum(int(r.get("request_count", 0)) for r in e5.get("rows", [])),
                   "kind_measurement": False, "cluster_recreations": 0, "steady_minutes": 0},
            "E6": {"attempts": len(live), "successful_http": sum(x["http_status"] == 200 for x in live.values()),
                   "calls_with_usage": sum(x["has_usage"] for x in live.values()), "attempts_by_endpoint": live,
                   "independent_windows": 0, "kind_path": False},
            "E7": {"aggregate_rows": len(e7.get("rows", [])),
                   "variants": sorted({r.get("variant", {}).get("name") for r in e7.get("rows", [])}),
                   "scenarios": sorted({r.get("scenario") for r in e7.get("rows", [])}),
                   "original_method_request_exposures_from_generator": 4 * (12 + 14),
                   "decision_attempts_including_queue_retries": sum(metric_exposure(r) for r in e7.get("rows", []))},
        },
        "implemented_baselines": ["empirical_quantile_0.75", "mean_plus_1_std"],
        "required_baselines_missing": ["no_budget", "settled_spend_only", "mean", "fixed_margin", "fixed_offline_quantile",
                                       "strict_max_output_tokens", "fixed_estimate_reserve_settle", "adaptive_quantile",
                                       "expected_cost_router", "oracle_future_cost", "GOV_AR_risk_allocation"],
        "tests": {"commands_claimed_passed_in_old_report": passed_claims, "explicitly_not_completed": remaining,
                  "independently_reexecuted_for_forensic_audit": False},
        "kind": {"nodes": nodes, "workloads": workloads, "jobs": jobs,
                 "conclusion": "A two-node cluster and Helm release existed, but diagnostics contain no workloads and no experiment Job."},
        "ghcr": {"old_report_claimed_published": False,
                 "old_report_result": "GHCR push denied; no Article 3 remote digest was recorded.",
                 "base_controller_0_5_11_digest": "sha256:abc591624156aa2a6dc958221d3c0968a1fed11475d6a8e2b38c6b6644a62e88",
                 "gov_ar_admission_0_5_11_exists": False},
        "unsupported_or_contradicted_claims": [
            "The three-page manuscript describes trace-driven E1-E7 evidence despite the absence of protocol-scale observations.",
            "Campaign outputs were described as paired although the generator cumulatively mutated seeds between variants.",
            "The reported overshoot metric is request under-reservation, not tenant budget-window overshoot.",
            "E0 was called end-to-end but performed a Go test and three file-existence checks.",
            "E5 throughput is an in-process Go replay measurement, not Kind service scalability.",
            "E6 made six endpoint calls outside the Kind reserve-settle path, not a three-window live validation.",
            "GHCR and OCI publication were incomplete despite final/complete artifact names."
        ],
        "reuse_after_review_and_tests": ["Go package scaffolds", "experiment CLI structure", "Kind script layout", "LaTeX build script structure"],
        "replace_or_rederive": ["all measurements", "all processed statistics", "all figures/tables", "protocol", "literature metadata",
                                "manuscript results/conclusions", "GOV-AR ledger and admission algorithm", "release declarations"]
    }

    (OUT / "codex_output_audit.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    pdf_lines = "\n".join(f"- `{p['path']}`: {p['pages']} pages, {p['words']} extracted words, {p['bytes']} bytes." for p in pdfs)
    bib_lines = "\n".join(f"- `{b['path']}`: {b['entries']} BibTeX entries." for b in bibs)
    e1 = campaign["E1"]; e2 = campaign["E2"]
    md = f"""# Independent audit of the prior Codex Article 3 output

Generated by `article3/tools/recompute_prior_audit.py` from quarantined files only. The machine-readable facts are in `audit/codex_output_audit.json`; the pre-archive checksum manifest protects provenance. None of these measurements is eligible for final analysis.

## Manuscript and bibliography

{pdf_lines}

{bib_lines}

The old screening table has {screening_rows} retained rows. The new audit counts **zero verified references** because the old files do not preserve sufficient primary-source metadata/review-status verification. A BibTeX entry is not verification.

## Actual experiment depth

- E1 initial traces: 2 unique requests per variant. Campaigns: 5 runs × 12 requests × 2 variants = {sum(x['method_request_exposures'] for x in e1['campaigns'])} method-request exposures. Matrix: {e1['matrix_rows']} rows, {len(e1['matrix_seeds'])} seeds, {e1['matrix_method_request_exposures']} exposures.
- E2 initial traces: 4 unique requests per variant. Campaigns: 5 runs × 14 requests × 2 variants = {sum(x['method_request_exposures'] for x in e2['campaigns'])} exposures. Matrix: {e2['matrix_rows']} rows, {len(e2['matrix_seeds'])} seeds, {e2['matrix_method_request_exposures']} exposures.
- E3: 2 unique requests in one hard-coded drift example.
- E4: {result['experiments']['E4']['simulated_checks']} deterministic in-memory checks, one repetition each; not the required 20 Kind/PostgreSQL fault scenarios.
- E5: {result['experiments']['E5']['rows']} in-process rows totaling {result['experiments']['E5']['total_requests']} requests; zero ten-minute Kind intervals and zero independent cluster recreations.
- E6: {result['experiments']['E6']['attempts']} attempts, {result['experiments']['E6']['successful_http']} HTTP 200 responses, {result['experiments']['E6']['calls_with_usage']} with usage, zero independently identified windows, and no Kind reserve-settle path.
- E7: {result['experiments']['E7']['aggregate_rows']} aggregate rows over {len(result['experiments']['E7']['variants'])} variants and {len(result['experiments']['E7']['scenarios'])} tiny scenarios; 4 × (12 + 14) = {result['experiments']['E7']['original_method_request_exposures_from_generator']} original method-request exposures, producing {result['experiments']['E7']['decision_attempts_including_queue_retries']} decision attempts after queue retries.

Only empirical quantile 0.75 and mean-plus-one-standard-deviation reservation variants were implemented. No strong published router, oracle policy, strict max reservation comparison, adaptive quantile, or actual GOV-AR concurrent risk allocation ran.

## Azure and infrastructure

The six live endpoint attempts used three named deployments/endpoints but do not record three independent time windows. Four responses contain usage. They are connectivity probes, not Article 3 live evidence. Token-derived spend was not measured.

Kind diagnostics show two Ready nodes (`gov-ar-control-plane`, `gov-ar-worker`) and Helm release `gov-ar-experiment-0.1.0`, but both workload listings are empty and the Job log says `No jobs found in namespace gov-ar`. Thus no measured request path is evidenced.

The old test report claims selected package tests, vet, Helm lint, paper build, and analysis scripts. It explicitly says full operator tests, race tests, and GHCR publication were not completed. This forensic audit does not promote those claims to current pass status.

The base controller 0.5.11 image resolves remotely, but `gov-ar-admission:0.5.11` does not. The old publication report records a denied GHCR push and no Article 3 digest.

## Unsupported or contradicted claims

""" + "\n".join(f"- {x}" for x in result["unsupported_or_contradicted_claims"]) + """

## Reuse decision

The Go/CLI/Kind/LaTeX scaffolds may be reused only after review and tests. All old data, statistics, figures, tables, protocol claims, literature verification, manuscript results, ledger design, and release declarations must be replaced or independently rederived.
"""
    (OUT / "CODEX_OUTPUT_AUDIT.md").write_text(md, encoding="utf-8")


if __name__ == "__main__":
    main()
