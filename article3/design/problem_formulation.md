# GOV-AR problem and safety formulation

## Scope and prior-art boundary

GOV-AR studies admission under uncertain provider cost when requests overlap and usage arrives after dispatch. Gateway-side reserve-before-dispatch, `budget - spent - reserved`, PostgreSQL pending-spend accounting, hierarchical budgets, approvals, and actual-cost reconciliation are existing practice (R55--R57). They are implementation ingredients, not contributions.

The candidate scientific contribution is narrower: define non-equivalent monetary risk events and compare reservation policies under concurrency, tenant isolation, drift, and faulty settlement. The operator and gateway are the reproducible test vehicle.

## Units and immutable identities

Money is represented as integer micro-units of the experiment currency. Every request record binds:

- tenant ID, authenticated workload UID, namespace, application, and budget-window ID;
- globally unique request ID and provider-attempt ID;
- selected deployment, provider, region, route, and immutable policy/catalog snapshots;
- pricing version and the unit prices for every billable category;
- known input usage and an output-cost predictive distribution;
- reservation method, reserved amount, allocated tail probability, and calibration cohort;
- lifecycle state, outbox/inbox event IDs, deadlines, and timestamps;
- authoritative usage, estimated token cost, corrections, and terminal reason.

Provider execution is not transactionally atomic with PostgreSQL. “Exactly once” refers only to effective ledger mutations for a request/attempt/event identity.

For every delivered attempt, `C_i*` denotes its latent provider-billable token-derived cost under the immutable price snapshot, whether or not usage telemetry is observed. `Y_i` denotes the usage-derived value visible to the ledger; it exists only when authoritative usage is received. A missing `Y_i` does not imply `C_i*=0`. The ledger instead retains a hold `L_i`. Trace experiments can compare `L_i` with the hidden `C_i*`; live missing-usage cases are censored and reported with prespecified bounds from provider caps and reservation assumptions, not silently excluded or assigned zero.

## Tenant budget state

For tenant `t` at decision time `k`, let:

- `B[t,w]` be the declared budget for window `w`;
- `S[t,w,k]` be effective settled estimated token cost charged to that window;
- `L[t,k]` be unresolved, residual correction, and provisional-rollover guard liability across active and provisionally settled requests, including liabilities originating in earlier windows;
- `D[t,k]` be non-negative late/correction debt carried into the active enforcement view.

The enforced availability is

`A[t,w,k] = B[t,w] - S[t,w,k] - L[t,k] - D[t,k]`.

An admission transaction may commit only when its reservation fits this view. Window renewal never erases a potentially billable in-flight liability. A dispatched timeout, client disconnect, or missing usage event moves the record to `UNRESOLVED`; it does not release the hold. Release requires authoritative evidence that no billable attempt occurred. The first usage event moves `Y_i` from the reservation into provisionally settled cost but retains residual hold `R_i-Y_i`. Monotone-versioned corrections move only their delta between provisional cost and that residual. Authoritative finality releases the remainder. Strict-mode safety assumes the final authoritative `C_i* <= R_i` and no upward correction after finality; violations become visible debt outside the theorem rather than being hidden.

The origin window remains immutable for reporting `H[t,w]`; the current enforcement window is a separate field. At rollover, unresolved holds and carried debt persist. For a provisional on-time settlement, the new enforcement view temporarily guards both its observed amount and residual, keeping total exposure at `R_i` until finality; finality releases that guard because the finalized cost remains attributed to the closed origin window. Late cost and positive later correction deltas become carried debt. A historical downward correction is recorded as audit credit against the origin window but never expands a later window's availability. Carried debt remains until explicit repayment or an authorized budget adjustment; it does not age away automatically.

## Hard feasibility before optimization

For request `r`, deployment `m` is eligible only if all prespecified checks pass:

- authenticated tenant/workload identity and policy binding;
- provider/model existence, readiness, routability, and deployment availability;
- residency/region and sensitive-workload restrictions;
- quality-gate state and minimum-quality requirement;
- approval and override state;
- pricing freshness and complete billable-category coverage;
- provider-attempt and output-cap semantics required by the selected safety mode.

An ineligible deployment cannot be restored by a soft score. Empty or untrustworthy eligible sets produce `ABSTAIN`, `REQUIRE_APPROVAL`, or a policy-specific `REJECT`.

## Decision

For an eligible request, the controller chooses one of `ADMIT`, `QUEUE`, `REJECT`, `ABSTAIN`, or `REQUIRE_APPROVAL`. `ADMIT` jointly fixes the deployment, provider attempt policy, reservation, risk allocation, policy/pricing versions, expiry semantics, and outbox record. Utility may include quality, latency, monetary cost, switching cost, and priority, but hard feasibility and the atomic budget check dominate the objective.

## Four distinct risk events

Let `K` be a set of arrival opportunities fixed before provider outcomes. For slot `i`, `A_i` is the predictable admission/dispatch indicator, `C_i*` is latent authoritative estimated token cost when `A_i=1`, `R_i` is the pre-dispatch reservation (zero when `A_i=0`), and `W_i` is its tenant window.

1. **Request under-reservation:** `U_i = {A_i=1 and C_i* > R_i}`.
2. **Fixed-opportunity-cohort liability exceedance:** `F_K = {sum(i in K) A_i C_i* > sum(i in K) A_i R_i}`. Necessarily `F_K` is a subset of `union(i in K) U_i`, because if every admitted cost is at most its reservation, their sums preserve the inequality.
3. **Selected outstanding-set exceedance:** at time `k`, with `O_k` selected by dispatch/completion/censoring history, `G_k = {sum(i in O_k) C_i* > sum(i in O_k) R_i}`.
4. **Tenant budget-window overshoot:** `H[t,w] = {sum(i: tenant(i)=t and charge_window(i)=w) C_i* > B[t,w]}` under the frozen charge/carryover rule.

These events are not interchangeable. A per-request marginal quantile does not automatically control `G_k` after duration-dependent selection, and neither a request nor cohort bound is a budget-window guarantee.

## Defensible conditional results

### Conditional strict-ledger feasibility

If every billable attempt/category is included, the immutable price is correct, provider-enforced caps ensure `C_i* <= R_i`, dispatch cannot occur without a committed hold, residual and provisional-rollover exposure is retained until authoritative finality, historical credits cannot expand later availability, and every effective transition is serialized, then `S + L + D <= B` is preserved by reserve, dispatch, provisional settle, correction, finality, rollover, and authoritative unbilled cancellation. This is conditional ledger arithmetic, not a universal provider-billing guarantee.

### Fixed-cohort union bound

Let a common earlier sigma-field `G` fix the opportunity cohort `K`, the experimental design, and target `alpha_K`; thus `K` and `alpha_K` are `G`-measurable. For each `i`, let `H_i` be the information available at its admission decision, with `G` contained in `H_i`. The indicator `A_i`, reservation, selected route, and `alpha_i` are `H_i`-measurable. If, on `A_i=1`, the selected-route tail statement satisfies `P(C_i* > R_i | H_i, A_i=1) <= alpha_i`, and `sum(i in K) alpha_i <= alpha_K` almost surely, then `E[1(U_i) | H_i] <= A_i alpha_i <= alpha_i` and

`P(F_K | G) <= P(union(i in K) U_i | G) <= sum(i in K) E[alpha_i | G] <= alpha_K`.

This follows from `F_K` being a subset of the union, the conditional union bound, and iterated expectation. No independence is required. If only unconditional marginal tail statements are available, only the corresponding unconditional bound follows; conditioning cannot be added after the fact. The result does not imply a selected-outstanding-set or budget-window bound. Those require a selection-valid construction or an explicit non-informative delay/censoring assumption that must be falsified experimentally.

## Statistical units and falsifier

Requests are nested observations, not independent inferential units. In the trace studies, the matched seed/stream is the primary independent block and tenant windows are nested unless the frozen data-generating design proves otherwise. Kind cluster recreations are independent performance units. Azure sampling windows are sampling clusters with within-window dependence; the final protocol must justify, rather than assume, independence across windows/deployments.

The advantage claim fails if a faithful R55/R56 fixed envelope, an R01-style locked adaptive estimator, strict reservation, or fixed/adaptive quantile reservation is equivalent or Pareto-nondominated on the frozen risk, utilization, served quality, refusal, availability, and fault metrics. Equivalence margins, the matched-risk rule, multiplicity family, and claim-removal decision are frozen before final outcomes.

## Two-contribution limit

Subject to frozen evidence, the paper may claim only:

1. this problem/safety/event formulation with assumption-scoped ledger invariants and fixed-cohort bound; and
2. a preregistered comparative measurement study of the resulting risk--utilization, isolation, drift, fault, and local-overhead trade-offs.

It does not claim a new routing algorithm, risk-allocation method, transaction primitive, budget envelope, or gateway integration.
