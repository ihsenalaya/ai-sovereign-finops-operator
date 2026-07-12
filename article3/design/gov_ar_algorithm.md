# GOV-AR evaluated policy

## Prior-art status

The reserve--dispatch--reconcile pattern is not the algorithmic contribution. R55 implements atomic gateway/PostgreSQL pending-spend reservation and provider-usage settlement; R56 documents the same budget-envelope arithmetic in integer microdollars; R57 uses two-phase agent-call cost control. GOV-AR uses these known mechanisms to compare reservation/routing policies under a stricter fault model.

## Required reservation policies

For each eligible deployment and immutable pricing snapshot, the implementation exposes:

- strict provider-cap reservation;
- output-cost mean;
- mean plus frozen fixed margin;
- fixed offline quantile;
- adaptive quantile without joint routing;
- R55/R56-style fixed estimate plus safety multiplier;
- R01-style locked adaptive estimator;
- GOV-AR allocation over concurrent tenant liabilities.

The no-control, settled-only, expected-cost router, oracle-future-cost, and required routing baselines share the same request streams and hidden outcome tables. Oracle outcomes are never exposed to online methods.

## Prediction and reservation

Known input cost and every billable category are represented explicitly. The output predictor returns an empirical or modeled distribution plus calibration cohort, support, freshness, and coverage diagnostics. A reservation is an integer monetary amount computed under the selected policy. Risk allocation across a prespecified fixed cohort is known chance-constraint machinery and is not claimed novel. For the theorem-bearing policy, a common pre-outcome field `G` fixes exactly `N` arrival-opportunity identities/features and non-negative slot weights `w_i` with `sum w_i = 1` before any provider outcome. It assigns `alpha_i = alpha_K w_i`; the default comparison uses `w_i = 1/N`. Admission indicator `A_i` may depend on `H_i`, but a non-dispatched slot has `A_i=0`, zero reservation, and an empty under-reservation event; unused weight is not redistributed. The weights may be frozen from development-only workload classes, but cannot react to completion, output length, or frozen-test outcomes. Therefore the allocation cap holds pathwise despite adaptive admissions. A dynamic active-set allocator may be evaluated as a separately named empirical policy, but it inherits no fixed-cohort theorem.

When support is insufficient or drift invalidates calibration, the service enters visible conservative mode and chooses a frozen fallback: provider-enforced strict cap where complete, a larger validated bound, queue, approval, abstention, or rejection. It stops reporting the calibrated risk target until a prespecified revalidation succeeds.

## Joint decision

1. Authenticate and bind tenant/workload identity.
2. Build a versioned hard-feasibility snapshot and discard every ineligible deployment.
3. If approval is required, return `REQUIRE_APPROVAL`; if evidence is insufficient, return `ABSTAIN`; if policy forbids service, return `REJECT`.
4. For every eligible deployment, compute each policy's reservation from prompt-visible features only.
5. For candidate `m`, compute the frozen score `J(r,m) = q_hat(r,m) - lambda_c E_hat[C|r,m] - lambda_l l_hat(r,m) - lambda_s 1[m != previous_route]`, with all features and non-negative coefficients frozen from development data. Maximize `J` only among hard-feasible candidates whose integer reservation fits the locked availability view. Break exact ties by ascending immutable deployment ID. Priority changes queue order or a prespecified coefficient; it never bypasses feasibility.
6. If no candidate fits, return the prespecified `QUEUE` or `REJECT` outcome.
7. Atomically commit the selected reservation, immutable snapshots, and dispatch outbox.
8. Dispatch only from the committed outbox. Separately reserve any retry/fallback attempt unless verified provider idempotency makes it non-billable.
9. Settle, late-settle, or correct using authoritative usage through the state machine. Missing evidence moves to `UNRESOLVED` without release.

## Decisions

- `ADMIT`: reservation and outbox commit succeeded; response includes deployment, attempt, monetary hold, method, allocated risk, policy/pricing versions, expiry semantics, and trace ID.
- `QUEUE`: no dispatch/reservation; retry ordering and expiry are durable and reason-coded.
- `REJECT`: hard policy or budget outcome that will not improve within the frozen queue rule.
- `ABSTAIN`: the controller lacks evidence for a governed decision.
- `REQUIRE_APPROVAL`: a versioned approval is required before a new decision attempt.

## Drift and fault semantics

- Drift fallback is operational; it does not retroactively restore statistical validity.
- Queue expiry and unambiguously undispatched reservations may release.
- `DISPATCH_PENDING`, `DISPATCHED`, and `UNRESOLVED` holds survive deadline and window rollover.
- Different settlement IDs for the same request/attempt are semantic duplicates after the first effective settlement.
- The first usage posting is provisional: it posts observed `Y_i` and retains residual hold `R_i-Y_i`. Monotone-versioned corrections move only their delta between provisional cost and residual. At rollover, an on-time provisional record also guards `Y_i`, so the new enforcement exposure remains `R_i` until finality; historical downward corrections never create reusable availability. Authoritative finality releases the applicable residual/rollover guard once. Strict mode excludes upward correction after finality.
- Provider execution and database commit are not called exactly once; only ledger effects are.

## Frozen falsifier

Before final outcomes, freeze a primary risk event, service/quality/utilization metrics, equivalence or noninferiority margins, matched-risk interpolation rule, Pareto/scalar decision rule, independent units, rare-event interval, and multiplicity family. If a faithful practical envelope, locked adaptive estimator, strict bound, or fixed/adaptive quantile is equivalent or Pareto-nondominated in the prespecified regimes, remove the advantage claim and report the null or negative result.
