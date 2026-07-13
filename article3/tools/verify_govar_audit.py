#!/usr/bin/env python3
"""Independently verify exported GOV-AR per-tenant audit chains."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import struct
import sys
from datetime import datetime, timezone
from pathlib import Path

ACTORS = {"ADMISSION", "GATEWAY", "RECONCILER", "CORRECTION", "AUTHORITY", "REGISTRY", "CALIBRATION"}
KINDS = {"RESERVE", "DISPATCH", "SETTLE", "CANCEL", "EXPIRY", "ROLLOVER", "BUDGET_ADJUSTMENT", "COHORT_REGISTRATION", "CALIBRATION_PUBLICATION", "DRIFT_CHANGE", "RECONCILIATION"}
SHA = re.compile(r"^[0-9a-f]{64}$")
REASON = re.compile(r"^[a-z0-9_:-]{1,64}$")

HASH_FIELDS = (
    "tenant_id", "sequence", "event_id", "event_kind", "payload_sha256",
    "request_id", "workload_uid", "provider_attempt_id", "actor_class", "reason",
    "before_state_sha256", "after_state_sha256", "policy_version",
    "pricing_snapshot_sha256", "route_snapshot_sha256", "cap_evidence_sha256",
    "cohort_sha256", "software_sha256", "calibration_sha256",
    "previous_entry_sha256", "committed_at",
)


def canonical_time(value: str) -> str:
    if not isinstance(value, str) or not value:
        raise ValueError("committed_at is required")
    match = re.fullmatch(r"(.+T\d{2}:\d{2}:\d{2})(?:\.(\d{1,6}))?(Z|[+-]\d{2}:\d{2})", value)
    if not match:
        raise ValueError("committed_at is not RFC3339 with microsecond-or-better database precision")
    fraction = match.group(2) or ""
    parse_value = match.group(1) + ("." + fraction.ljust(6, "0") if fraction else "") + ("+00:00" if match.group(3) == "Z" else match.group(3))
    parsed = datetime.fromisoformat(parse_value)
    if parsed.tzinfo is None:
        raise ValueError("committed_at lacks timezone")
    parsed = parsed.astimezone(timezone.utc)
    base = parsed.strftime("%Y-%m-%dT%H:%M:%S")
    fraction = f"{parsed.microsecond:06d}".rstrip("0")
    return base + ("." + fraction if fraction else "") + "Z"


def entry_hash(row: dict) -> str:
    values = ["govar-tenant-audit-v2"]
    for name in HASH_FIELDS:
        value = row.get(name, "")
        if name == "sequence":
            value = str(value)
        elif name == "committed_at":
            value = canonical_time(value)
        elif value is None:
            value = ""
        elif not isinstance(value, str):
            raise ValueError(f"{name} must be a string")
        values.append(value)
    digest = hashlib.sha256()
    for value in values:
        encoded = value.encode("utf-8")
        digest.update(struct.pack(">Q", len(encoded)))
        digest.update(encoded)
    return digest.hexdigest()


def validate_digest(name: str, value: str, optional: bool = False) -> None:
    if optional and value == "":
        return
    if not isinstance(value, str) or not SHA.fullmatch(value):
        raise ValueError(f"{name} is not a canonical SHA-256")


def verify(rows: list[dict]) -> dict:
    if not isinstance(rows, list):
        raise ValueError("export must be a JSON array")
    if not rows:
        raise ValueError("audit export is empty")
    by_tenant: dict[str, list[dict]] = {}
    event_ids: set[str] = set()
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError("every audit row must be an object")
        tenant = row.get("tenant_id", "")
        if not isinstance(tenant, str) or not tenant.strip():
            raise ValueError("tenant_id is required")
        event_id = row.get("event_id", "")
        if not isinstance(event_id, str) or not event_id or event_id in event_ids:
            raise ValueError("event_id is empty or not globally unique")
        event_ids.add(event_id)
        if row.get("event_kind") not in KINDS or row.get("actor_class") not in ACTORS:
            raise ValueError(f"event {event_id} has an unknown kind or actor")
        if not REASON.fullmatch(str(row.get("reason", ""))):
            raise ValueError(f"event {event_id} has an unbounded reason")
        for name in ("payload_sha256", "after_state_sha256", "entry_sha256"):
            validate_digest(name, row.get(name, ""))
        for name in ("before_state_sha256", "pricing_snapshot_sha256", "route_snapshot_sha256", "cap_evidence_sha256", "cohort_sha256", "software_sha256", "calibration_sha256", "previous_entry_sha256"):
            validate_digest(name, row.get(name, ""), optional=True)
        by_tenant.setdefault(tenant, []).append(row)

    heads: dict[str, str] = {}
    for tenant, chain in sorted(by_tenant.items()):
        previous = ""
        for index, row in enumerate(chain, 1):
            if row.get("sequence") != index:
                raise ValueError(f"tenant {tenant} sequence is not contiguous at {index}")
            if row.get("previous_entry_sha256", "") != previous:
                raise ValueError(f"tenant {tenant} predecessor mismatch at {index}")
            calculated = entry_hash(row)
            if calculated != row.get("entry_sha256"):
                raise ValueError(f"tenant {tenant} entry hash mismatch at {index}")
            previous = calculated
        heads[tenant] = previous
    return {"schema": "govar-audit-verification-v1", "verified": True,
            "tenant_count": len(by_tenant), "event_count": len(rows), "chain_heads": heads}


def canonical_json(value: object) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("input", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--checkpoint", type=Path,
                        help="expected JSON with event_count and chain_heads; detects deletion/wrong head")
    args = parser.parse_args()
    raw = args.input.read_bytes()
    try:
        document = json.loads(raw)
        rows = document.get("entries") if isinstance(document, dict) else document
        result = verify(rows)
        if args.checkpoint:
            checkpoint = json.loads(args.checkpoint.read_text(encoding="utf-8"))
            if checkpoint.get("event_count") != result["event_count"] or checkpoint.get("chain_heads") != result["chain_heads"]:
                raise ValueError("export does not match the expected audit checkpoint")
        result["input_sha256"] = hashlib.sha256(raw).hexdigest()
        result["result_sha256"] = hashlib.sha256(canonical_json(result)).hexdigest()
        status = 0
    except Exception as exc:  # deliberate fail-closed command boundary
        result = {"schema": "govar-audit-verification-v1", "verified": False,
                  "input_sha256": hashlib.sha256(raw).hexdigest(), "error": str(exc)}
        result["result_sha256"] = hashlib.sha256(canonical_json(result)).hexdigest()
        status = 1
    encoded = json.dumps(result, sort_keys=True, indent=2) + "\n"
    if args.output:
        args.output.write_text(encoded, encoding="utf-8")
    else:
        sys.stdout.write(encoded)
    return status


if __name__ == "__main__":
    raise SystemExit(main())
