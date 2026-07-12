# Reserve--dispatch--settle state machine

## Decision-only outcomes

- `QUEUED`: no provider attempt and no active monetary reservation; retry creates a new decision attempt under the same workload identity.
- `REJECTED`: terminal policy/budget denial with a machine-readable reason.
- `ABSTAINED`: terminal refusal to make a governed decision because evidence is insufficient.
- `REQUIRE_APPROVAL`: no dispatch; a later approved request re-enters with a versioned approval reference.

Queue or approval expiry releases no provider liability because dispatch was never authorized.

## Admitted request/attempt lifecycle

`NEW -> RESERVED -> DISPATCH_PENDING -> DISPATCHED -> SETTLED_PROVISIONAL -> FINALIZED`

Additional transitions are:

- `RESERVED -> CANCELED_UNBILLED` only in one transaction that cancels the pending outbox before releasing its hold, or with authoritative proof that no dispatch/outbox delivery occurred;
- `RESERVED -> EXPIRED_UNDISPATCHED` only by an atomic compare/cancel of the still-pending outbox followed by release; a concurrent or later claim then fails;
- `DISPATCH_PENDING -> UNRESOLVED` when delivery outcome is unknown;
- `DISPATCHED -> UNRESOLVED` on timeout, client disconnect, missing telemetry, or response-without-usage;
- `DISPATCH_PENDING -> FAILED_UNBILLED` only when the outbox/provider proves that no billable attempt was delivered;
- `DISPATCH_PENDING|DISPATCHED -> SETTLED_PROVISIONAL` when authoritative usage arrives; usage received before a delivery acknowledgement is itself evidence of billable delivery;
- `UNRESOLVED -> LATE_SETTLED_PROVISIONAL` when authoritative usage arrives after the deadline or window rollover;
- either provisional state `-> CORRECTED_PROVISIONAL` for the next authoritative correction version;
- any provisional state `-> FINALIZED|LATE_FINALIZED` only on authoritative usage finality, which releases the residual correction hold;
- replayed or semantically duplicate events in any terminal state are recorded as inbox no-ops.

`UNRESOLVED` is not a released terminal state. Its conservative hold remains in the tenant availability calculation and carries across budget-window renewal. Operational policy may escalate it for investigation or conservatively convert it to a declared maximum debt, but cannot treat missing evidence as zero cost.

## Atomic database effects

### Reserve

One serializable transaction:

1. binds authenticated workload/tenant and immutable policy/pricing snapshots;
2. verifies no active or terminal record conflicts with the globally unique request/attempt identity;
3. locks the tenant enforcement row;
4. checks `settled + carried_debt + unresolved_liability + new_reservation <= budget`;
5. inserts the reservation and increments outstanding liability;
6. inserts a dispatch outbox record in `pending` state.

No provider call is allowed before commit.

### Dispatch

An outbox worker claims the versioned record, marks `DISPATCH_PENDING`, and performs the provider attempt. Provider idempotency is used only when its semantics are verified. Otherwise each retry is a distinct, separately reserved attempt. A delivery receipt marks `DISPATCHED`. A crash or ambiguous timeout leaves `DISPATCH_PENDING` and therefore charged.

### Provisional settle and late settle

One transaction locks the request/attempt, deduplicates both event ID and effective request settlement, validates pricing/usage version, moves observed `Y_i` from the active hold into provisional estimated cost, retains residual hold `R_i-Y_i`, writes an immutable event/inbox row, and updates aggregates. Different event IDs for the same effective usage cannot charge twice. A duplicate acknowledgement after commit is a no-op. Missing usage leaves latent `C_i*` unknown and the existing hold charged.

### Correction and authoritative finality

A correction has a unique monotone version/ID and authoritative predecessor. The ledger applies `new_actual - previous_actual` once while changing the residual hold by the negative delta. Within the origin window, provisional cost plus residual remains the reservation. After rollover, an on-time provisional record adds a temporary guard equal to its provisional cost, so guard plus residual remains the reservation until finality. Negative historical corrections become origin-window audit credits only; they cannot expand a later enforcement window. Late-settled cost and positive late correction deltas become non-negative carried debt. An authoritative finality event releases the remaining residual and any temporary rollover guard exactly once. Strict-mode assumptions exclude upward corrections after finality; a violation is recorded as external debt and invalidates the theorem for that attempt.

### Authoritative unbilled cancellation/failure

Only a state proving that no provider attempt was delivered may release the hold. Client cancellation, timeout, expiry, network partition, or missing response is not such proof after `DISPATCH_PENDING`.

## Budget-window renewal

Renewal creates a new active window but preserves unresolved holds and non-negative carried debt. For on-time provisionally settled records it also adds a temporary guard so guard plus residual remains `R_i`; finality releases both without recharging finalized origin-window spend. The immutable origin window remains the reporting attribution. Late usage moves part of an unresolved hold to carried debt; corrections move exposure between debt/guard and residual. Historical credit is audit-only and cannot be reused in later windows. Carried debt persists until explicit repayment or an authorized budget adjustment and never disappears merely because the origin window closed. The experiment reports both safety and the refusal/backlog cost of this conservative rule.

## Required reason codes

Every transition and no-op records request/attempt ID, tenant/workload UID, previous/new state, effective ledger delta, event ID/version, policy/pricing versions, trace ID, and one stable reason code. At minimum the API distinguishes governance infeasibility, approval required, budget unavailable, insufficient calibration, drift fallback, duplicate request, duplicate event, already effectively settled, unresolved timeout, authoritative unbilled cancellation, late settlement, correction, and invalid transition.
