#!/usr/bin/env python3
"""Run a conservative, evidence-based gate over the Article 3 workspace.

The default mode writes a report and exits successfully so non-blocking work
can continue. ``--strict`` is intended for submission/release automation and
fails when prompt-scale evidence or external publication is missing.
"""

from __future__ import annotations

import argparse
import csv
import json
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--strict", action="store_true")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    checks: list[tuple[str, bool, str]] = []

    def check(name: str, passed: bool, detail: str) -> None:
        checks.append((name, passed, detail))

    required = [
        root / "artifacts/GOV_AR_article.pdf",
        root / "artifacts/GOV_AR_overleaf.zip",
        root / "artifacts/GOV_AR_replication_package.zip",
        root / "provenance/generated_artifacts.sha256",
        root / "experiments/registry/frozen_protocol.yaml",
    ]
    check("named_artifacts", all(p.is_file() and p.stat().st_size for p in required), "required PDFs/ZIPs/protocol exist")

    with (root / "literature/related_work_matrix.csv").open(newline="", encoding="utf-8") as handle:
        matrix = list(csv.DictReader(handle))
    schema_ok = {"verified_url", "verification_date"}.issubset(matrix[0]) if matrix else False
    check("literature_schema", schema_ok, f"{len(matrix)} references; target is 35-50")
    check("literature_target", len(matrix) >= 35, f"{len(matrix)}/35 minimum references")

    status = json.loads((root / "STATUS.json").read_text(encoding="utf-8"))
    allowed = {"PENDING", "RUNNING", "PASSED", "FAILED_RETRYABLE", "BLOCKED", "INVALIDATED", "COMPLETE"}
    states_ok = all(value in allowed for value in status.get("tasks", {}).values())
    check("status_state_vocabulary", states_ok, f"{len(status.get('tasks', {}))} task states")
    check("ghcr_publish", "docker_helm_release" in status.get("tasks", {}) and status["tasks"]["docker_helm_release"] == "COMPLETE", "GHCR/OCI publication requires external package-write access")

    report = root / "reports/PROMPT_RELEASE_GATE.md"
    lines = ["# Article 3 Prompt Release Gate", "", "This report is generated from repository evidence; it does not infer success from planned protocol values.", ""]
    for name, passed, detail in checks:
        lines.append(f"- {'PASS' if passed else 'FAIL'} `{name}`: {detail}")
    lines += ["", f"Overall strict result: {'PASS' if all(item[1] for item in checks) else 'FAIL'}"]
    report.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    return 0 if (not args.strict or all(item[1] for item in checks)) else 1


if __name__ == "__main__":
    raise SystemExit(main())
