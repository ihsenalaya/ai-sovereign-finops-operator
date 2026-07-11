#!/usr/bin/env python3
from __future__ import annotations

import csv
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
INPUT = ROOT / "provenance" / "claims_to_evidence.csv"
OUTPUT = ROOT / "provenance" / "claims_to_evidence.prompt_schema.csv"


def main() -> None:
    rows = list(csv.DictReader(INPUT.open()))
    with OUTPUT.open("w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(
            [
                "claim_id",
                "section",
                "claim_text",
                "evidence_type",
                "experiment_id",
                "run_ids",
                "raw_files",
                "processed_files",
                "figure_or_table",
                "statistical_test",
                "assumptions",
                "limitations",
                "status",
            ]
        )
        for row in rows:
            claim_id = row["claim_id"]
            section = "audit" if claim_id.startswith("A3-AUDIT") else "gov_ar"
            writer.writerow(
                [
                    claim_id,
                    section,
                    row["claim_text"],
                    row["evidence_type"],
                    "",
                    "",
                    row["evidence_path"] if "raw" in row["evidence_type"] else "",
                    row["evidence_path"] if "raw" not in row["evidence_type"] else "",
                    "",
                    "",
                    "",
                    row.get("notes", ""),
                    "SUPPORTED" if row["status"] == "verified" else "DRAFT",
                ]
            )
    print("wrote", OUTPUT)


if __name__ == "__main__":
    main()
