# GOV-AR invariants and assumptions

## Ledger invariants

1. **Integer accounting:** budget, settled cost, outstanding/residual/rollover-guard liability, carried debt, historical audit credit, and per-record actual cost are non-negative integer monetary units. Historical credit is not an enforcement offset.
2. **Record/aggregate equality:** tenant outstanding liability equals the sum of holds and rollover guards for active, unresolved, and provisionally settled records; aggregate settled cost, carried debt, and audit credit equal effective request outcomes and corrections in the frozen views.
3. **Atomic feasibility:** a committed reservation and dispatch outbox exist only if the locked tenant view had sufficient availability. A provider attempt cannot precede that commit.
4. **One effective settlement/finality:** any number of identical or differently keyed duplicate usage deliveries causes at most one base settlement effect for a request/attempt. Corrections apply only their monotone-versioned delta once, and finality releases the residual once.
5. **No double release:** an authoritative unbilled cancellation, settlement, late settlement, or correction cannot release the same hold twice.
6. **Unresolved and correction carryover:** expiry, timeout, disconnect, missing usage, ambiguous delivery, and window rollover do not reduce the hold of a potentially billable attempt. Provisional settlement retains `R_i-Y_i`; after rollover, an on-time provisional record also guards `Y_i`, keeping current exposure at `R_i`. Corrections move deltas between cost/debt/guard and residual; only authoritative finality releases the applicable remainder.
7. **No reusable historical credit:** a downward correction attributed to a closed window may reduce that window's reported cost, but never increases current or future admission availability.
8. **Tenant/workload isolation:** an event authenticated for one tenant/workload UID cannot mutate another tenant's record or aggregate.
9. **Price/version determinism:** settlement uses the immutable pricing/billable-category snapshot bound to the attempt; a later price change does not rewrite prior cost except through an explicit correction.
10. **State validity:** terminal/no-op transitions are absorbing except for explicitly versioned provisional correction/finality transitions. Upward post-finality correction is outside the strict-mode guarantee and becomes visible external debt.
11. **Outbox/inbox boundary:** PostgreSQL can make ledger/outbox/inbox effects atomic; it does not make external provider execution exactly once.

## Hard governance invariants

An admitted route passes every frozen provider, deployment readiness, routability, region/residency, workload sensitivity, quality-gate, approval, price freshness, and provider-attempt eligibility check. Optimization cannot override a failed hard check. Every decision and transition has a stable reason code and immutable policy/pricing references.

## Conditional strict-ledger feasibility

The invariant `settled_current + carried_debt + outstanding_residual_and_guard_liability <= active_budget` is deterministic only under all of these assumptions:

- every potentially billable provider attempt is separately reserved before dispatch;
- provider-enforced token/charge caps cover input, output, cached input, hidden reasoning, tools, media, retries, fallbacks, cancellations, and any other billed category;
- authoritative estimated token cost for an attempt never exceeds its reservation;
- residual `R_i-Y_i` and any required provisional rollover guard are retained until authoritative finality, historical credit does not expand availability, correction versions are monotone, and no upward correction occurs after finality;
- pricing and token accounting are correct and immutable for the attempt;
- the transactional ledger serializes every effective transition;
- bypass of the governed gateway path is prevented within the tested trust boundary.

Without those assumptions the result is not a hard provider-invoice guarantee. The manuscript calls it conditional strict-ledger feasibility.

## Conditional probabilistic statement

For an arrival-opportunity cohort fixed before outcomes, predictable admission indicators, selected-route tail bounds, and a pathwise allocated sum of tail probabilities imply only the fixed-opportunity-cohort union bound defined in `problem_formulation.md`. They do not imply selected-outstanding-set or tenant-window coverage. Empirical calibration is reported separately, and drift fallback stops advertising calibration rather than restoring a theorem.

## Liveness obligations

Conservative unresolved, residual, and rollover-guard holds can reduce availability indefinitely. Carried debt persists until explicit repayment or authorized budget adjustment; historical credit remains audit-only. The experiments therefore measure unresolved backlog, hold age, false refusal, time in conservative mode, operator intervention, payoff, and recovery. Safety is not presented without this liveness/utility cost.
