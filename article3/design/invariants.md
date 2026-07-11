# Invariants

## Hard Invariants

These must hold in every mode unless the system explicitly abstains or rejects.

### I1. Governance Safety

No admitted request may be assigned to a model that violates:

- sovereignty zone constraints
- sensitive data provider restrictions
- explicit provider deny rules
- route availability constraints

### I2. Tenant Isolation

Reservations and settlements for tenant `t1` must not consume the budget of tenant `t2`.

### I3. Idempotent Settlement

Repeated settlement events for the same request must not double-charge the ledger.

### I4. Non-Negative Ledger

For every tenant and window:

- settled budget is non-negative
- reserved budget is non-negative
- released reservation amount is non-negative

### I5. Explainable Decision

Every non-trivial decision must have a machine-readable reason:

- admitted with chosen model and reservation
- queued with queue reason
- rejected with blocking reason
- abstained with confidence or evidence reason

## Strict-Mode Invariants

These are intended for a deterministic safety mode.

### S1. Deterministic Budget Safety

If GOV-AR runs in strict mode and its reservation upper bound is valid, then:

- a request is admitted only if the reserved amount fits within remaining tenant budget

### S2. No Overcommit by Construction

In strict mode:

- `settled + reserved <= budget`

must hold at admission time for every tenant.

## Risk-Bounded Invariants

These depend on statistical assumptions and calibration quality.

### R1. Calibrated Reservation

For an admitted request with reservation quantile level `q`,
the realized cost should exceed the reserved cost no more often than the target risk level, up to calibration error and distribution shift.

### R2. Portfolio Risk Allocation

If per-tenant risk budget `alpha_t` is respected, then aggregate overshoot probability should remain bounded by the chosen policy envelope under the stated dependence assumptions.

## Operational Invariants

### O1. Atomic Admission Outcome

A request admission is valid only if the following commit atomically from the decision layer perspective:

- chosen action
- chosen model when admitted
- reservation record
- request identifier

### O2. Reservation Release

Every terminal request outcome must eventually trigger one of:

- settlement and release
- expiry and release
- cancellation and release

### O3. Conservative Degradation

When evidence is stale, missing, or drift alarms fire, GOV-AR must degrade toward safer actions:

- lower-capacity admission
- cheaper or better-calibrated model
- queue
- abstain
- reject
