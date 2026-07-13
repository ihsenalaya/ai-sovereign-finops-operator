#!/usr/bin/env python3
"""Fail-closed structural validation for the pre-freeze baseline registry."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[2]
REGISTRY = ROOT / "article3/experiments/baseline_registry.json"
AUDIT = ROOT / "article3/reviews/baseline_implementation_audit.json"
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")

BUDGET_METHODS = [
    "no_budget", "settled_only", "mean", "mean_margin", "fixed_quantile",
    "strict_max", "fixed_estimate", "adaptive_quantile",
    "expected_cost_router", "oracle_future_cost", "gov_ar",
]
ROUTING_METHODS = [
    "always_premium", "always_cheapest", "cheapest_governance_compliant",
    "current_operator_weighted_score", "routellm_or_hybridllm_compatible",
    "pilot_or_closest_budget_router", "contextual_knn_router",
    "oracle_quality_cost", "gov_ar",
]
METHOD_FIELDS = {
    "registry_id", "protocol_id", "information_set",
    "reservation_rule", "admission_rule", "production_mapping", "parameters",
    "development_selection", "selected_feedback", "oracle_access",
    "expected_invariants", "acceptance_tests", "blockers", "literature_refs",
}


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_object(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"expected JSON object: {path}")
    return value


def validate_registry() -> dict[str, object]:
    value = load_object(REGISTRY)
    require(value.get("schema_version") == "govar-baseline-registry-v1", "unexpected registry schema")
    require(value.get("status") == "pre_freeze_specification", "registry must remain pre-freeze")
    protocol = value.get("protocol")
    require(isinstance(protocol, dict) and protocol.get("frozen_test_authorized") is False,
            "baseline registry may not authorize frozen-test access")
    budget = value.get("budget_admission_baselines")
    routing = value.get("routing_baselines")
    require(isinstance(budget, list) and isinstance(routing, list), "method arrays are missing")
    require([row.get("protocol_id") for row in budget] == BUDGET_METHODS,
            "budget baseline identity/order differs from protocol")
    require([row.get("protocol_id") for row in routing] == ROUTING_METHODS,
            "routing baseline identity/order differs from protocol")
    methods = budget + routing
    registry_ids = [row.get("registry_id") for row in methods]
    require(len(registry_ids) == len(set(registry_ids)) == 20, "namespaced method IDs are not unique")
    for row in methods:
        require(isinstance(row, dict) and METHOD_FIELDS <= set(row),
                f"baseline record is incomplete: {row.get('registry_id') if isinstance(row, dict) else '<nonobject>'}")
        if row["registry_id"].startswith("budget_admission/"):
            require(all(isinstance(row.get(field), str) and row[field]
                        for field in ("role", "routing_rule")),
                    f"budget baseline behavior is incomplete: {row['registry_id']}")
        else:
            require(isinstance(row.get("rule"), str) and row["rule"],
                    f"routing baseline rule is incomplete: {row['registry_id']}")
        expected = row["registry_id"].split("/", 1)[-1]
        require(expected == row["protocol_id"], f"registry/protocol ID mismatch: {row['registry_id']}")
        for field in ("parameters", "expected_invariants", "acceptance_tests", "blockers", "literature_refs"):
            require(isinstance(row[field], list), f"{row['registry_id']} {field} is not a list")
        require(row["acceptance_tests"], f"{row['registry_id']} has no acceptance tests")
    compatibility = value.get("compatibility_and_exclusion_rules")
    require(isinstance(compatibility, dict) and compatibility, "compatibility/exclusion rules are absent")
    companions = value.get("companion_practical_comparators")
    require(isinstance(companions, list) and len(companions) == 3,
            "three companion practical comparators are required")
    require(isinstance(value.get("global_acceptance_matrix"), list)
            and "independent_baseline_review" in value["global_acceptance_matrix"],
            "independent baseline review is not an acceptance condition")
    require(isinstance(value.get("freeze_blockers"), list) and value["freeze_blockers"],
            "open implementation blockers must remain explicit")
    return {
        "passed": True,
        "registry_sha256": digest(REGISTRY),
        "budget_methods": len(budget),
        "routing_methods": len(routing),
        "namespaced_methods": len(registry_ids),
        "companion_comparators": len(companions),
    }


def validate_implementation_audit(registry_sha256: str) -> dict[str, object]:
    audit = load_object(AUDIT)
    require(audit.get("verdict") == "approved", "baseline implementation audit is not approved")
    require(audit.get("unresolved_critical_major") == 0, "baseline implementation audit has unresolved findings")
    require(audit.get("registry_sha256") == registry_sha256, "baseline audit registry hash is stale")
    sources = audit.get("reviewed_source_hashes")
    require(isinstance(sources, dict) and sources, "baseline audit has no source binding")
    for relative, expected in sources.items():
        path = ROOT / str(relative)
        require(path.is_file() and isinstance(expected, str) and SHA256_RE.fullmatch(expected)
                and digest(path) == expected, f"baseline audit source mismatch: {relative}")
    return {"audit": str(AUDIT.relative_to(ROOT)), "reviewed_sources": len(sources)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--require-implementation-audit", action="store_true")
    args = parser.parse_args()
    try:
        result = validate_registry()
        if args.require_implementation_audit:
            result.update(validate_implementation_audit(str(result["registry_sha256"])))
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"baseline_registry_validation=FAIL reason={exc}", file=sys.stderr)
        raise SystemExit(1)
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
