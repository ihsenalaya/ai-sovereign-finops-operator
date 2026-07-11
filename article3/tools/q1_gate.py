#!/usr/bin/env python3
"""Independent, fail-closed Article 3 completion gate.

The gate recomputes repository, evidence, experiment, manuscript, and release
facts. It never consumes article3/STATUS.json as evidence. Missing or malformed
evidence is a failure. The normal mode is useful during development; --strict
adds remote/release and full experimental-floor checks and is the only mode
that can authorize release artifacts.
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
import zipfile
from collections import Counter, defaultdict
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Iterable


ROOT = Path(__file__).resolve().parents[2]
A3 = ROOT / "article3"
ARCHIVE = A3 / "archive" / "codex_20260711"


@dataclass
class Check:
    group: str
    name: str
    passed: bool
    detail: str
    critical: bool = True


CHECKS: list[Check] = []


def add(group: str, name: str, passed: bool, detail: str, critical: bool = True) -> bool:
    CHECKS.append(Check(group, name, bool(passed), detail, critical))
    return bool(passed)


def run(args: list[str], timeout: int = 60) -> tuple[int, str]:
    try:
        p = subprocess.run(args, cwd=ROOT, text=True, stdout=subprocess.PIPE,
                           stderr=subprocess.STDOUT, timeout=timeout, check=False)
        return p.returncode, p.stdout.strip()
    except (OSError, subprocess.TimeoutExpired) as exc:
        return 127, str(exc)


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for block in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def load_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def csv_rows(path: Path) -> list[dict[str, str]]:
    try:
        with path.open(newline="", encoding="utf-8") as fh:
            return list(csv.DictReader(fh))
    except (OSError, csv.Error):
        return []


def file_ok(path: Path, minimum: int = 1) -> bool:
    return path.is_file() and path.stat().st_size >= minimum


def review_clear(path: Path) -> tuple[bool, str]:
    doc = load_json(path)
    if not isinstance(doc, dict):
        return False, "missing or malformed JSON review"
    findings = doc.get("findings", [])
    unresolved = [f for f in findings if str(f.get("severity", "")).lower() in {"critical", "major"}
                  and str(f.get("status", "open")).lower() not in {"resolved", "accepted"}]
    approved = str(doc.get("verdict", "")).lower() in {"approve", "approved", "pass", "passed"}
    return approved and not unresolved, f"verdict={doc.get('verdict')!r}, unresolved_critical_major={len(unresolved)}"


def verify_archive() -> tuple[bool, str]:
    manifest = ARCHIVE / "PRE_ARCHIVE_SHA256SUMS.txt"
    if not file_ok(manifest):
        return False, "checksum manifest missing"
    checked = 0
    failures: list[str] = []
    for line in manifest.read_text(encoding="utf-8").splitlines():
        m = re.match(r"^([0-9a-f]{64})\s+(.+)$", line)
        if not m:
            failures.append("malformed manifest line")
            continue
        expected, old = m.groups()
        old_path = Path(old)
        if old_path.parts[:1] != ("article3",):
            failures.append(old)
            continue
        rel = Path(*old_path.parts[2:]) if len(old_path.parts) > 2 else Path()
        section = old_path.parts[1] if len(old_path.parts) > 1 else ""
        current = ARCHIVE / section / rel
        if not current.is_file() or sha256(current) != expected:
            failures.append(str(current.relative_to(ROOT) if current.is_absolute() else current))
        checked += 1
    legacy_manifest = ARCHIVE / "LEGACY_CODE_SHA256SUMS.txt"
    if not file_ok(legacy_manifest):
        failures.append("legacy checksum manifest missing")
    else:
        for line in legacy_manifest.read_text(encoding="utf-8").splitlines():
            m = re.match(r"^([0-9a-f]{64})\s+(.+)$", line)
            if not m:
                failures.append("malformed legacy manifest line")
                continue
            expected, old = m.groups()
            rel = old.removeprefix("article3/")
            if rel.startswith("experiments/manifests/"):
                rel = "experiments/" + rel.removeprefix("experiments/manifests/")
            current = ARCHIVE / "legacy" / rel
            if not current.is_file() or sha256(current) != expected:
                failures.append(str(current))
            checked += 1
    return checked > 0 and not failures, f"checked={checked}, failures={failures[:5]}"


def repository_checks(strict: bool) -> None:
    rc, branch = run(["git", "branch", "--show-current"])
    add("repository", "dedicated recovery branch", rc == 0 and bool(re.fullmatch(r"article3-q1-recovery-\d{8}(?:[-\w.]*)?", branch)), branch)

    base = load_json(A3 / "provenance" / "base_version.json")
    base_ok = isinstance(base, dict) and re.fullmatch(r"[0-9a-f]{40}", str(base.get("base_sha", ""))) is not None
    if base_ok:
        rc_obj, _ = run(["git", "cat-file", "-e", f"{base['base_sha']}^{{commit}}"])
        rc_anc, _ = run(["git", "merge-base", "--is-ancestor", base["base_sha"], "HEAD"])
        base_ok = rc_obj == 0 and rc_anc == 0 and base.get("operator_app_version") == "0.5.11"
    add("repository", "recorded operator base is an ancestor", base_ok, json.dumps(base, sort_keys=True)[:600] if base else "missing")

    ok, detail = verify_archive()
    add("repository", "archived evidence checksum integrity", ok, detail)
    forbidden = [A3 / p for p in ("figures", "tables", "reports", "overleaf") if (A3 / p).exists()]
    # These directories are allowed only after the new protocol has valid runs.
    protocol = A3 / "experiments" / "registry" / "frozen_protocol.yaml"
    protocol_frozen = file_ok(protocol) and "status: frozen" in protocol.read_text(encoding="utf-8", errors="ignore").lower()
    add("repository", "old evidence excluded from active roots", not forbidden or protocol_frozen,
        "active evidence dirs=" + ",".join(str(p.relative_to(ROOT)) for p in forbidden))

    rc, tracked_status = run(["git", "status", "--porcelain"])
    status_lines = [x for x in tracked_status.splitlines() if x]
    preserved = load_json(A3 / "provenance" / "preexisting_worktree.json") or {}
    allowed_lines: set[str] = set()
    for row in preserved.get("preserved_untracked", []):
        path = ROOT / str(row.get("path", ""))
        if path.is_file() and re.fullmatch(r"[0-9a-f]{64}", str(row.get("sha256", ""))) and sha256(path) == row["sha256"]:
            allowed_lines.add(f"?? {row['path']}")
    unexpected = [x for x in status_lines if x not in allowed_lines]
    add("repository", "final tree committed", (not strict) or (rc == 0 and not unexpected),
        f"unexpected={unexpected[:30]}, preserved={sorted(allowed_lines)}")
    rc, head = run(["git", "rev-parse", "HEAD"])
    add("repository", "HEAD resolves", rc == 0 and bool(re.fullmatch(r"[0-9a-f]{40}", head)), head)

    # Recompute a focused secret scan; do not trust a prose report.
    patterns = [r"-----BEGIN [A-Z ]*PRIVATE KEY-----", r"\bgh[opusr]_[A-Za-z0-9]{30,}\b",
                r"\bsk-[A-Za-z0-9_-]{24,}\b", r"(?i)(api[_-]?key|access[_-]?token)\s*[:=]\s*['\"]?[A-Za-z0-9_./+=-]{20,}"]
    rc, files = run(["git", "ls-files", "-z"])
    hits: list[str] = []
    if rc == 0:
        for name in files.split("\0"):
            path = ROOT / name
            if not name or not path.is_file() or path.stat().st_size > 5_000_000:
                continue
            try:
                text = path.read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            if any(re.search(p, text) for p in patterns):
                hits.append(name)
    add("security", "tracked-file secret pattern scan", not hits, f"hits={hits[:10]}")


def literature_checks(strict: bool) -> None:
    lit = A3 / "literature"
    required = ["search_log.csv", "screening.csv", "related_work_matrix.csv", "novelty_assessment.md", "references.bib", "rejected_references.csv"]
    missing = [n for n in required if not file_ok(lit / n)]
    add("literature", "required review outputs", not missing, f"missing={missing}")
    bib = (lit / "references.bib").read_text(encoding="utf-8", errors="ignore") if file_ok(lit / "references.bib") else ""
    bib_count = len(re.findall(r"(?m)^\s*@(?:article|inproceedings|book|incollection|misc|techreport|phdthesis|mastersthesis)\s*\{", bib, re.I))
    screening = csv_rows(lit / "screening.csv")
    verified = [r for r in screening if r.get("decision", "").lower() in {"include", "included"}
                and r.get("metadata_verified", "").lower() == "true" and r.get("relevant", "").lower() == "true"]
    peer = [r for r in verified if r.get("peer_reviewed", "").lower() == "true"]
    add("literature", ">=40 verified relevant bibliography records", bib_count >= 40 and len(verified) >= 40,
        f"bib={bib_count}, verified_relevant={len(verified)}")
    add("literature", "majority peer-reviewed primary research", len(verified) >= 40 and len(peer) > len(verified) / 2,
        f"peer_reviewed={len(peer)}/{len(verified)}")
    searches = csv_rows(lit / "search_log.csv")
    saturation = [r for r in searches if r.get("pass_type", "").lower() in {"expanded", "citation_chase", "citation-chase"}
                  and r.get("material_new_work", "").lower() == "false"]
    add("literature", "two material-novelty saturation passes", len(saturation) >= 2, f"passes={len(saturation)}")
    matrix_text = (lit / "novelty_assessment.md").read_text(encoding="utf-8", errors="ignore").lower() if file_ok(lit / "novelty_assessment.md") else ""
    closest_markers = ["token budgets", "paretobandit", "r2-router", "pilot"]
    add("literature", "closest current prior art explicitly assessed", all(x in matrix_text for x in closest_markers),
        f"markers_present={[x for x in closest_markers if x in matrix_text]}")
    for role in ("literature_novelty_audit.json", "scientific_red_team.json"):
        ok, detail = review_clear(A3 / "reviews" / role)
        add("review", role, ok, detail)


def implementation_checks(strict: bool) -> None:
    test_doc = load_json(A3 / "reports" / "test_results.json")
    required = {"format", "go_test", "go_race_govar", "go_vet", "lint", "unit_reservations", "transactions",
                "idempotence", "envtest", "postgres_integration", "gateway_e2e", "helm_lint", "helm_install",
                "helm_upgrade", "helm_rollback", "helm_uninstall", "sbom", "vulnerability_scan", "secret_scan"}
    passed: set[str] = set()
    if isinstance(test_doc, dict):
        passed = {str(x.get("name")) for x in test_doc.get("checks", []) if x.get("exit_code") == 0 and x.get("status") == "passed"}
    add("implementation", "required tests and scans pass", required <= passed,
        f"missing_or_failed={sorted(required - passed)}")

    e0 = load_json(A3 / "experiments" / "manifests" / "final" / "E0.json")
    e0_ok = isinstance(e0, dict) and e0.get("status") == "valid" and all(e0.get("path_checks", {}).get(k) is True for k in
        ("gateway", "admission", "atomic_reserve", "backend", "usage", "settlement", "aggregate_state"))
    add("implementation", "full measured E0 path", e0_ok, json.dumps(e0, sort_keys=True)[:600] if e0 else "missing")

    formal = load_json(A3 / "formal" / "results.json")
    formal_ok = isinstance(formal, dict) and formal.get("exit_code") == 0 and formal.get("invariants_checked", 0) >= 3
    add("theory", "formal invariant model passes", formal_ok, json.dumps(formal, sort_keys=True)[:500] if formal else "missing")

    digests = csv_rows(A3 / "provenance" / "image_digests.csv")
    immutable = [r for r in digests if re.fullmatch(r"sha256:[0-9a-f]{64}", r.get("digest", ""))]
    kinds = {r.get("artifact_type") for r in immutable}
    add("release", "immutable image and OCI chart digests recorded", {"image", "chart"} <= kinds and len(immutable) >= 3,
        f"valid_digest_rows={len(immutable)}, types={sorted(str(x) for x in kinds)}")
    if strict and immutable:
        failures = []
        for row in immutable:
            ref = row.get("reference", "")
            digest = row.get("digest", "")
            if row.get("artifact_type") == "image":
                rc, out = run(["docker", "buildx", "imagetools", "inspect", f"{ref}@{digest}"], timeout=120)
                if rc != 0 or digest not in out:
                    failures.append(ref)
            elif row.get("artifact_type") == "chart":
                # A successful OCI pull is authoritative; checksum is checked below.
                version = row.get("version", "")
                with tempfile.TemporaryDirectory() as td:
                    rc, _ = run(["helm", "pull", ref, "--version", version, "--destination", td], timeout=120)
                    tgzs = list(Path(td).glob("*.tgz"))
                    if rc != 0 or len(tgzs) != 1 or sha256(tgzs[0]) != row.get("package_sha256"):
                        failures.append(ref)
        add("release", "remote image/chart digests resolve", not failures, f"failures={failures}")


def dataset_protocol_checks() -> None:
    protocol = A3 / "experiments" / "registry" / "frozen_protocol.yaml"
    text = protocol.read_text(encoding="utf-8", errors="ignore") if file_ok(protocol) else ""
    frozen = re.search(r"(?m)^status:\s*frozen\s*$", text, re.I) is not None
    required_terms = ["hypotheses:", "primary_metrics:", "baselines:", "seeds:", "invalid_run_rules:",
                      "multiplicity", "azure", "frozen_test_data_hashes:", "software_hashes:"]
    add("protocol", "protocol frozen and complete", frozen and all(t.lower() in text.lower() for t in required_terms),
        f"frozen={frozen}, missing_terms={[t for t in required_terms if t.lower() not in text.lower()]}")
    deviations = csv_rows(A3 / "provenance" / "protocol_deviations.csv")
    unresolved = [r for r in deviations if r.get("impact", "").lower() in {"primary", "critical", "major"}
                  and r.get("resolution", "").lower() not in {"resolved", "rerun_complete", "not_applicable"}]
    add("protocol", "no unresolved primary protocol deviation", not unresolved, f"unresolved={len(unresolved)}")

    datasets = csv_rows(A3 / "datasets" / "dataset_registry.csv")
    valid = []
    for row in datasets:
        path = ROOT / row.get("local_path", "")
        split_path = ROOT / row.get("split_manifest", "")
        if (row.get("source_url") and row.get("license") and re.fullmatch(r"[0-9a-f]{64}", row.get("sha256", ""))
                and path.is_file() and sha256(path) == row["sha256"] and split_path.is_file()):
            valid.append(row)
    add("datasets", "licensed checksummed datasets and split manifests", len(valid) >= 2,
        f"valid={len(valid)}/{len(datasets)}")
    leakage = load_json(A3 / "datasets" / "leakage_check.json")
    add("datasets", "train/calibration/dev/test separation verified", isinstance(leakage, dict)
        and leakage.get("passed") is True and leakage.get("frozen_test_access_before_freeze", 1) == 0,
        json.dumps(leakage, sort_keys=True)[:500] if leakage else "missing")


def read_manifest_records() -> dict[str, list[dict[str, Any]]]:
    result: dict[str, list[dict[str, Any]]] = defaultdict(list)
    root = A3 / "experiments" / "manifests" / "final"
    if not root.is_dir():
        return result
    for path in sorted(root.glob("*.json")):
        doc = load_json(path)
        docs = doc if isinstance(doc, list) else [doc]
        for item in docs:
            if not isinstance(item, dict):
                continue
            experiment = str(item.get("experiment", path.stem)).upper()
            hashes_ok = all(re.fullmatch(r"[0-9a-f]{64}", str(item.get(k, ""))) for k in
                            ("protocol_sha256", "code_sha256", "config_sha256", "data_sha256"))
            raw_files = item.get("raw_files", [])
            raw_ok = bool(raw_files)
            for raw in raw_files:
                p = ROOT / raw.get("path", "")
                raw_ok = raw_ok and p.is_file() and sha256(p) == raw.get("sha256")
            item["_valid"] = item.get("status") == "valid" and hashes_ok and raw_ok
            result[experiment].append(item)
    return result


def experiment_checks(strict: bool) -> None:
    manifests = read_manifest_records()
    for exp in [f"E{i}" for i in range(1, 8)]:
        valid = [m for m in manifests.get(exp, []) if m.get("_valid")]
        add("experiments", f"{exp} valid checksummed manifests", bool(valid), f"valid={len(valid)}, total={len(manifests.get(exp, []))}")
    if not strict:
        return

    e1 = [m for m in manifests.get("E1", []) if m.get("_valid") and m.get("cell_type") == "principal"]
    e1_cells: dict[str, dict[int, int]] = defaultdict(dict)
    for m in e1:
        e1_cells[str(m.get("cell_id"))][int(m.get("seed", -1))] = int(m.get("analyzed_events", 0))
    e1_ok = bool(e1_cells) and all(len(v) >= 10 and min(v.values()) >= 10_000 for v in e1_cells.values())
    methods = {str(m.get("method")) for m in e1}
    required_budget = {"no_budget", "settled_only", "mean", "mean_margin", "fixed_quantile", "strict_max",
                       "fixed_estimate", "adaptive_quantile", "expected_cost_router", "oracle_future_cost", "gov_ar"}
    add("experiments", "E1 principal floors and admission baselines", e1_ok and required_budget <= methods,
        f"cells={len(e1_cells)}, methods_missing={sorted(required_budget-methods)}")

    e2 = [m for m in manifests.get("E2", []) if m.get("_valid") and m.get("cell_type") == "principal"]
    e2_groups: dict[tuple[str, int], int] = defaultdict(int)
    tenants: set[str] = set()
    for m in e2:
        e2_groups[(str(m.get("scenario")), int(m.get("seed", -1)))] += int(m.get("shared_stream_events", 0))
        tenants.update(str(x) for x in m.get("tenants", []))
    required_scenarios = {"balanced", "noisy_tenant", "asymmetric_budgets", "long_output", "strict_governance",
                          "window_renewal", "policy_change", "high_priority_capacity"}
    e2_ok = required_scenarios <= {x[0] for x in e2_groups} and len(tenants) >= 6
    for scenario in required_scenarios:
        values = [n for (s, _), n in e2_groups.items() if s == scenario]
        e2_ok = e2_ok and len(values) >= 10 and min(values) >= 50_000
    add("experiments", "E2 scenarios, tenants, seeds, event floors", e2_ok,
        f"scenarios={sorted({x[0] for x in e2_groups})}, tenants={len(tenants)}")

    e4 = [m for m in manifests.get("E4", []) if m.get("_valid")]
    fault_counts = Counter(str(m.get("scenario")) for m in e4 if m.get("post_fix") is True and m.get("invariants_passed") is True)
    add("experiments", "E4 20 fault scenarios x100 post-fix repetitions", len(fault_counts) >= 20 and min(fault_counts.values(), default=0) >= 100,
        f"scenarios={len(fault_counts)}, min_repetitions={min(fault_counts.values(), default=0)}")

    e5 = [m for m in manifests.get("E5", []) if m.get("_valid") and m.get("cell_type") == "principal"]
    perf: dict[str, set[int]] = defaultdict(set)
    durations: list[int] = []
    for m in e5:
        perf[str(m.get("load_point"))].add(int(m.get("cluster_recreation", 0)))
        durations.append(int(m.get("steady_seconds", 0)))
    add("experiments", "E5 three recreations and ten-minute steady points", bool(perf)
        and all(len(x) >= 3 for x in perf.values()) and min(durations, default=0) >= 600,
        f"points={len(perf)}, min_recreations={min((len(x) for x in perf.values()), default=0)}, min_seconds={min(durations, default=0)}")

    e6 = [m for m in manifests.get("E6", []) if m.get("_valid")]
    live: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
    for m in e6:
        live[str(m.get("deployment"))][str(m.get("window_id"))] += int(m.get("successful_requests", 0))
    live_ok = len(live) >= 3 and all(sum(w.values()) >= 300 and len(w) >= 3 for w in live.values())
    add("experiments", "E6 deployment/request/window floors", live_ok,
        f"deployments={len(live)}, counts={{{', '.join(f'{k}:{sum(v.values())}/{len(v)}w' for k,v in live.items())}}}")

    e7 = [m for m in manifests.get("E7", []) if m.get("_valid")]
    required_ablations = {"no_inflight_liability", "no_risk_allocation", "mean_only", "no_drift_fallback", "no_tenant_isolation",
                          "soft_governance", "no_atomic_transaction", "no_switch_penalty", "no_settlement_idempotence"}
    ablations = {str(m.get("ablation")) for m in e7}
    add("experiments", "E7 required ablations", required_ablations <= ablations,
        f"missing={sorted(required_ablations-ablations)}")

    accounting = csv_rows(A3 / "provenance" / "experiment_registry.csv")
    bad = [r for r in accounting if r.get("status") in {"valid", "complete"} and not all(r.get(k) for k in
           ("run_id", "code_sha", "config_sha256", "data_sha256", "manifest_path", "raw_path"))]
    add("experiments", "exact run accounting and hashes", len(accounting) > 0 and not bad,
        f"rows={len(accounting)}, incomplete_valid_rows={len(bad)}")


def statistics_claim_checks() -> None:
    ok, detail = review_clear(A3 / "reviews" / "statistical_audit.json")
    add("statistics", "independent raw-only statistical audit", ok, detail)
    stat = load_json(A3 / "analysis" / "final" / "statistical_results.json")
    required = {"observation_counts", "exclusions", "primary_metrics", "confidence_intervals", "tests",
                "holm_correction", "figure_source_hashes", "negative_null_results"}
    add("statistics", "complete generated statistical output", isinstance(stat, dict) and required <= set(stat),
        f"missing={sorted(required - set(stat or {}))}")

    claims = csv_rows(A3 / "provenance" / "claims_to_evidence.csv")
    required_cols = ("claim_id", "claim_text", "run_ids", "raw_files", "processed_files", "analysis_script",
                     "statistical_output", "figure_table_ids", "status", "reviewer")
    valid = [r for r in claims if all(r.get(k) for k in required_cols) and r.get("status") == "supported"]
    add("claims", "principal claims mapped end-to-end", bool(valid) and len(valid) == len(claims),
        f"supported={len(valid)}/{len(claims)}")


def manuscript_release_checks(strict: bool) -> None:
    journal = load_json(A3 / "journal" / "selection.json")
    journal_ok = isinstance(journal, dict) and journal.get("current_q1") is True and all(journal.get(k) for k in
        ("journal", "publisher", "scope_source_url", "quartile_source_url", "quartile_year", "verified_at",
         "author_instructions_url", "template_sha256"))
    add("manuscript", "current in-scope Q1 target verified", journal_ok,
        json.dumps(journal, sort_keys=True)[:700] if journal else "missing")

    tex = A3 / "manuscript" / "main.tex"
    text = tex.read_text(encoding="utf-8", errors="ignore") if file_ok(tex) else ""
    plain = re.sub(r"%.*", " ", text)
    plain = re.sub(r"\\[A-Za-z@]+(?:\[[^]]*\])?(?:\{[^{}]*\})?", " ", plain)
    words = re.findall(r"\b[A-Za-z][A-Za-z'-]*\b", plain)
    placeholders = re.findall(r"(?i)\b(?:TODO|TBD|placeholder|scaffold|not yet evaluated)\b", text)
    add("manuscript", "substantive 8k-14k manuscript with no placeholders", 8000 <= len(words) <= 14000 and not placeholders,
        f"words={len(words)}, placeholders={sorted(set(x.lower() for x in placeholders))}")

    pdf = A3 / "artifacts" / "GOV_AR_article.pdf"
    pages = 0
    if file_ok(pdf):
        rc, out = run(["pdfinfo", str(pdf)])
        m = re.search(r"(?m)^Pages:\s+(\d+)", out)
        pages = int(m.group(1)) if rc == 0 and m else 0
    add("manuscript", "final PDF is substantive", file_ok(pdf, 50_000) and pages >= 10, f"pages={pages}, bytes={pdf.stat().st_size if pdf.exists() else 0}")

    artifacts = ["GOV_AR_article.pdf", "GOV_AR_overleaf.zip", "GOV_AR_replication_package.zip", "FINAL_REPORT.md",
                 "REPRODUCTION.md", "EXPERIMENT_SUMMARY.csv", "STATISTICAL_SUMMARY.md", "LITERATURE_REVIEW.md",
                 "AZURE_VALIDATION_SUMMARY.md", "IMAGE_DIGESTS.csv", "TEST_REPORT.md", "CLAIMS_TO_EVIDENCE.csv",
                 "CHANGELOG_ARTICLE3.md"]
    missing = [x for x in artifacts if not file_ok(A3 / "artifacts" / x)]
    add("release", "required final artifacts", not missing, f"missing={missing}")

    for name in ("GOV_AR_overleaf.zip", "GOV_AR_replication_package.zip"):
        path = A3 / "artifacts" / name
        safe = False
        detail = "missing"
        if file_ok(path):
            try:
                with zipfile.ZipFile(path) as zf:
                    names = zf.namelist()
                    traversal = [n for n in names if n.startswith("/") or ".." in Path(n).parts]
                    absolute_hits = []
                    secret_hits = []
                    for info in zf.infolist():
                        if info.file_size > 5_000_000 or info.is_dir():
                            continue
                        try:
                            payload = zf.read(info).decode("utf-8")
                        except (UnicodeDecodeError, OSError):
                            continue
                        if str(ROOT) in payload or re.search(r"(?i)[A-Z]:\\Users\\", payload):
                            absolute_hits.append(info.filename)
                        if re.search(r"-----BEGIN [A-Z ]*PRIVATE KEY-----|\bgh[opusr]_[A-Za-z0-9]{30,}\b|\bsk-[A-Za-z0-9_-]{24,}\b", payload):
                            secret_hits.append(info.filename)
                    safe = not traversal and not absolute_hits and not secret_hits
                    detail = f"entries={len(names)}, traversal={traversal[:3]}, absolute={absolute_hits[:3]}, secrets={secret_hits[:3]}"
            except zipfile.BadZipFile as exc:
                detail = str(exc)
        add("security", f"{name} safe paths and secret scan", safe, detail)

    ok, detail = review_clear(A3 / "reviews" / "release_verification.json")
    add("review", "clean-checkout release verification", ok, detail)
    if strict:
        clean = load_json(A3 / "reproduction" / "clean_checkout_results.json")
        add("release", "clean checkout rebuild/reproduction", isinstance(clean, dict) and clean.get("passed") is True
            and all(clean.get(k) is True for k in ("software_build", "kind_path", "tables_figures", "paper", "overleaf_zip", "replication_primary")),
            json.dumps(clean, sort_keys=True)[:700] if clean else "missing")


def write_status(path: Path, strict: bool, success: bool) -> None:
    groups: dict[str, dict[str, int]] = defaultdict(lambda: {"passed": 0, "failed": 0})
    for check in CHECKS:
        groups[check.group]["passed" if check.passed else "failed"] += 1
    rc, head = run(["git", "rev-parse", "HEAD"])
    rc2, branch = run(["git", "branch", "--show-current"])
    payload = {
        "schema_version": 1,
        "generated_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
        "generated_by": "article3/tools/q1_gate.py",
        "strict": strict,
        "gate_passed": success,
        "branch": branch if rc2 == 0 else None,
        "head_sha": head if rc == 0 else None,
        "summary": {"passed": sum(c.passed for c in CHECKS), "failed": sum(not c.passed for c in CHECKS)},
        "groups": groups,
        "checks": [asdict(c) for c in CHECKS],
    }
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--strict", action="store_true", help="enforce all remote, experiment-floor, and release checks")
    parser.add_argument("--write-status", action="store_true", help="write derived article3/STATUS.json")
    parser.add_argument("--json", action="store_true", help="emit JSON instead of text")
    args = parser.parse_args()

    repository_checks(args.strict)
    literature_checks(args.strict)
    implementation_checks(args.strict)
    dataset_protocol_checks()
    experiment_checks(args.strict)
    statistics_claim_checks()
    manuscript_release_checks(args.strict)

    critical_failures = [c for c in CHECKS if c.critical and not c.passed]
    success = not critical_failures
    if args.write_status:
        write_status(A3 / "STATUS.json", args.strict, success)
    if args.json:
        print(json.dumps({"passed": success, "checks": [asdict(c) for c in CHECKS]}, indent=2))
    else:
        for c in CHECKS:
            print(f"[{'PASS' if c.passed else 'FAIL'}] {c.group}/{c.name}: {c.detail}")
        print(f"Q1_GATE={'PASS' if success else 'FAIL'} passed={sum(c.passed for c in CHECKS)} failed={len(critical_failures)}")
    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
