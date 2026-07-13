#!/usr/bin/env python3
"""Bounded exhaustive model of GOV-AR ledger, outbox, and correction holds.

The model establishes conditional ledger arithmetic only. Provider billing
caps, calibration, availability, and population inference remain assumptions
or empirical obligations.
"""

from __future__ import annotations

import copy
import hashlib
import json
from dataclasses import asdict, dataclass, field, replace
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable


BUDGET = 5
PRINCIPALS = (
    ("tenant-a", "uid-a1"),
    ("tenant-a", "uid-a2"),
    ("tenant-b", "uid-b1"),
)
REQUEST_CATALOG = {
    "req-a1": ("tenant-a", "uid-a1", 2),
    "req-a2": ("tenant-a", "uid-a2", 3),
    "req-b1": ("tenant-b", "uid-b1", 2),
    "req-a-refill": ("tenant-a", "uid-a2", 5),
    "req-a-over": ("tenant-a", "uid-a2", 6),
}
REQUEST_MODEL = {request_id: ("model-a" if tenant == "tenant-a" else "model-b")
                 for request_id, (tenant, _uid, _amount) in REQUEST_CATALOG.items()}
# Exhaustive exploration uses one request per tenant; explicit scenario traces
# below cover same-tenant concurrency, a second semantic event, and correction
# version ordering without multiplying symmetric states.
EXPLORED_REQUESTS = ("req-a1", "req-b1")
BASE_EVENTS = ("evt-1",)
MAX_DEPTH = 6
ACTIVE = {"reserved", "dispatch_pending", "dispatched", "unresolved"}
PROVISIONAL = {"settled_provisional", "late_settled_provisional", "corrected_provisional"}
FINAL = {"finalized", "late_finalized"}
UNBILLED = {"expired_undispatched", "canceled_unbilled", "failed_unbilled"}


HEX_A = "a" * 64
HEX_B = "b" * 64
PATH_MODES = {"openai-body", "azure-deployment-path", "anthropic-body", "google-generate-path"}


@dataclass(frozen=True)
class RouteBinding:
    name: str
    provider_deployment: str
    cluster: str
    authority: str
    path_mode: str


@dataclass(frozen=True)
class AIModelObject:
    name: str
    uid: str
    generation: int
    resource_version: str
    provider_ref: str
    route_binding_ref: str
    # This represents the deprecated model-owned route surface. Snapshot
    # construction never reads it.
    untrusted_route_hint: RouteBinding | None = None


@dataclass(frozen=True)
class AIProviderObject:
    name: str
    uid: str
    generation: int
    resource_version: str
    pricing_version: str
    pricing_compliance_hash: str
    route_bindings: tuple[RouteBinding, ...]


def model_defaults() -> dict[str, AIModelObject]:
    return {
        "model-a": AIModelObject("model-a", "model-a-u1", 1, "m-a-rv1", "provider-a", "primary"),
        "model-b": AIModelObject("model-b", "model-b-u1", 1, "m-b-rv1", "provider-b", "primary"),
    }


def provider_defaults() -> dict[str, AIProviderObject]:
    return {
        "provider-a": AIProviderObject("provider-a", "provider-a-u1", 1, "p-a-rv1",
                                       "pricing-a-v1", HEX_A,
                                       (RouteBinding("primary", "deployment-a", "cluster-a",
                                                     "a.example", "openai-body"),)),
        "provider-b": AIProviderObject("provider-b", "provider-b-u1", 1, "p-b-rv1",
                                       "pricing-b-v1", HEX_B,
                                       (RouteBinding("primary", "deployment-b", "cluster-b",
                                                     "b.example", "openai-body"),
                                        RouteBinding("provider-b-only", "deployment-b2", "cluster-b2",
                                                     "b2.example", "openai-body"))),
    }


@dataclass(frozen=True)
class RouteSnapshot:
    namespace: str
    model_name: str
    model_uid: str
    model_generation: int
    model_resource_version: str
    provider_name: str
    provider_uid: str
    provider_generation: int
    provider_resource_version: str
    pricing_version: str
    pricing_compliance_hash: str
    route_binding_name: str
    provider_deployment: str
    cluster: str
    authority: str
    path_mode: str
    snapshot_hash: str


def fixed_hash(domain: str, values: tuple[object, ...]) -> str:
    payload = [domain, *values]
    return hashlib.sha256(json.dumps(payload, separators=(",", ":")).encode()).hexdigest()


def snapshot_fields(snapshot: RouteSnapshot) -> tuple[object, ...]:
    return (snapshot.namespace, snapshot.model_name, snapshot.model_uid, snapshot.model_generation,
            snapshot.model_resource_version, snapshot.provider_name, snapshot.provider_uid,
            snapshot.provider_generation, snapshot.provider_resource_version,
            snapshot.pricing_version, snapshot.pricing_compliance_hash,
            snapshot.route_binding_name, snapshot.provider_deployment, snapshot.cluster,
            snapshot.authority, snapshot.path_mode)


def rehash_snapshot(snapshot: RouteSnapshot) -> RouteSnapshot:
    return replace(snapshot, snapshot_hash=fixed_hash("govar-route-snapshot-v1",
                                                     snapshot_fields(snapshot)))


def provider_binding(provider: AIProviderObject, name: str) -> RouteBinding | None:
    matches = [binding for binding in provider.route_bindings if binding.name == name]
    return matches[0] if len(matches) == 1 else None


def snapshot_from_catalog(models: dict[str, AIModelObject], providers: dict[str, AIProviderObject],
                          model_name: str) -> tuple[RouteSnapshot, AIModelObject, AIProviderObject] | None:
    model = models.get(model_name)
    if model is None or not model.provider_ref or not model.route_binding_ref:
        return None
    provider = providers.get(model.provider_ref)
    if provider is None:
        return None
    binding = provider_binding(provider, model.route_binding_ref)
    if binding is None:
        return None
    fields = ("finance", model.name, model.uid, model.generation, model.resource_version,
              provider.name, provider.uid, provider.generation, provider.resource_version,
              provider.pricing_version, provider.pricing_compliance_hash, binding.name,
              binding.provider_deployment, binding.cluster, binding.authority, binding.path_mode)
    snapshot = RouteSnapshot(*fields, fixed_hash("govar-route-snapshot-v1", fields))
    return snapshot, model, provider


def valid_snapshot(snapshot: RouteSnapshot) -> bool:
    fields = snapshot_fields(snapshot)
    required_strings = (snapshot.namespace, snapshot.model_name, snapshot.model_uid,
                        snapshot.model_resource_version, snapshot.provider_name,
                        snapshot.provider_uid, snapshot.provider_resource_version,
                        snapshot.pricing_version, snapshot.route_binding_name,
                        snapshot.provider_deployment, snapshot.cluster, snapshot.authority)
    is_hex = lambda value: len(value) == 64 and all(c in "0123456789abcdef" for c in value)
    return (all(required_strings) and snapshot.model_generation > 0 and snapshot.provider_generation > 0
            and snapshot.path_mode in PATH_MODES and is_hex(snapshot.pricing_compliance_hash)
            and is_hex(snapshot.snapshot_hash)
            and snapshot.snapshot_hash == fixed_hash("govar-route-snapshot-v1", fields))


def validate_snapshot_abstraction() -> None:
    resolved = snapshot_from_catalog(model_defaults(), provider_defaults(), "model-a")
    assert resolved is not None
    valid = resolved[0]
    assert valid_snapshot(valid)
    malformed = [
        replace(valid, model_uid=""), replace(valid, model_resource_version=""),
        replace(valid, provider_uid=""), replace(valid, provider_resource_version=""),
        replace(valid, route_binding_name=""), replace(valid, provider_deployment=""),
        replace(valid, cluster=""), replace(valid, authority=""),
        replace(valid, path_mode="unknown-adapter"),
        replace(valid, pricing_compliance_hash="a" * 63),
        replace(valid, pricing_compliance_hash="g" * 64),
    ]
    for value in malformed:
        assert not valid_snapshot(rehash_snapshot(value))
    assert not valid_snapshot(replace(valid, snapshot_hash="f" * 64))


def snapshot_route(snapshot: RouteSnapshot) -> tuple[str, str, str, str]:
    return (snapshot.provider_deployment, snapshot.cluster, snapshot.authority, snapshot.path_mode)


def response_bytes(record: "Record") -> str:
    payload = {"selected_deployment": record.route_snapshot.model_name,
               "pricing_version": record.route_snapshot.pricing_version,
               "route_snapshot": asdict(record.route_snapshot)}
    return json.dumps(payload, separators=(",", ":"), sort_keys=False)


@dataclass(frozen=True)
class Tenant:
    budget: int = BUDGET
    current_window: int = 0
    settled: int = 0
    reserved: int = 0
    carried_debt: int = 0
    historical_credit: int = 0  # audit-only; never increases a later window's availability


@dataclass(frozen=True)
class Record:
    request_id: str
    tenant_id: str
    workload_uid: str
    reserved_ceiling: int
    residual_hold: int
    origin_window: int
    route_snapshot: RouteSnapshot
    reserved_model: AIModelObject
    reserved_provider: AIProviderObject
    persisted_response_bytes: str
    replay_response_bytes: str = ""
    dispatch_target: tuple[str, str, str, str] = ()
    status: str = "reserved"
    outbox: str = "pending"
    actual: int = 0
    base_actual: int = 0
    settled_effect: int = 0
    enforcement_window: int = -1
    carried: bool = False
    carry_effect: int = 0
    credit_effect: int = 0
    rollover_guard: int = 0  # temporary prior-window provisional actual held until finality
    base_event: str = ""
    correction_version: int = 0
    correction_events: tuple[str, ...] = ()
    postfinal_correction_effects: int = 0
    settlement_effects: int = 0
    finalization_effects: int = 0
    unbilled_release_effects: int = 0


@dataclass
class State:
    tenants: dict[str, Tenant] = field(default_factory=lambda: {"tenant-a": Tenant(), "tenant-b": Tenant()})
    records: dict[str, Record] = field(default_factory=dict)
    models: dict[str, AIModelObject] = field(default_factory=model_defaults)
    providers: dict[str, AIProviderObject] = field(default_factory=provider_defaults)
    catalog_variant: str = "base"
    inbox: dict[str, str] = field(default_factory=dict)  # event ID -> immutable payload identity
    transition_coverage: set[str] = field(default_factory=set)

    def clone(self) -> "State":
        return copy.deepcopy(self)

    def fingerprint(self) -> str:
        # All leaves are frozen dataclasses, integers, strings, and tuples.
        # Their ordered repr is deterministic and materially faster than a
        # recursive asdict/JSON conversion at every explored transition.
        payload = (tuple(sorted(self.tenants.items())), tuple(sorted(self.records.items())),
                   tuple(sorted(self.models.items())), tuple(sorted(self.providers.items())),
                   self.catalog_variant, tuple(sorted(self.inbox.items())))
        return hashlib.sha256(repr(payload).encode()).hexdigest()


def replace_tenant(state: State, tenant_id: str, **kwargs: int) -> None:
    values = asdict(state.tenants[tenant_id])
    values.update(kwargs)
    state.tenants[tenant_id] = Tenant(**values)


def replace_record(state: State, request_id: str, **kwargs: object) -> None:
    state.records[request_id] = replace(state.records[request_id], **kwargs)


def authorized(record: Record, tenant_id: str, workload_uid: str) -> bool:
    return (record.tenant_id, record.workload_uid) == (tenant_id, workload_uid)


def reserve(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    expected = REQUEST_CATALOG.get(request_id)
    if expected is None or expected[:2] != (tenant_id, workload_uid) or request_id in state.records:
        return False
    amount = expected[2]
    resolved = snapshot_from_catalog(state.models, state.providers, REQUEST_MODEL[request_id])
    if resolved is None:
        return False
    snapshot, model, provider = resolved
    if not valid_snapshot(snapshot):
        return False
    tenant = state.tenants[tenant_id]
    if tenant.settled + tenant.reserved + tenant.carried_debt + amount > tenant.budget:
        return False
    replace_tenant(state, tenant_id, reserved=tenant.reserved + amount)
    record = Record(request_id, tenant_id, workload_uid, amount, amount,
                    tenant.current_window, snapshot, model, provider, "")
    state.records[request_id] = replace(record, persisted_response_bytes=response_bytes(record))
    state.transition_coverage.add("atomic_reserve_and_pending_outbox")
    return True


def claim_outbox(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    if record.status != "reserved" or record.outbox != "pending":
        return False
    replace_record(state, request_id, status="dispatch_pending", outbox="claimed",
                   dispatch_target=snapshot_route(record.route_snapshot))
    state.transition_coverage.add("outbox_claim")
    return True


def mark_dispatched(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    if record.status != "dispatch_pending" or record.outbox != "claimed":
        return False
    replace_record(state, request_id, status="dispatched", outbox="delivered",
                   dispatch_target=snapshot_route(record.route_snapshot))
    state.transition_coverage.add("provider_delivery")
    return True


def catalog_action_allowed(state: State, variant: str) -> bool:
    if state.catalog_variant != "base":
        return False
    # Symmetry reduction: mutating model-a while the state contains only
    # model-b records cannot affect a route snapshot or dispatch target. Keep
    # the empty-state mutation (mutation before reserve) and every state that
    # already contains a model-a reservation (mutation at each later boundary).
    if variant in {"cross-provider-binding", "model-owned-route-only"}:
        if state.records:
            return False
    elif not any(REQUEST_MODEL[r.request_id] == "model-a" for r in state.records.values()):
        return False
    state.catalog_variant = variant
    return True


def recreate_model(state: State) -> bool:
    if not catalog_action_allowed(state, "model-recreated"):
        return False
    model = state.models["model-a"]
    state.models["model-a"] = replace(model, uid="model-a-u2", generation=2,
                                      resource_version="m-a-rv2")
    state.transition_coverage.add("model_same_name_delete_recreate")
    return True


def recreate_provider_and_route(state: State) -> bool:
    if not catalog_action_allowed(state, "provider-recreated"):
        return False
    provider = state.providers["provider-a"]
    state.providers["provider-a"] = replace(
        provider, uid="provider-a-u2", generation=2, resource_version="p-a-rv2",
        pricing_version="pricing-a-v2", pricing_compliance_hash="c" * 64,
        route_bindings=(RouteBinding("primary", "deployment-a2", "cluster-a2",
                                     "a2.example", "openai-body"),))
    state.transition_coverage.add("provider_route_price_same_name_delete_recreate")
    return True


def inject_cross_provider_binding(state: State) -> bool:
    if not catalog_action_allowed(state, "cross-provider-binding"):
        return False
    model = state.models["model-a"]
    # provider-b-only exists under provider-b, but model-a still references
    # provider-a. Resolution must not search globally by binding name.
    state.models["model-a"] = replace(model, route_binding_ref="provider-b-only")
    state.transition_coverage.add("cross_provider_binding_injection")
    return True


def inject_model_owned_route_only(state: State) -> bool:
    if not catalog_action_allowed(state, "model-owned-route-only"):
        return False
    model = state.models["model-a"]
    state.models["model-a"] = replace(
        model, route_binding_ref="",
        untrusted_route_hint=RouteBinding("attacker", "deployment-b", "cluster-b",
                                          "b.example", "openai-body"))
    state.transition_coverage.add("model_owned_route_injection")
    return True


def duplicate_admit(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    replay = response_bytes(record)
    if replay != record.persisted_response_bytes:
        return False
    if record.replay_response_bytes and record.replay_response_bytes != replay:
        return False
    replace_record(state, request_id, replay_response_bytes=replay)
    state.transition_coverage.add("duplicate_admit_byte_stable_route_response")
    return True


def conflicting_duplicate_admit(state: State, request_id: str, tenant_id: str,
                                 workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    resolved = snapshot_from_catalog(state.models, state.providers, REQUEST_MODEL[request_id])
    # This action represents a duplicate Admit carrying the live, conflicting
    # catalog snapshot rather than the persisted admission fingerprint. It must
    # be rejected atomically; it never replaces the record or response.
    if resolved is None or resolved[0] == record.route_snapshot:
        return False
    # The live proposal conflicts with the persisted snapshot. Rejection is an
    # atomic no-op by construction and is asserted by the required scenario.
    return False


def timeout_or_expire(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    if record.status == "reserved" and record.outbox == "pending":
        # One atomic compare/cancel/release transition. A later claim sees canceled.
        tenant = state.tenants[tenant_id]
        replace_tenant(state, tenant_id, reserved=tenant.reserved - record.residual_hold)
        replace_record(state, request_id, status="expired_undispatched", outbox="canceled",
                       residual_hold=0, unbilled_release_effects=1)
        state.transition_coverage.add("pending_outbox_cancel_and_release")
        return True
    if record.status in {"dispatch_pending", "dispatched"}:
        replace_record(state, request_id, status="unresolved")
        state.transition_coverage.add("ambiguous_timeout_retains_hold")
        return True
    return record.status == "unresolved"


def prove_unbilled(state: State, request_id: str, tenant_id: str, workload_uid: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    if record.status not in {"reserved", "dispatch_pending"} or record.outbox not in {"pending", "claimed"}:
        return False
    tenant = state.tenants[tenant_id]
    replace_tenant(state, tenant_id, reserved=tenant.reserved - record.residual_hold)
    status = "canceled_unbilled" if record.status == "reserved" else "failed_unbilled"
    replace_record(state, request_id, status=status, outbox="canceled", residual_hold=0,
                   unbilled_release_effects=1)
    state.transition_coverage.add("authoritative_unbilled_cancel_and_release")
    return True


def settle(state: State, request_id: str, tenant_id: str, workload_uid: str,
           event_id: str, actual: int) -> bool:
    record = state.records.get(request_id)
    if (record is None or not authorized(record, tenant_id, workload_uid)
            or actual < 0 or actual > record.reserved_ceiling):
        return False
    payload = f"settle|{request_id}|{actual}"
    prior_payload = state.inbox.get(event_id)
    if prior_payload is not None:
        return prior_payload == payload
    if record.status in (PROVISIONAL | FINAL):
        if record.actual != actual:
            return False
        state.inbox[event_id] = payload  # differently keyed semantic duplicate, no posting
        state.transition_coverage.add("semantic_duplicate_no_effect")
        return True
    if record.status not in {"dispatch_pending", "dispatched", "unresolved"}:
        return False
    tenant = state.tenants[tenant_id]
    late = record.status == "unresolved" or tenant.current_window != record.origin_window
    residual = record.reserved_ceiling - actual
    replace_tenant(
        state,
        tenant_id,
        reserved=tenant.reserved - actual,
        settled=tenant.settled + (0 if late else actual),
        carried_debt=tenant.carried_debt + (actual if late else 0),
    )
    replace_record(
        state,
        request_id,
        status="late_settled_provisional" if late else "settled_provisional",
        outbox="delivered",  # authoritative usage proves a billable delivery
        residual_hold=residual,
        actual=actual,
        base_actual=actual,
        settled_effect=0 if late else actual,
        enforcement_window=tenant.current_window,
        carried=late,
        carry_effect=actual if late else 0,
        base_event=event_id,
        settlement_effects=1,
    )
    state.inbox[event_id] = payload
    state.transition_coverage.add("reordered_usage_proves_delivery" if record.status == "dispatch_pending" else
                                  ("late_settlement" if late else "settlement_with_residual_hold"))
    return True


def correct(state: State, request_id: str, tenant_id: str, workload_uid: str,
            correction_id: str, version: int, new_actual: int) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    payload = f"correct|{request_id}|{version}|{new_actual}"
    prior_payload = state.inbox.get(correction_id)
    if prior_payload is not None:
        if prior_payload == payload:
            state.transition_coverage.add("exact_correction_replay_no_effect")
            return True
        return False
    if (record.status not in (PROVISIONAL | FINAL) or new_actual < 0
            or new_actual > record.reserved_ceiling or version != record.correction_version + 1):
        return False
    tenant = state.tenants[tenant_id]
    delta = new_actual - record.actual
    if record.status in FINAL:
        # Finality releases the residual hold. A provider-side correction can
        # therefore no longer be funded retroactively from that hold. Preserve
        # the original settled charge and expose every upward difference as
        # carried debt; for an already-late request the whole authoritative
        # amount remains external to the active window. Downward corrections
        # are audit-only credits and never mint availability.
        new_carry = record.carry_effect
        if delta > 0 and new_actual > record.base_actual:
            target_debt = new_actual if record.carried else new_actual - record.base_actual
            replace_tenant(state, tenant_id,
                           carried_debt=tenant.carried_debt + target_debt - record.carry_effect)
            new_carry = target_debt
            tenant = state.tenants[tenant_id]
        desired_credit = max(record.base_actual - new_actual, 0)
        replace_tenant(state, tenant_id,
                       historical_credit=tenant.historical_credit + desired_credit - record.credit_effect)
        replace_record(state, request_id, actual=new_actual, carry_effect=new_carry,
                       credit_effect=desired_credit, correction_version=version,
                       correction_events=record.correction_events + (correction_id,),
                       postfinal_correction_effects=record.postfinal_correction_effects + 1)
        state.inbox[correction_id] = payload
        state.transition_coverage.add("postfinal_correction_preserves_external_debt")
        return True
    new_residual = record.residual_hold - delta
    if not 0 <= new_residual <= record.reserved_ceiling:
        return False
    new_carry = record.carry_effect
    new_credit = record.credit_effect
    new_guard = record.rollover_guard
    settled = tenant.settled
    carried_debt = tenant.carried_debt
    historical_credit = tenant.historical_credit
    reserved = tenant.reserved
    settled_effect = record.settled_effect
    if record.carried:
        new_carry += delta
        carried_debt += delta
        reserved -= delta
    elif record.enforcement_window != tenant.current_window:
        # The rollover guard and residual move in opposite directions, keeping
        # current-window exposure at the original R until finality.
        new_guard += delta
        desired_credit = max(record.base_actual - new_actual, 0)
        historical_credit += desired_credit - new_credit
        new_credit = desired_credit
    else:
        settled += delta
        settled_effect += delta
        reserved -= delta
    replace_tenant(state, tenant_id, reserved=reserved,
                   settled=settled, carried_debt=carried_debt,
                   historical_credit=historical_credit)
    replace_record(state, request_id, status="corrected_provisional", residual_hold=new_residual,
                   actual=new_actual, carry_effect=new_carry, credit_effect=new_credit,
                   rollover_guard=new_guard, settled_effect=settled_effect,
                   correction_version=version,
                   correction_events=record.correction_events + (correction_id,))
    state.inbox[correction_id] = payload
    state.transition_coverage.add("monotone_correction_with_residual_hold")
    return True


def finalize(state: State, request_id: str, tenant_id: str, workload_uid: str, event_id: str) -> bool:
    record = state.records.get(request_id)
    if record is None or not authorized(record, tenant_id, workload_uid):
        return False
    payload = f"finalize|{request_id}"
    prior_payload = state.inbox.get(event_id)
    if prior_payload is not None:
        return prior_payload == payload
    if record.status not in PROVISIONAL:
        return False
    tenant = state.tenants[tenant_id]
    replace_tenant(state, tenant_id,
                   reserved=tenant.reserved - record.residual_hold - record.rollover_guard)
    replace_record(state, request_id, status="late_finalized" if record.carried else "finalized",
                   residual_hold=0, rollover_guard=0, finalization_effects=1)
    state.inbox[event_id] = payload
    state.transition_coverage.add("authoritative_finality_releases_residual")
    return True


def rollover(state: State, target_tenant: str, actor_tenant: str, workload_uid: str) -> bool:
    if target_tenant != actor_tenant or (actor_tenant, workload_uid) not in PRINCIPALS:
        return False
    tenant = state.tenants[target_tenant]
    added_guard = 0
    for request_id, record in list(state.records.items()):
        if (record.tenant_id == target_tenant and record.status in PROVISIONAL
                and not record.carried and record.enforcement_window == tenant.current_window
                and record.rollover_guard == 0):
            replace_record(state, request_id, rollover_guard=record.actual)
            added_guard += record.actual
    replace_tenant(state, target_tenant, current_window=tenant.current_window + 1,
                   settled=0, reserved=tenant.reserved + added_guard)
    state.transition_coverage.add("rollover_preserves_holds_and_adjustments")
    return True


def inv_non_negative(state: State) -> bool:
    if not all(min(t.budget, t.settled, t.reserved, t.carried_debt, t.historical_credit) >= 0
               for t in state.tenants.values()):
        return False
    return all(min(r.reserved_ceiling, r.residual_hold, r.actual, r.settled_effect, r.carry_effect,
                   r.credit_effect, r.rollover_guard) >= 0
               for r in state.records.values())


def inv_conditional_strict_feasibility(state: State) -> bool:
    # A correction after authoritative finality is explicitly outside the
    # instantaneous strict guarantee: its hold has already been released and
    # another request may have consumed it. The model still requires the full
    # external debt to remain visible (checked separately). Before such an
    # exogenous correction, strict feasibility must hold for every state.
    for tenant_id, tenant in state.tenants.items():
        postfinal = any(r.tenant_id == tenant_id and r.postfinal_correction_effects > 0
                        for r in state.records.values())
        if not postfinal and tenant.settled + tenant.reserved + tenant.carried_debt > tenant.budget:
            return False
    return True


def inv_record_accounting(state: State) -> bool:
    for tenant_id, tenant in state.tenants.items():
        records = [r for r in state.records.values() if r.tenant_id == tenant_id]
        reserved = sum(r.residual_hold + r.rollover_guard
                       for r in records if r.status in (ACTIVE | PROVISIONAL))
        settled = sum(r.settled_effect for r in records if r.status in (PROVISIONAL | FINAL)
                      and not r.carried and r.enforcement_window == tenant.current_window)
        carry = sum(r.carry_effect for r in records if r.status in (PROVISIONAL | FINAL))
        credit = sum(r.credit_effect for r in records if r.status in (PROVISIONAL | FINAL))
        if (tenant.reserved, tenant.settled, tenant.carried_debt, tenant.historical_credit) != (
                reserved, settled, carry, credit):
            return False
    return True


def inv_outbox_relation(state: State) -> bool:
    for r in state.records.values():
        if r.status == "reserved" and r.outbox != "pending":
            return False
        if r.status == "dispatch_pending" and r.outbox != "claimed":
            return False
        if r.status == "dispatched" and r.outbox != "delivered":
            return False
        if r.status == "unresolved" and r.outbox not in {"claimed", "delivered"}:
            return False
        if r.status in (PROVISIONAL | FINAL) and r.outbox != "delivered":
            return False
        if r.status in UNBILLED and r.outbox != "canceled":
            return False
    return True


def inv_effect_counts(state: State) -> bool:
    for r in state.records.values():
        if r.status in ACTIVE and any((r.settlement_effects, r.finalization_effects, r.unbilled_release_effects)):
            return False
        if r.status in PROVISIONAL and (r.settlement_effects, r.finalization_effects, r.unbilled_release_effects) != (1, 0, 0):
            return False
        if r.status in FINAL and (r.settlement_effects, r.finalization_effects, r.unbilled_release_effects) != (1, 1, 0):
            return False
        if r.status in UNBILLED and (r.settlement_effects, r.finalization_effects, r.unbilled_release_effects) != (0, 0, 1):
            return False
        if r.correction_version != len(r.correction_events) or len(set(r.correction_events)) != len(r.correction_events):
            return False
        if r.postfinal_correction_effects < 0 or r.postfinal_correction_effects > r.correction_version:
            return False
    return True


def inv_identity_isolation(state: State) -> bool:
    return all(REQUEST_CATALOG[r.request_id][:2] == (r.tenant_id, r.workload_uid)
               for r in state.records.values())


def inv_residual_correction_exposure(state: State) -> bool:
    return all(r.residual_hold == r.reserved_ceiling - r.actual
               and (r.rollover_guard == 0 or r.residual_hold + r.rollover_guard == r.reserved_ceiling)
               for r in state.records.values() if r.status in PROVISIONAL)


def inv_postfinal_correction_visible(state: State) -> bool:
    for record in state.records.values():
        if record.postfinal_correction_effects == 0:
            continue
        if record.status not in FINAL or record.residual_hold != 0:
            return False
        expected_carry = record.base_actual if record.carried else 0
        if record.actual > record.base_actual:
            expected_carry = record.actual if record.carried else record.actual - record.base_actual
        # Downward corrections never erase already posted carried debt.
        if record.carry_effect < expected_carry:
            return False
        if record.credit_effect != max(record.base_actual - record.actual, 0):
            return False
    return True


def inv_provisional_rollover_guard(state: State) -> bool:
    for record in state.records.values():
        if record.status not in PROVISIONAL or record.carried:
            continue
        current_window = state.tenants[record.tenant_id].current_window
        if record.enforcement_window != current_window:
            if record.residual_hold + record.rollover_guard != record.reserved_ceiling:
                return False
        elif record.rollover_guard != 0:
            return False
    return True


def inv_provider_route_ownership(state: State) -> bool:
    for record in state.records.values():
        snapshot, model, provider = record.route_snapshot, record.reserved_model, record.reserved_provider
        binding = provider_binding(provider, model.route_binding_ref)
        if (binding is None or model.provider_ref != provider.name
                or snapshot.model_name != model.name or snapshot.model_uid != model.uid
                or snapshot.model_generation != model.generation
                or snapshot.model_resource_version != model.resource_version
                or snapshot.provider_name != provider.name or snapshot.provider_uid != provider.uid
                or snapshot.provider_generation != provider.generation
                or snapshot.provider_resource_version != provider.resource_version
                or snapshot.pricing_version != provider.pricing_version
                or snapshot.pricing_compliance_hash != provider.pricing_compliance_hash
                or snapshot.route_binding_name != binding.name
                or snapshot_route(snapshot) != (binding.provider_deployment, binding.cluster,
                                                binding.authority, binding.path_mode)):
            return False
    return True


def inv_route_snapshot_integrity(state: State) -> bool:
    return all(valid_snapshot(record.route_snapshot) for record in state.records.values())


def inv_dispatch_uses_reserved_snapshot(state: State) -> bool:
    return all((r.dispatch_target == snapshot_route(r.route_snapshot)
                if r.outbox in {"claimed", "delivered"} else
                (not r.dispatch_target or r.dispatch_target == snapshot_route(r.route_snapshot)))
               for r in state.records.values())


def inv_replay_stable_route(state: State) -> bool:
    return all(r.persisted_response_bytes == response_bytes(r)
               and (not r.replay_response_bytes or r.replay_response_bytes == r.persisted_response_bytes)
               for r in state.records.values())


INVARIANTS: dict[str, Callable[[State], bool]] = {
    "non_negative_integer_ledger": inv_non_negative,
    "conditional_strict_ledger_feasibility": inv_conditional_strict_feasibility,
    "record_aggregate_equality": inv_record_accounting,
    "outbox_claim_cancel_delivery_relation": inv_outbox_relation,
    "one_effective_settlement_finalization_and_correction": inv_effect_counts,
    "tenant_and_workload_uid_isolation": inv_identity_isolation,
    "residual_correction_exposure_retained": inv_residual_correction_exposure,
    "postfinal_correction_external_debt_visible": inv_postfinal_correction_visible,
    "provisional_rollover_guard_prevents_credit_reuse": inv_provisional_rollover_guard,
    "provider_route_ownership": inv_provider_route_ownership,
    "route_snapshot_integrity": inv_route_snapshot_integrity,
    "dispatch_uses_reserved_snapshot": inv_dispatch_uses_reserved_snapshot,
    "duplicate_admit_byte_stable_route_response": inv_replay_stable_route,
}


def check_invariants(state: State) -> None:
    failed = [name for name, fn in INVARIANTS.items() if not fn(state)]
    if failed:
        raise AssertionError(",".join(failed))


def enabled_actions() -> list[tuple[str, tuple]]:
    """Return a symmetry-reduced but transition-complete action alphabet.

    Principal mismatches are identity checks, so exhaustively repeating every
    mismatch for every request, event identifier, amount, and version adds no
    state behavior.  Explore all value-bearing transitions for each correct
    principal, then one representative wrong-tenant and wrong-workload attempt
    for every mutating API shape.
    """
    actions: list[tuple[str, tuple]] = []
    for request_id in EXPLORED_REQUESTS:
        tenant, uid, amount = REQUEST_CATALOG[request_id]
        actions.extend([
            ("reserve", (request_id, tenant, uid)),
            ("claim", (request_id, tenant, uid)),
            ("dispatch", (request_id, tenant, uid)),
            ("timeout", (request_id, tenant, uid)),
            ("prove_unbilled", (request_id, tenant, uid)),
            ("finalize", (request_id, tenant, uid, f"fin-{request_id}")),
        ])
        for event in BASE_EVENTS:
            for actual in sorted({0, amount // 2, amount}):
                actions.append(("settle", (request_id, tenant, uid, event, actual)))
        for version in (1,):
            for actual in sorted({0, amount // 2, amount}):
                actions.append(("correct", (request_id, tenant, uid,
                                            f"corr-{request_id}-{version}", version, actual)))

    request_id = "req-a1"
    tenant, uid, amount = REQUEST_CATALOG[request_id]
    actions.append(("duplicate_admit", (request_id, tenant, uid)))
    actions.append(("conflicting_duplicate_admit", (request_id, tenant, uid)))
    for actor_tenant, actor_uid in (("tenant-b", uid), (tenant, "uid-a2")):
        actions.extend([
            ("reserve", (request_id, actor_tenant, actor_uid)),
            ("claim", (request_id, actor_tenant, actor_uid)),
            ("dispatch", (request_id, actor_tenant, actor_uid)),
            ("timeout", (request_id, actor_tenant, actor_uid)),
            ("prove_unbilled", (request_id, actor_tenant, actor_uid)),
            ("settle", (request_id, actor_tenant, actor_uid, "evt-wrong", amount)),
            ("correct", (request_id, actor_tenant, actor_uid, "corr-wrong", 1, amount)),
            ("finalize", (request_id, actor_tenant, actor_uid, "fin-wrong")),
        ])
    for target in state_tenants():
        for actor_tenant, actor_uid in PRINCIPALS:
            actions.append(("rollover", (target, actor_tenant, actor_uid)))
    actions.extend([
        ("model_recreate", ()),
        ("provider_recreate", ()),
        ("cross_provider_binding", ()),
        ("model_owned_route_only", ()),
    ])
    return actions


def state_tenants() -> tuple[str, ...]:
    return ("tenant-a", "tenant-b")


def apply_action(state: State, name: str, args: tuple) -> tuple[State, bool]:
    next_state = state.clone()
    before = state.fingerprint()
    functions = {
        "reserve": reserve,
        "claim": claim_outbox,
        "dispatch": mark_dispatched,
        "timeout": timeout_or_expire,
        "prove_unbilled": prove_unbilled,
        "settle": settle,
        "correct": correct,
        "finalize": finalize,
        "rollover": rollover,
        "duplicate_admit": duplicate_admit,
        "conflicting_duplicate_admit": conflicting_duplicate_admit,
        "model_recreate": recreate_model,
        "provider_recreate": recreate_provider_and_route,
        "cross_provider_binding": inject_cross_provider_binding,
        "model_owned_route_only": inject_model_owned_route_only,
    }
    ok = functions[name](next_state, *args)
    if not ok and next_state.fingerprint() != before:
        raise AssertionError(f"rejected {name} changed state")
    return next_state, ok


def required_scenarios() -> dict[str, list[tuple[str, tuple]]]:
    a = ("tenant-a", "uid-a1")
    b = ("tenant-b", "uid-b1")
    return {
        "unresolved_rollover_late_correction_finality": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("dispatch", ("req-a1", *a)), ("timeout", ("req-a1", *a)),
            ("rollover", ("tenant-a", *a)),
            ("settle", ("req-a1", *a, "evt-late", 1)),
            ("correct", ("req-a1", *a, "corr-late-1", 1, 2)),
            ("finalize", ("req-a1", *a, "fin-late")),
        ],
        "late_final_upward_correction_preserves_full_external_debt": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("dispatch", ("req-a1", *a)), ("timeout", ("req-a1", *a)),
            ("rollover", ("tenant-a", *a)),
            ("settle", ("req-a1", *a, "evt-late-final", 1)),
            ("finalize", ("req-a1", *a, "fin-late-final")),
            ("correct", ("req-a1", *a, "corr-after-final", 1, 2)),
        ],
        "atomic_pending_expiry_blocks_late_claim": [
            ("reserve", ("req-a1", *a)), ("timeout", ("req-a1", *a)),
            ("claim", ("req-a1", *a)),
        ],
        "reordered_usage_proves_delivery": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("settle", ("req-a1", *a, "evt-reordered", 1)),
            ("dispatch", ("req-a1", *a)),
        ],
        "residual_hold_blocks_correction_refill_race": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("dispatch", ("req-a1", *a)), ("settle", ("req-a1", *a, "evt-zero", 0)),
            ("reserve", ("req-a-refill", "tenant-a", "uid-a2")),
            ("correct", ("req-a1", *a, "corr-up", 1, 2)),
            ("finalize", ("req-a1", *a, "fin-up")),
        ],
        "semantic_duplicate_no_second_effect": [
            ("reserve", ("req-b1", *b)), ("claim", ("req-b1", *b)),
            ("dispatch", ("req-b1", *b)), ("settle", ("req-b1", *b, "evt-d1", 1)),
            ("settle", ("req-b1", *b, "evt-d2", 1)),
        ],
        "same_tenant_concurrent_reservations_isolated_by_workload_uid": [
            ("reserve", ("req-a1", "tenant-a", "uid-a1")),
            ("reserve", ("req-a2", "tenant-a", "uid-a2")),
            ("claim", ("req-a1", "tenant-a", "uid-a1")),
            ("claim", ("req-a2", "tenant-a", "uid-a2")),
            ("settle", ("req-a1", "tenant-a", "uid-a1", "evt-w1", 1)),
            ("settle", ("req-a2", "tenant-a", "uid-a2", "evt-w2", 2)),
        ],
        "historical_credit_cannot_expand_future_windows": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("settle", ("req-a1", *a, "evt-credit", 1)),
            ("rollover", ("tenant-a", *a)),
            ("correct", ("req-a1", *a, "corr-credit", 1, 0)),
            ("finalize", ("req-a1", *a, "fin-credit")),
            ("reserve", ("req-a-over", "tenant-a", "uid-a2")),
            ("rollover", ("tenant-a", *a)),
            ("reserve", ("req-a-over", "tenant-a", "uid-a2")),
        ],
        "correction_exact_replay_after_later_version": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("settle", ("req-a1", *a, "evt-replay", 0)),
            ("correct", ("req-a1", *a, "corr-replay-1", 1, 1)),
            ("correct", ("req-a1", *a, "corr-replay-2", 2, 2)),
            ("correct", ("req-a1", *a, "corr-replay-1", 1, 1)),
        ],
        "stale_correction_version_rejected": [
            ("reserve", ("req-a1", *a)), ("claim", ("req-a1", *a)),
            ("dispatch", ("req-a1", *a)), ("settle", ("req-a1", *a, "evt-c", 1)),
            ("correct", ("req-a1", *a, "corr-v2", 2, 2)),
            ("correct", ("req-a1", *a, "corr-v1", 1, 2)),
        ],
        "provider_mutation_then_duplicate_admit_and_dispatch_use_persisted_snapshot": [
            ("reserve", ("req-a1", *a)),
            ("provider_recreate", ()),
            ("duplicate_admit", ("req-a1", *a)),
            ("claim", ("req-a1", *a)),
            ("dispatch", ("req-a1", *a)),
        ],
        "model_same_name_recreate_then_replay_is_byte_stable": [
            ("reserve", ("req-a1", *a)),
            ("model_recreate", ()),
            ("duplicate_admit", ("req-a1", *a)),
        ],
        "conflicting_duplicate_after_provider_mutation_is_rejected": [
            ("reserve", ("req-a1", *a)),
            ("provider_recreate", ()),
            ("conflicting_duplicate_admit", ("req-a1", *a)),
        ],
        "cross_provider_binding_name_is_rejected": [
            ("cross_provider_binding", ()),
            ("reserve", ("req-a1", *a)),
        ],
        "model_owned_route_without_provider_binding_is_rejected": [
            ("model_owned_route_only", ()),
            ("reserve", ("req-a1", *a)),
        ],
    }


def run_required_scenarios() -> tuple[list[str], list[dict[str, object]], set[str]]:
    covered: list[str] = []
    failures: list[dict[str, object]] = []
    transition_coverage: set[str] = set()
    expected_rejections = {
        ("atomic_pending_expiry_blocks_late_claim", 2),
        ("reordered_usage_proves_delivery", 3),
        ("residual_hold_blocks_correction_refill_race", 4),
        ("stale_correction_version_rejected", 4),
        ("historical_credit_cannot_expand_future_windows", 6),
        ("historical_credit_cannot_expand_future_windows", 8),
        ("cross_provider_binding_name_is_rejected", 1),
        ("model_owned_route_without_provider_binding_is_rejected", 1),
        ("conflicting_duplicate_after_provider_mutation_is_rejected", 2),
    }
    for scenario, actions in required_scenarios().items():
        state = State()
        try:
            for index, (name, args) in enumerate(actions):
                next_state, ok = apply_action(state, name, args)
                if (scenario, index) in expected_rejections:
                    assert not ok, f"expected rejection at {scenario}:{index}"
                else:
                    assert ok, f"expected success at {scenario}:{index}"
                state = next_state
                check_invariants(state)
                transition_coverage.update(state.transition_coverage)
            covered.append(scenario)
        except AssertionError as exc:
            failures.append({"scenario": scenario, "error": str(exc)})
    return covered, failures, transition_coverage


def main() -> int:
    failures: list[dict[str, object]] = []
    try:
        validate_snapshot_abstraction()
    except AssertionError as exc:
        failures.append({"invariant": "route_snapshot_validation_abstraction", "error": str(exc)})
    visited: set[tuple[str, int]] = set()
    states_checked = 0
    transitions_checked = 0
    coverage: set[str] = set()
    actions = enabled_actions()

    def dfs(state: State, depth: int, trace: list[str]) -> None:
        nonlocal states_checked, transitions_checked
        state_fp = state.fingerprint()
        key = (state_fp, depth)
        if key in visited or failures:
            return
        visited.add(key)
        states_checked += 1
        coverage.update(state.transition_coverage)
        try:
            check_invariants(state)
        except AssertionError as exc:
            failures.append({"invariant": str(exc), "trace": trace, "state": state_fp})
            return
        if depth == MAX_DEPTH:
            return
        for name, args in actions:
            transitions_checked += 1
            try:
                next_state, ok = apply_action(state, name, args)
            except AssertionError as exc:
                failures.append({"invariant": "rejected_transition_atomic_noop",
                                 "trace": trace + [f"{name}{args}"], "error": str(exc)})
                return
            if ok and next_state.fingerprint() != state_fp:
                dfs(next_state, depth + 1, trace + [f"{name}{args}"])

    scenario_names, scenario_failures, scenario_coverage = run_required_scenarios()
    failures.extend(scenario_failures)
    coverage.update(scenario_coverage)
    if not failures:
        dfs(State(), 0, [])

    required_coverage = {
        "atomic_reserve_and_pending_outbox", "outbox_claim", "provider_delivery",
        "pending_outbox_cancel_and_release", "ambiguous_timeout_retains_hold",
        "settlement_with_residual_hold", "reordered_usage_proves_delivery",
        "semantic_duplicate_no_effect", "monotone_correction_with_residual_hold",
        "exact_correction_replay_no_effect",
        "postfinal_correction_preserves_external_debt",
        "authoritative_finality_releases_residual", "rollover_preserves_holds_and_adjustments",
        "model_same_name_delete_recreate", "provider_route_price_same_name_delete_recreate",
        "cross_provider_binding_injection", "model_owned_route_injection",
        "duplicate_admit_byte_stable_route_response",
    }
    missing_coverage = sorted(required_coverage - coverage)
    if missing_coverage:
        failures.append({"invariant": "required_transition_coverage", "missing": missing_coverage})

    model_path = Path(__file__)
    result = {
        "schema_version": 5,
        "checked_at_utc": datetime.now(timezone.utc).isoformat(),
        "model": "bounded exhaustive outbox and residual-correction ledger exploration",
        "model_sha256": hashlib.sha256(model_path.read_bytes()).hexdigest(),
        "assumptions": [
            "every potentially billable attempt is reserved with a pending outbox before dispatch",
            "strict-mode authoritative final estimated token cost is no greater than its reservation",
            "residual reservation R-C is retained until authoritative finality",
            "prior-window provisional actual plus residual is guarded at R until finality",
            "historical downward-correction credit is audit-only and cannot expand later availability",
            "corrections are monotone-versioned; a post-finality correction voids instantaneous strict feasibility from that exogenous transition but its full debt remains visible",
            "one transactional ledger serializes every effective transition",
            "provider execution itself is not claimed exactly once",
        ],
        "depth": MAX_DEPTH,
        "states_checked": states_checked,
        "transitions_checked": transitions_checked,
        "scenario_traces_checked": scenario_names,
        "transition_coverage": sorted(coverage),
        "invariants_checked": len(INVARIANTS) + 2,
        "route_snapshot_negative_checks": 12,
        "invariants": sorted([*INVARIANTS, "rejected_transition_atomic_noop", "required_transition_coverage"]),
        "failures": failures[:20],
        "exit_code": 0 if not failures else 1,
    }
    model_path.with_name("results.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(result, indent=2, sort_keys=True))
    return result["exit_code"]


if __name__ == "__main__":
    raise SystemExit(main())
