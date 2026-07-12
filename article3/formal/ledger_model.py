#!/usr/bin/env python3
"""Exhaustive finite-state model for the GOV-AR reservation ledger.

The model is intentionally small and fail-closed. It checks the deterministic
ledger safety obligations used by Phase C, not the empirical calibration claim.
Assumption A1 mirrors strict mode: every authoritative settlement cost is less
than or equal to the pre-dispatch reservation.
"""

from __future__ import annotations

import copy
import hashlib
import json
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable


BUDGET = 5
TENANTS = ("tenant-a", "tenant-b")
REQUESTS = (
    ("req-a1", "tenant-a", 2),
    ("req-a2", "tenant-a", 3),
    ("req-b1", "tenant-b", 2),
    ("req-b2", "tenant-b", 3),
)
EVENTS = ("evt-1", "evt-2")
MAX_DEPTH = 5


@dataclass(frozen=True)
class Tenant:
    budget: int = BUDGET
    reserved: int = 0
    settled: int = 0


@dataclass(frozen=True)
class Record:
    request_id: str
    tenant_id: str
    reserved: int
    status: str = "reserved"
    settled: int = 0
    event_id: str = ""


@dataclass
class State:
    tenants: dict[str, Tenant] = field(default_factory=lambda: {t: Tenant() for t in TENANTS})
    records: dict[str, Record] = field(default_factory=dict)

    def clone(self) -> "State":
        return copy.deepcopy(self)

    def fingerprint(self) -> str:
        payload = {
            "tenants": {k: asdict(v) for k, v in sorted(self.tenants.items())},
            "records": {k: asdict(v) for k, v in sorted(self.records.items())},
        }
        return hashlib.sha256(json.dumps(payload, sort_keys=True).encode()).hexdigest()


def replace_tenant(state: State, tenant_id: str, **kwargs: int) -> None:
    current = asdict(state.tenants[tenant_id])
    current.update(kwargs)
    state.tenants[tenant_id] = Tenant(**current)


def replace_record(state: State, request_id: str, **kwargs: int | str) -> None:
    current = asdict(state.records[request_id])
    current.update(kwargs)
    state.records[request_id] = Record(**current)


def reserve(state: State, request_id: str, tenant_id: str, amount: int) -> bool:
    if request_id in state.records:
        return False
    tenant = state.tenants[tenant_id]
    if tenant.settled + tenant.reserved + amount > tenant.budget:
        return False
    replace_tenant(state, tenant_id, reserved=tenant.reserved + amount)
    state.records[request_id] = Record(request_id=request_id, tenant_id=tenant_id, reserved=amount)
    return True


def settle(state: State, request_id: str, actor_tenant: str, event_id: str, actual: int) -> bool:
    record = state.records.get(request_id)
    if record is None or record.tenant_id != actor_tenant:
        return False
    if record.status == "settled":
        return record.event_id == event_id
    if record.status != "reserved":
        return False
    if actual > record.reserved:
        return False
    tenant = state.tenants[actor_tenant]
    replace_tenant(
        state,
        actor_tenant,
        reserved=tenant.reserved - record.reserved,
        settled=tenant.settled + actual,
    )
    replace_record(state, request_id, status="settled", settled=actual, event_id=event_id)
    return True


def expire(state: State, request_id: str, actor_tenant: str) -> bool:
    record = state.records.get(request_id)
    if record is None or record.tenant_id != actor_tenant or record.status != "reserved":
        return False
    tenant = state.tenants[actor_tenant]
    replace_tenant(state, actor_tenant, reserved=tenant.reserved - record.reserved)
    replace_record(state, request_id, status="expired")
    return True


def inv_non_negative(state: State) -> bool:
    return all(t.budget >= 0 and t.reserved >= 0 and t.settled >= 0 for t in state.tenants.values())


def inv_strict_budget(state: State) -> bool:
    return all(t.settled + t.reserved <= t.budget for t in state.tenants.values())


def inv_record_accounting(state: State) -> bool:
    for tenant_id, tenant in state.tenants.items():
        reserved = sum(r.reserved for r in state.records.values() if r.tenant_id == tenant_id and r.status == "reserved")
        settled = sum(r.settled for r in state.records.values() if r.tenant_id == tenant_id and r.status == "settled")
        if tenant.reserved != reserved or tenant.settled != settled:
            return False
    return True


def inv_terminal_single_effect(state: State) -> bool:
    return all(r.status in {"reserved", "settled", "expired"} and not (r.status == "expired" and r.settled) for r in state.records.values())


def inv_tenant_isolation(state: State) -> bool:
    return all(r.tenant_id in TENANTS and r.request_id.startswith("req-" + r.tenant_id[-1]) for r in state.records.values())


INVARIANTS: dict[str, Callable[[State], bool]] = {
    "non_negative_ledger": inv_non_negative,
    "strict_budget_safety": inv_strict_budget,
    "record_accounting_matches_tenant_views": inv_record_accounting,
    "terminal_events_have_single_effect": inv_terminal_single_effect,
    "tenant_isolation": inv_tenant_isolation,
}


def enabled_actions() -> list[tuple[str, tuple]]:
    actions: list[tuple[str, tuple]] = []
    for request_id, tenant_id, amount in REQUESTS:
        actions.append(("reserve", (request_id, tenant_id, amount)))
        actions.append(("expire", (request_id, tenant_id)))
        other_tenant = "tenant-b" if tenant_id == "tenant-a" else "tenant-a"
        actions.append(("expire_wrong_tenant", (request_id, other_tenant)))
        for event_id in EVENTS:
            for actual in range(0, amount + 1):
                actions.append(("settle", (request_id, tenant_id, event_id, actual)))
                actions.append(("settle_wrong_tenant", (request_id, other_tenant, event_id, actual)))
    return actions


def apply_action(state: State, name: str, args: tuple) -> tuple[State, bool]:
    next_state = state.clone()
    if name == "reserve":
        ok = reserve(next_state, *args)
    elif name in {"settle", "settle_wrong_tenant"}:
        before = state.fingerprint()
        ok = settle(next_state, *args)
        if name == "settle" and ok:
            after_first = next_state.fingerprint()
            duplicate = next_state.clone()
            ok_duplicate = settle(duplicate, *args)
            if not ok_duplicate or duplicate.fingerprint() != after_first:
                raise AssertionError("same settlement event is not idempotent")
        elif next_state.fingerprint() != before:
            raise AssertionError("rejected settlement changed state")
    elif name in {"expire", "expire_wrong_tenant"}:
        before = state.fingerprint()
        ok = expire(next_state, *args)
        if not ok and next_state.fingerprint() != before:
            raise AssertionError("rejected expiry changed state")
    else:
        raise ValueError(name)
    return next_state, ok


def check_state(state: State, trace: list[str]) -> list[dict[str, object]]:
    failures = []
    for name, fn in INVARIANTS.items():
        if not fn(state):
            failures.append({"invariant": name, "trace": trace, "state": state.fingerprint()})
    return failures


def main() -> int:
    failures: list[dict[str, object]] = []
    visited: set[tuple[str, int]] = set()
    states_checked = 0
    transitions_checked = 0
    actions = enabled_actions()

    def dfs(state: State, depth: int, trace: list[str]) -> None:
        nonlocal states_checked, transitions_checked
        key = (state.fingerprint(), depth)
        if key in visited:
            return
        visited.add(key)
        states_checked += 1
        failures.extend(check_state(state, trace))
        if failures or depth == MAX_DEPTH:
            return
        for name, args in actions:
            transitions_checked += 1
            try:
                next_state, ok = apply_action(state, name, args)
            except AssertionError as exc:
                failures.append({"invariant": "transition_atomicity_or_idempotence", "trace": trace + [f"{name}{args}"], "error": str(exc)})
                return
            dfs(next_state, depth + 1, trace + [f"{name}{args} -> {'ok' if ok else 'reject'}"])

    dfs(State(), 0, [])
    result = {
        "schema_version": 1,
        "checked_at_utc": datetime.now(timezone.utc).isoformat(),
        "model": "bounded exhaustive reservation-ledger state exploration",
        "assumptions": [
            "strict-mode authoritative actual cost is <= reserved amount",
            "one transactional ledger serializes each accepted transition",
            "rejected transitions are atomic no-ops",
        ],
        "depth": MAX_DEPTH,
        "states_checked": states_checked,
        "transitions_checked": transitions_checked,
        "invariants_checked": len(INVARIANTS) + 1,
        "invariants": sorted([*INVARIANTS, "settlement_idempotence_and_rejected_transition_noop"]),
        "failures": failures[:20],
        "exit_code": 0 if not failures else 1,
    }
    out = Path(__file__).with_name("results.json")
    out.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(result, indent=2, sort_keys=True))
    return result["exit_code"]


if __name__ == "__main__":
    raise SystemExit(main())
