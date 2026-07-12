#!/usr/bin/env python3
"""Fail-closed structural and optional live validation of the literature corpus."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import re
import ssl
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[2]
LIT = ROOT / "article3" / "literature"
OUT = LIT / "validation_report.json"
CORPUS_FILES = ("search_log.csv", "screening.csv", "related_work_matrix.csv",
                "novelty_assessment.md", "references.bib", "rejected_references.csv")


def rows(name: str) -> list[dict[str, str]]:
    with (LIT / name).open(newline="", encoding="utf-8") as fh:
        return list(csv.DictReader(fh))


def bib_keys(text: str) -> list[str]:
    return re.findall(r"(?mi)^\s*@\w+\s*\{\s*([^,\s]+)\s*,", text)


def corpus_sha256() -> str:
    digest = hashlib.sha256()
    for name in CORPUS_FILES:
        digest.update(name.encode("utf-8") + b"\0")
        digest.update((LIT / name).read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


def fetch(url: str) -> tuple[str, bool, str]:
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Article3-literature-verifier/1.0",
                                                   "Range": "bytes=0-16383"})
        with urllib.request.urlopen(req, timeout=20, context=ssl.create_default_context()) as response:
            return url, 200 <= response.status < 400, f"HTTP {response.status} {response.geturl()}"
    except Exception as exc:  # exact error is evidence; no credentials are sent
        return url, False, f"{type(exc).__name__}: {exc}"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--live", action="store_true")
    args = ap.parse_args()

    required = ["search_log.csv", "screening.csv", "related_work_matrix.csv",
                "novelty_assessment.md", "references.bib", "rejected_references.csv"]
    issues: list[str] = []
    missing = [x for x in required if not (LIT / x).is_file()]
    if missing:
        print(f"missing files: {missing}")
        return 1

    screening = rows("screening.csv")
    searches = rows("search_log.csv")
    matrix = rows("related_work_matrix.csv")
    bib_text = (LIT / "references.bib").read_text(encoding="utf-8")
    keys = bib_keys(bib_text)
    included = [r for r in screening if r.get("decision", "").lower() in {"include", "included", "retain", "retained"}]
    verified = [r for r in included if r.get("metadata_verified", "").lower() == "true"
                and r.get("relevant", "").lower() == "true"]
    peer = [r for r in verified if r.get("peer_reviewed", "").lower() == "true"]

    if len(verified) < 40: issues.append(f"verified relevant references={len(verified)} < 40")
    if not verified or len(peer) <= len(verified) / 2: issues.append(f"peer reviewed={len(peer)}/{len(verified)} is not a majority")
    if len(keys) < 40: issues.append(f"BibTeX entries={len(keys)} < 40")
    if len(keys) != len(set(keys)): issues.append("duplicate BibTeX keys")

    detail_by_id = {r.get("id", ""): r for r in matrix}
    required_fields = ["id", "title", "authors", "year", "venue", "identifier", "review_status",
                       "primary_source_url", "problem", "assumptions", "method", "datasets", "baselines",
                       "main_results", "limitations", "gov_ar_difference", "code_data", "verification_date"]
    for i, row in enumerate(verified, 2):
        detail = detail_by_id.get(row.get("id", ""), {})
        absent = [f for f in required_fields if not detail.get(f, "").strip()]
        if absent: issues.append(f"retained record {row.get('id')} missing matrix fields {absent}")
        key = row.get("bib_key") or detail.get("bib_key") or row.get("id")
        if key not in keys: issues.append(f"retained record {row.get('id')} key not in BibTeX: {key}")
        url = row.get("primary_source_url", "")
        parsed = urlparse(url)
        if parsed.scheme != "https" or not parsed.netloc: issues.append(f"screening row {i} invalid primary URL")
        status_text = f"{detail.get('venue','')} {detail.get('review_status','')}".lower()
        if row.get("peer_reviewed", "").lower() == "false" and not any(x in status_text for x in
                ("preprint", "arxiv", "submission", "under review", "workshop", "non-archival",
                 "industry", "product_documentation", "documentation", "whitepaper")):
            issues.append(f"screening row {i} non-peer-reviewed status not explicit in matrix venue/review status")

    verified_keys = {r.get("bib_key") or detail_by_id.get(r.get("id", ""), {}).get("bib_key") or r.get("id") for r in verified}
    orphan = sorted(set(keys) - verified_keys)
    if orphan: issues.append(f"BibTeX entries not in verified screening set: {orphan}")
    matrix_keys = {r.get("bib_key") or r.get("id") for r in matrix}
    unknown_matrix = sorted(x for x in matrix_keys if x and x not in verified_keys)
    if unknown_matrix: issues.append(f"related-work keys not verified: {unknown_matrix}")
    if len(matrix_keys & verified_keys) < 20: issues.append("related-work matrix covers fewer than 20 retained works")

    families = {(r.get("search_family") or r.get("source_family") or "").strip().lower() for r in searches
                if (r.get("search_family") or r.get("source_family"))}
    if len(families) < 10: issues.append(f"search families={len(families)} < 10")
    last_material = max((i for i, r in enumerate(searches)
                         if r.get("material_new_work", "").lower() == "true"), default=-1)
    saturated = [r for i, r in enumerate(searches) if i > last_material
                 and r.get("pass_type", "").lower() in {"expanded", "expanded_search", "citation_chase", "citation-chase"}
                 and r.get("material_new_work", "").lower() == "false"]
    if len(saturated) < 2: issues.append(f"saturation passes={len(saturated)} < 2")

    corpus_hash = corpus_sha256()
    reviews: dict[str, dict] = {}
    for filename in ("bibliography_verification.json", "literature_novelty_audit.json",
                     "scientific_red_team.json"):
        review_path = ROOT / "article3" / "reviews" / filename
        try:
            review = json.loads(review_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            review = {}
        reviews[filename] = review
        open_major = [f for f in review.get("findings", [])
                      if str(f.get("severity", "")).lower() in {"critical", "major"}
                      and str(f.get("status", "open")).lower() != "resolved"]
        if (str(review.get("verdict", "")).lower() not in {"approved", "pass", "passed"}
                or review.get("corpus_sha256") != corpus_hash or open_major):
            issues.append(f"{filename} is absent, stale, unapproved, or has unresolved critical/major findings")

    review = reviews["bibliography_verification.json"]
    verified_ids = review.get("verified_ids", [])
    if (int(review.get("independently_verified_records", 0)) < 40
            or not isinstance(verified_ids, list)
            or len(set(map(str, verified_ids))) != int(review.get("independently_verified_records", 0))):
        issues.append("independent bibliography record verification is incomplete or internally inconsistent")

    prose = (LIT / "novelty_assessment.md").read_text(encoding="utf-8")
    placeholders = re.findall(r"(?i)\b(?:TODO|TBD|placeholder|scaffold)\b", prose)
    if placeholders: issues.append(f"novelty assessment placeholders={sorted(set(placeholders))}")
    for marker in ("Token Budgets", "ParetoBandit", "R2-Router", "PILOT", "RACER", "CONCUR",
                   "Selective Deferred Routing", "Solo.io", "Keel", "AgentBudget", "falsif", "systems integration"):
        if marker.lower() not in prose.lower(): issues.append(f"novelty assessment missing marker: {marker}")

    live_results = []
    warnings: list[str] = []
    if args.live:
        urls = sorted({r["primary_source_url"] for r in verified if r.get("primary_source_url")})
        with ThreadPoolExecutor(max_workers=8) as pool:
            futures = [pool.submit(fetch, url) for url in urls]
            for future in as_completed(futures): live_results.append(future.result())
        failures = [x for x in live_results if not x[1]]
        # A transient source failure is surfaced but does not erase a current
        # independent comparison against the primary record. The corpus-bound
        # bibliography review is the fail-closed metadata control.
        reachable = len(live_results) - len(failures)
        if reachable < 40:
            issues.append(f"live reachable primary records={reachable}/{len(live_results)} < 40")
        elif failures:
            warnings.append(f"live primary URL failures={len(failures)}/{len(live_results)}; reachable={reachable}")

    report = {
        "passed": not issues,
        "verified_relevant": len(verified),
        "peer_reviewed": len(peer),
        "bibtex_entries": len(keys),
        "search_families": len(families),
        "saturation_passes": len(saturated),
        "post_material_saturation_search_ids": [r.get("search_id") for r in saturated],
        "corpus_sha256": corpus_hash,
        "live_checked": len(live_results),
        "live_results": [{"url": u, "reachable": ok, "detail": detail} for u, ok, detail in sorted(live_results)],
        "warnings": warnings,
        "issues": issues,
    }
    OUT.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in report.items() if k != "live_results"}, indent=2))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
