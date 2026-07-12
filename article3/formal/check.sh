#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

python3 article3/formal/ledger_model.py
python3 - <<'PY'
import hashlib
import json
from pathlib import Path

model = Path("article3/formal/ledger_model.py")
result = json.loads(Path("article3/formal/results.json").read_text(encoding="utf-8"))
required = {
    "conditional_strict_ledger_feasibility",
    "one_effective_settlement_finalization_and_correction",
    "outbox_claim_cancel_delivery_relation",
    "record_aggregate_equality",
    "tenant_and_workload_uid_isolation",
    "residual_correction_exposure_retained",
    "provisional_rollover_guard_prevents_credit_reuse",
    "required_transition_coverage",
}
actual_hash = hashlib.sha256(model.read_bytes()).hexdigest()
assert result.get("schema_version", 0) >= 4
assert result.get("exit_code") == 0
assert result.get("failures") == []
assert result.get("model_sha256") == actual_hash
assert required <= set(result.get("invariants", []))
assert result.get("states_checked", 0) > 0
assert result.get("transitions_checked", 0) > 0
assert len(result.get("scenario_traces_checked", [])) >= 9
print(f"formal_check=PASS model_sha256={actual_hash} states={result['states_checked']} transitions={result['transitions_checked']}")
PY
