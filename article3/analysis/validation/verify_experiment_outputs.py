#!/usr/bin/env python3
"""Validate the current GOV-AR scaffold experiment outputs."""

from __future__ import annotations

import json
import sys
from pathlib import Path


EXPECTED_VARIANTS = {
    "E0_smoke.json": ("experiment_id", "variant", "checks"),
    "E1_mean_std.json": ("experiment_id", "variant", "config", "metrics", "events"),
    "E1_quantile.json": ("experiment_id", "variant", "config", "metrics", "events"),
    "E2_mean_std.json": ("experiment_id", "variant", "config", "metrics", "events"),
    "E2_quantile.json": ("experiment_id", "variant", "config", "metrics", "events"),
    "E3_drift.json": ("experiment_id", "variant", "drift", "metrics", "events"),
    "E4_faults.json": (
        "experiment_id",
        "variant",
        "duplicate_settlement",
        "reservation_expiry",
        "telemetry_fault",
    ),
    "E5_scalability.json": ("experiment_id", "rows"),
    "E7_ablation.json": ("experiment_id", "rows"),
}

EXPECTED_PROCESSED = {
    "E0_smoke_summary.json",
    "E1_mean_std_summary.json",
    "E1_quantile_summary.json",
    "E2_mean_std_summary.json",
    "E2_quantile_summary.json",
    "E1_mean_std_campaign.json",
    "E1_quantile_campaign.json",
    "E2_mean_std_campaign.json",
    "E2_quantile_campaign.json",
    "E1_matrix.json",
    "E2_matrix.json",
    "E1_comparison.json",
    "E2_comparison.json",
    "E3_drift_summary.json",
    "E4_faults_summary.json",
    "E5_scalability_summary.json",
    "E7_ablation_summary.json",
}


def load_json(path: Path):
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def validate_raw(raw_dir: Path) -> None:
    for filename, fields in EXPECTED_VARIANTS.items():
        path = raw_dir / filename
        if not path.is_file():
            raise SystemExit(f"missing raw artifact: {path}")
        payload = load_json(path)
        missing = [field for field in fields if field not in payload]
        if missing:
            raise SystemExit(f"{path} missing keys: {missing}")
        if "events" in payload and not isinstance(payload["events"], list):
            raise SystemExit(f"{path} has non-list events payload")
        if "checks" in payload and not isinstance(payload["checks"], list):
            raise SystemExit(f"{path} has non-list checks payload")


def validate_processed(processed_dir: Path) -> None:
    for filename in EXPECTED_PROCESSED:
        path = processed_dir / filename
        if not path.is_file():
            raise SystemExit(f"missing processed artifact: {path}")
        payload = load_json(path)
        if not isinstance(payload, dict):
            raise SystemExit(f"{path} must decode to a JSON object")


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("usage: verify_experiment_outputs.py <raw_dir> <processed_dir>", file=sys.stderr)
        return 2
    raw_dir = Path(argv[1])
    processed_dir = Path(argv[2])
    validate_raw(raw_dir)
    validate_processed(processed_dir)
    print("verification ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
