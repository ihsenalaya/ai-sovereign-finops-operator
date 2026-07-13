#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

recompute_dir="$(mktemp -d)"
trap 'rm -rf "$recompute_dir"' EXIT INT TERM
cp article3/formal/ledger_model.py "$recompute_dir/ledger_model.py"
python3 "$recompute_dir/ledger_model.py"
python3 - "$recompute_dir/results.json" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

model = Path("article3/formal/ledger_model.py")
recomputed = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
recorded = json.loads(Path("article3/formal/results.json").read_text(encoding="utf-8"))
required = {
    "conditional_strict_ledger_feasibility",
    "one_effective_settlement_finalization_and_correction",
    "outbox_claim_cancel_delivery_relation",
    "record_aggregate_equality",
    "tenant_and_workload_uid_isolation",
    "residual_correction_exposure_retained",
    "provisional_rollover_guard_prevents_credit_reuse",
    "postfinal_correction_external_debt_visible",
    "required_transition_coverage",
    "provider_route_ownership",
    "route_snapshot_integrity",
    "dispatch_uses_reserved_snapshot",
    "duplicate_admit_byte_stable_route_response",
}
actual_hash = hashlib.sha256(model.read_bytes()).hexdigest()
for result in (recomputed, recorded):
    assert result.get("schema_version", 0) >= 4
    assert result.get("exit_code") == 0
    assert result.get("failures") == []
    assert result.get("model_sha256") == actual_hash
    assert required <= set(result.get("invariants", []))
    assert result.get("states_checked", 0) > 0
    assert result.get("transitions_checked", 0) > 0
    assert len(result.get("scenario_traces_checked", [])) >= 14
    assert result.get("route_snapshot_negative_checks") == 12
    assert result.get("invariants_checked") == len(result.get("invariants", []))
    assert len(result.get("invariants", [])) == len(set(result.get("invariants", [])))
for key in sorted(set(recomputed) | set(recorded)):
    if key == "checked_at_utc":
        continue
    assert recomputed.get(key) == recorded.get(key), f"recorded formal result differs at {key}"
print(f"formal_check=PASS model_sha256={actual_hash} states={recomputed['states_checked']} transitions={recomputed['transitions_checked']} recorded_result_unchanged=true")
PY
