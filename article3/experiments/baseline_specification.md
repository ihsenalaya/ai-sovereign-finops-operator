# GOV-AR baseline specification

Status: pre-freeze specification; no result claims  
Protocol source: `article3/experiments/registry/frozen_protocol.yaml` version
`0.3-draft-candidate`  
Registry source: `article3/experiments/baseline_registry.json`

## Purpose and comparison design

This document fixes the identity and permitted information set of the Article 3
comparators before a frozen campaign. It is not evidence that a comparator has
been implemented or run. An experiment manifest may name a comparator only if
all of its acceptance tests in the registry pass against the exact source,
configuration, data, protocol, and image hashes in that manifest.

The experiment has two deliberately separated axes.

1. The **liability-control axis** compares the eleven protocol
   `budget_admission` methods on the same request order, arrival and settlement
   schedule, hard-feasible candidate set, routing decisions, prices, tenant
   windows, budgets, faults, and seeds. The router used for this axis is frozen
   from development data and its selected deployment is replayed identically to
   every non-oracle liability method. `expected_cost_router` is additionally a
   prespecified joint comparator and is reported separately from the pure
   liability contrasts.
2. The **routing axis** compares the nine protocol `routing` methods on the same
   hard-feasible candidate set and shared stream. Except for an explicitly
   labeled oracle analysis, routing methods are paired with the same frozen
   admission/reservation controller within a contrast. A result must not
   attribute a difference caused by both a changed router and a changed
   reservation rule to either component alone.

The crossed `budget_admission/gov_ar` and `routing/gov_ar` configuration is the
joint GOV-AR system. Each component is also crossed with a common comparator on
the other axis for attribution. Method IDs are namespaced in manifests as
`budget_admission/<id>` and `routing/<id>`; the duplicate protocol label
`gov_ar` is therefore never ambiguous.

## Common enforcement and information boundary

Every method, including the oracles, first receives the same production hard
feasibility result. It may never soften or trade away provider availability,
residency, sensitivity permission, model readiness, route binding, fresh
quality-gate state, approval state, context limit, verified pricing, or any
configured latency/quality guardrail. A fixed-model method returns the frozen
fallback decision when its model is infeasible; it does not bypass the filter.

At an online decision time a non-oracle method may use only:

- request identity and features available before dispatch, including exact
  input tokens, declared charge bounds, provider-enforced output cap, tenant,
  workload, and frozen-cohort identity where applicable;
- the current hard-feasible catalog and immutable model/provider/route,
  pricing, policy, and cap-evidence versions;
- the budget fields explicitly allowed for that method;
- training/calibration/development artifacts and hyperparameters frozen before
  test access; and
- previously committed feedback for the model actually dispatched by that
  method. Provider usage and deterministic quality are released only after
  dispatch/settlement through their respective audited boundaries.

It may not read a current or future output length, actual cost, unselected
model score, later arrival, later settlement, future policy/model state, or the
RouterEval oracle volume. An isolated offline trainer may read **training and
calibration** outcome matrices only when the primary published method requires
supervised full-feedback training. Such access must be declared, audited, and
produce a hash-bound artifact; the online router sees the artifact, never the
matrix. Development is used only for the prespecified selection described
below. Frozen-test matrices remain evaluator-only.

All non-oracle methods obtain online quality feedback through the dispatch-
authorized `selected_feedback.py` service. The service returns only the
selected model's score after an immutable dispatch. Usage feedback likewise
comes from the selected provider attempt. Duplicate receipts are replays, not
new observations. A method that does not learn online still generates and
settles the same selected-feedback receipt so outcome accounting remains
matched.

The two oracle methods run in an evaluator process that is unavailable to the
router and load generator. Oracle outputs are written only after all
non-oracle decisions for a shared stream are immutable. Oracles are unattainable
references, are excluded from superiority claims, and may not update a
non-oracle artifact or policy.

## Name and alias rules

The protocol names are the canonical result labels. Production strings are
recorded separately:

| Protocol label | Accepted display alias | Production method | Rule |
|---|---|---|---|
| `strict_max` | strict max-token reservation | `strict_provider_cap` | Never call `strict_provider_cap` a separate baseline. The manifest records both fields. |
| `mean_margin` | mean plus fixed margin | `fixed_margin` | `fixed_margin` is the production mapping, not a twelfth method. |
| `gov_ar` (budget axis) | GOV-AR fixed-cohort liability | `govar_fixed_cohort` | Never label `adaptive_quantile` alone as GOV-AR. |
| `gov_ar` (routing axis) | GOV-AR joint router | comparator adapter plus production engine | The axis namespace is mandatory. |
| `fixed_estimate` | static fixed estimate | `mean` used as an execution primitive | It remains distinct from empirical `mean`; the estimate is a globally configured constant, not an estimated conditional mean. |

Raw records must store the namespaced registry ID, protocol ID, production
reservation method returned by the engine, effective fallback method, and
reason code. A conservative fallback from an adaptive method therefore remains
in that method's intention-to-treat cell but records
`effective_reservation_method=strict_provider_cap`; a per-protocol analysis may
also report fallback exposure.

## Eleven budget/admission baselines

### 1. `no_budget`

**Information and rule.** The method ignores all monetary budget and liability
state. It admits every request for which the common hard filter and approval
state permit admission. It uses the shared frozen routing decision. It records
actual usage after dispatch for evaluation but never uses it to gate a later
admission.

**Implementation.** This requires a comparator adapter. Assigning a very large
budget to the production `Engine` is not a faithful implementation because it
still creates reservations and can expose reservation-driven behavior. The
adapter must retain request/event idempotence for measurement while keeping
budget control disabled and label its ledger as observational.

**Invariant.** For a matched stream, changing only budget amount, settled
spend, outstanding liability, or carried adjustment cannot change this
method's decision. Governance and approval changes may change it.

### 2. `settled_only`

**Information and rule.** The atomic admission check is
`settled_spend + carried_adjustment + actual_cost_if_already_known <= budget`.
Outstanding requests contribute zero until settlement. A request is admitted
when its hard-feasible shared route exists and the settled-only available
amount is positive under the frozen boundary convention. It creates no
pre-dispatch monetary hold; settlement atomically adds actual estimated token
cost. The window and late-correction semantics otherwise match the production
ledger.

**Implementation.** A comparator ledger adapter is required; setting a zero
reservation in the production engine would misstate the method and does not
provide its state transitions. The adapter must still provide unique requests,
idempotent settlement, principal isolation, frozen pricing at dispatch, and
complete lifecycle evidence.

**Invariant.** Pending request count and value do not change admission before
their first effective settlement. A duplicate settlement has no second effect.

### 3. `mean`

**Information and rule.** For each frozen calibration stratum and candidate,
reserve input charges plus output charges evaluated at the arithmetic mean
output-token estimate. The mean is computed only from eligible calibration
observations available before the evaluated split; it is clipped to the
verified request/provider cap. Admit only if the full production available
balance (`budget - settled - outstanding - carried`) covers the atomic hold.

**Implementation.** Reuse `Engine`/`PostgresEngine` with production method
`mean`. A policy-per-regime adapter may be needed because the CRD currently
holds one scalar. The frozen artifact must expose count, sum, stratum identity,
clip count, and digest.

**Selection.** Stratum definition and minimum support may be selected on
development from a predeclared finite grid, then frozen. Test outcomes never
change the mean.

### 4. `mean_margin`

**Information and rule.** Reserve the same calibrated mean as `mean` plus a
non-negative, fixed output-token margin, clipped to the verified cap. Admission
uses settled, outstanding, and carried liability atomically.

**Implementation.** Reuse the production engine with method `fixed_margin`.
The raw/manifest protocol ID stays `mean_margin` and records the two token
components. The margin must not be recomputed online.

**Selection.** Select one margin rule from a frozen development grid (absolute
tokens or a calibration-scale multiplier converted once to integer tokens) by
the prespecified matched risk/utilization criterion. Record the complete grid
and all development scores. No test-cell-specific margin is permitted.

### 5. `fixed_quantile`

**Information and rule.** Reserve a one-sided empirical or split-conformal
output-token order statistic computed once from the calibration split for the
frozen feature/price/cap regime. The coverage target and finite-sample rank are
fixed before test access. The estimate is clipped to the verified cap;
admission atomically accounts for all outstanding liability.

**Implementation.** Reuse the production engine with `fixed_quantile` and
`fixedQuantileOutputTokens`. This method has no online update, no per-request
risk allocation, and must not be labeled adaptive or GOV-AR.

**Selection.** The finite target grid may be compared on development. The
selected target, rank convention, support rule, strata, and tie convention are
then frozen globally or by a prospectively declared regime.

### 6. `strict_max`

**Information and rule.** Reserve every billable request-declared maximum,
including the exact input charge and provider-enforced `max_output_tokens`,
plus verified bounds for retries, tools, media, time, and other priced bases.
Unknown exact input is replaced by the conservative context-window bound. If a
complete provider-enforced cap or price component cannot be verified, fail
closed using the configured non-admit fallback.

**Implementation.** Reuse the production engine with
`strict_provider_cap`. There is no parameter tuning and allocated risk is zero.
Deterministic budget safety is evaluated only under the formal assumptions,
including authoritative actual cost not exceeding every enforced bound.

**Invariant.** An admitted request's frozen component bounds cover every
authoritative final usage component under the stated strict-mode assumptions;
the engine never advertises nonzero risk.

### 7. `fixed_estimate`

**Information and rule.** Reserve one globally configured, model-independent
output-token estimate `E_fixed` for every request, clipped to each verified
cap, plus exact known input and other declared charge bounds. This represents a
static gateway estimate, not an empirical conditional mean. Admission uses the
full atomic reserve-settle ledger.

**Implementation.** Reuse the production engine's `mean` primitive with
`meanOutputTokens=E_fixed`, but record `budget_admission/fixed_estimate` as the
method identity and `mean` only as the production primitive. A policy adapter
must assert that `E_fixed` is identical across models, tenants, workloads, and
test cells.

**Selection.** `E_fixed` is taken from the prospectively declared gateway
configuration or selected once on development from a finite grid. It cannot be
the test mean and cannot vary by feature or candidate.

### 8. `adaptive_quantile`

**Information and rule.** Begin from a digest-bound calibration quantile and
update it only at prespecified epochs using prior, selected-at-dispatch output
usage whose finality and regime eligibility are verified. There is no
concurrent-request risk allocation. Drift, insufficient support, stale
evidence, cap change, price-regime change, or feature-regime change visibly
invalidates calibrated mode and invokes the frozen conservative fallback.

**Implementation.** Reuse `adaptive_quantile` only with controller-published
PostgreSQL-v8 calibration evidence and exact status/spec digests. The routing
adapter must prevent a global status artifact from being applied to a different
candidate price/cap regime.

**Selection.** Coverage target, update batch/epoch, history window, minimum
support, detector, threshold, and fallback are selected on development from a
finite grid and frozen. Test feedback changes the estimate only through this
fixed update rule.

### 9. `expected_cost_router`

**Information and rule.** This is the prespecified expected-cost joint
comparator. For each hard-feasible candidate it predicts expected output
tokens and selected-feedback quality from training/calibration and prior
selected observations. It selects the candidate maximizing the frozen expected
quality-cost utility subject to its **expected** monetary reservation fitting
the production available balance; it then atomically reserves that expected
cost. It does not use an upper-tail estimate or cohort risk allocation.

**Implementation.** A comparator router must select a singleton candidate and
invoke the production engine with `mean` and the candidate/regime-specific
expected output estimate. The common engine supplies reserve/dispatch/settle
atomicity. Passing all candidates to the current engine is not equivalent to
this rule because the engine uses a lexicographic objective.

**Selection.** Predictor class, regularization, exploration (if any), and the
quality-cost utility coefficient are selected on training/development only.
The finite grid and tie-breaking by model ID are frozen.

### 10. `oracle_future_cost`

**Information and rule.** The isolated evaluator takes the already frozen
non-oracle routing decision and reveals that selected route's realized future
estimated token cost before admission. The method reserves exactly that amount
and admits iff the production available balance covers it. It does not inspect
unselected quality or change the route. Consequently it isolates the value of
perfect cost foresight from routing foresight.

**Implementation.** Evaluator-only adapter plus a production ledger. It must be
run after the paired non-oracle route record is immutable. Per-request policy
construction may use `mean` as an execution primitive, but the manifest and raw
rows must say `oracle_future_cost`. No online method or artifact may consume
the oracle decision or future cost.

**Invariant.** Reserved cost equals the evaluator's hash-bound final actual
estimated token cost for the selected provider attempt. The method is excluded
from confirmatory superiority tests.

### 11. `gov_ar`

**Information and rule.** The liability component uses a prospectively frozen
tenant opportunity cohort. Tenant risk `alpha_K` is divided by immutable slot
weights, with `alpha_i = floor(alpha_K * weight_i / 1e9)`. Each slot must use an
upper reservation whose bound is valid at its allocated `alpha_i` for the
exact feature, price, cap, split-opportunity, and cohort regimes. The joint
router chooses only hard-feasible candidates and applies the frozen
quality/cost/switch objective among candidates whose reservation fits
`budget - settled - outstanding - carried`. Drift or invalid evidence stops the
calibrated-risk label and invokes the visible conservative fallback. Reserve,
dispatch, and settlement use the production atomic/idempotent ledger.

**Implementation.** The intended production method is
`govar_fixed_cohort`, not `adaptive_quantile`. Frozen cohort membership,
opportunity digests, weights, risk, authority proof, protocol/data/config/
software hashes, route schema, and ledger layout must all match. The production
engine can provide the ledger and cohort checks, subject to the blockers below.

**Selection.** Tenant risk targets are protocol factors rather than fitted
outcomes. Weight rule, quantile construction, joint utility, switch penalty,
detector, and fallback are selected on development only and frozen. Test
outcomes may update calibration only under the frozen adaptive rule; cohort
membership and slot weights never change.

## Required routing baselines

All routing methods receive the common hard-feasible set before their own rule.
For the primary routing contrast they are paired with the same production
liability method; their router output is committed before selected feedback is
released.

### `always_premium`

Freeze one designated premium model from training/calibration metadata and
objective quality evidence. Route every request to that model when it is hard
feasible and return the frozen fallback decision otherwise; do not substitute a
second model. Premium identity, selection rule, and tie-break are hash-bound.
The production engine can be reused by passing only the designated candidate.

### `always_cheapest`

Freeze one designated catalog model with the lowest expected charge under the
development reference request mix. Route every request to that model when hard
feasible; otherwise return the frozen fallback without substitution. This
fixed-model rule is distinct from the next per-request eligible-minimum rule.
The engine is reused with a singleton candidate.

### `cheapest_governance_compliant`

For every request choose the hard-feasible candidate with minimum predicted
integer charge under the common reservation method, breaking ties by canonical
model ID. It may choose a different fallback model when the nominal cheapest is
infeasible. The production cost objective implements this only when the
reservation inputs are candidate-correct; a comparator adapter must assert the
candidate ordering and pass the selected singleton for an unambiguous replay.

### `current_operator_weighted_score`

Use the existing `internal/routingscore` implementation and its frozen default
weights: cost 0.40, catalog-tier quality 0.30, observed latency 0.20, and
reliability 0.10. Cost and latency are min-max normalized over the currently
eligible observed groups; absent latency receives the explicit neutral score
0.5; sovereignty remains a hard gate. Only prior selected usage telemetry is
available. Ties use canonical model ID. This is a production-code comparator,
but a router adapter is required to turn its recommendation into a singleton
synchronous admission candidate.

### `routellm_or_hybridllm_compatible`

Use the official RouteLLM implementation when the frozen benchmark exposes a
compatible two-model preference-routing topology. Otherwise use a faithful
Hybrid LLM difficulty-router reproduction for a frozen weak/strong pair. The
chosen alternative is fixed before test access and is not switched after
seeing results. Official training uses training data; threshold/quality knob is
chosen on development. The selected model is then passed through the hard
filter and common production ledger. RouteLLM's official repository/archive,
dependency lock, model artifact, pair mapping, and exact threshold must be
recorded. If neither method is compatible, the exclusion needs primary-source
evidence and the strongest compatible published router; it cannot be silently
replaced by a home-grown score.

### `pilot_or_closest_budget_router`

Prefer the official PILOT artifact from R04: its offline preference prior is
trained only on authorized training data and its online LinUCB refinement sees
only selected feedback. The remaining-horizon budget exposed to PILOT is the
frozen expected-cost routing budget, not future actual cost. Production
reserve-settle accounting wraps the selected action and remains the authority
for hard admission. If the artifact or benchmark is incompatible, reproduce
the closest implementable published budget router (in priority order specified
before test: PILOT, a compatible R63 MESS+-style router, then an R71
SLARouter-style router) from primary equations. Record why the earlier choice
failed. No unavailable method may be silently omitted or mislabeled official.

### `contextual_knn_router`

Implement the simple nonparametric comparator reported with ContextualRouter
(R60): retrieve `k` nearest authorized training-history query embeddings and
estimate each candidate's score from those neighbors; select the candidate
maximizing the frozen quality-cost rule. Distance, normalization, missing-model
rule, `k`, cost coefficient, and deterministic tie-break are selected on
development. An isolated trainer may construct the training history; the online
router receives the index and only selected feedback. Because the primary page
did not identify an official artifact, this must be labeled a faithful
reproduction, not official ContextualRouter.

### `oracle_quality_cost`

After non-oracle decisions are immutable, the evaluator reads all hard-feasible
frozen-test candidate scores and realized candidate costs for the current
request and selects the maximum of the frozen utility `quality - lambda * cost`
(with cost expressed in the registered normalized unit), breaking ties by
lower cost then model ID. It is a per-request routing oracle, not a clairvoyant
global-window optimizer. The selected route is passed to the common admission
method so routing and admission oracles remain separable. This method is never
used for tuning or confirmatory superiority claims.

### `gov_ar`

The joint routing component evaluates the hard-feasible candidate-specific
upper reservation and frozen selected-feedback quality estimate, rejects
candidates whose hold does not fit the production available balance, and
maximizes the frozen quality/cost/switch utility with canonical tie-breaking.
It then invokes `govar_fixed_cohort` atomically for the selected candidate. Its
online quality estimator receives only selected feedback. The routing and
reservation artifacts must be bound to the same candidate price/cap and cohort
regime; otherwise it enters the declared conservative fallback and cannot
advertise calibrated risk.

## Companion practical-envelope comparators

These are mandatory novelty controls but do not change the protocol's count of
eleven budget/admission IDs.

- `faithful_r55_gateway_fixed_multiplier` must execute or faithfully port the
  public `day0ops/quota-management` implementation at commit
  `c72d26a7f74f761a9f871b91b8c320db0825db25`. It uses its static estimate and
  safety multiplier with pending-spend accounting. Its native expiry/late-event
  behavior is preserved and measured; suspected defects are not silently
  repaired. Any normalized port is validated event-for-event against the
  pinned artifact and is labeled a port.
- `faithful_r56_budget_envelope` is a documentation-conformance reproduction
  of the Keel integer-microdollar equation
  `remaining = total - reserved - spent`, pre-dispatch estimate lock, and
  post-response actual-minus-locked reconciliation. No public implementation
  was verified in the literature corpus, so it must not be called official.
  Documented examples and boundary cases become immutable conformance fixtures.
- `r01_locked_adaptive_estimator` reproduces the adaptive estimator and
  reservation receipt behavior of R01. The linked Rust artifact/crate must be
  pinned by immutable source revision and checksum before implementation can be
  accepted; until then this comparator is blocked rather than approximated
  without disclosure.

The `fixed_estimate` and `mean_margin` methods remain controlled internal
baselines; the R55/R56 methods are separate faithful-prior-work checks and must
not be merged merely because some nominal parameters coincide.

## Implementation blockers that must be resolved before freeze

1. **Per-slot risk is not bound to the executed quantile.** The current
   `govar_fixed_cohort` path validates and records
   `allocated_risk_ppb = floor(alpha_K * weight_i / 1e9)`, but
   `reservationTokens` reads one policy-level
   `status.govar.calibration.adaptiveOutputTokens`. It does not verify that the
   artifact's `CoverageTargetPPB` equals `1e9 - allocated_risk_ppb`, and it
   cannot execute heterogeneous slot quantiles from one status object. A
   faithful GOV-AR run requires either a digest-bound per-slot/per-risk artifact
   lookup verified inside the transaction or a prospectively restricted
   uniform allocation with a single matching target plus an explicit engine
   check. Merely recording allocated risk is insufficient.
2. **One adaptive status is tied to one price/cap regime.** Validation compares
   the global routing-policy calibration status with each candidate's price and
   cap regime. A single policy therefore cannot faithfully quantify multiple
   candidates with different regimes. Candidate-specific immutable artifacts
   and atomic selection, or a pre-admission adapter whose selection and
   artifact are transactionally bound, are required.
3. **The production choice is lexicographic, not the specified joint utility.**
   `chooseAdmission` selects quality-first, latency-first, or minimum
   reservation, and contains no switch penalty. The joint GOV-AR router and the
   `no_switch_penalty` ablation need one explicit, tested, integer/deterministic
   utility and candidate-specific reservation binding. Until implemented, the
   engine provides ledger enforcement but not the claimed joint router.
4. **Comparator adapters do not yet constitute validated methods.**
   `no_budget`, `settled_only`, expected-cost routing, oracles, and published
   routers require raw lifecycle emission and information-access assertions.
   A manifest label alone is not acceptance evidence.
5. **Published artifact pins remain incomplete.** Exact immutable artifact
   checksums/dependency locks are required for PILOT and RouteLLM. The current
   corpus identified no official code URL for QUARTZ, MESS+, ContextualRouter,
   or SLARouter; reproductions must be equation/fixture validated and labeled
   accordingly. R01's linked artifact revision is also unresolved.
6. **Static scalar policies need regime-safe construction.** `mean`, margin,
   fixed estimate, and fixed quantile must not reuse one scalar across strata
   unless that is the specified baseline. The adapter must make the distinction
   visible in the policy and raw manifest.

## Test and acceptance matrix

| Acceptance area | Required executable evidence | Applies to |
|---|---|---|
| Registry identity | Reject unknown IDs/aliases; raw rows contain namespace, protocol ID, production/effective method, config hash | all |
| Hard-filter equivalence | Property tests feed identical candidate snapshots and assert identical pre-router feasibility/reason codes | all, including oracles |
| Information-set enforcement | Adversarial test removes oracle mount/credential; denies future usage and unselected score access; records allowed reads | all non-oracles |
| Selected feedback | One authorized selected score per effective dispatch; replay is idempotent; no unselected vector in router logs/artifacts | all online methods |
| Matched stream | Request/arrival/delay/fault/policy/price hashes identical across compared non-oracle methods; only method-internal seed differs where frozen | all contrasts |
| Determinism | Same source/config/input/seed reproduces decisions and lifecycle/raw hashes, excluding declared wall-clock performance fields | all trace methods |
| Parameter provenance | Every fitted value points to training/calibration/development access and a pre-test artifact hash; no frozen-test read | all fitted methods |
| Budget-state sensitivity | Metamorphic tests change settled/outstanding/carried separately and assert the method-specific behavior | eleven budget methods |
| Atomic lifecycle | Concurrent check-and-reserve, duplicate events, late/final settlement, rollover, corrections, and principal isolation satisfy the method's documented ledger | all except observational `no_budget`; its event identity still applies |
| Reservation arithmetic | Component-wise integer recomputation equals engine/raw reservation; caps and clipping are explicit | reservation methods |
| Adaptive validity | Wrong/stale/low-support/drifted price/cap/feature/cohort artifact cannot advertise calibrated risk and follows frozen fallback | `adaptive_quantile`, both `gov_ar` components |
| Cohort/risk binding | Opportunity mismatch, post-outcome registration, weight/risk mismatch, heterogeneous target mismatch, or unsigned cohort fails closed | `budget_admission/gov_ar` |
| Router conformance | Golden fixtures compare frozen equations or pinned artifact output, tie-breaks, and hard-filter wrapper | published/faithful routers |
| Oracle separation | Oracle starts only after non-oracle decision commitment; oracle receipt cannot enter training/online state; paired route identity is checked | both oracle methods |
| Published artifact provenance | Source URL, revision, archive hash, patch set, environment lock, license, and conformance report exist | R01/R04/R15/R16/R55/R56/R60/R62/R63/R71 adaptations |
| Factorial attribution | Liability contrasts reuse route-decision hash; routing contrasts reuse admission-policy hash; joint results are labeled joint | E1/E2/E3/E7 |
| Raw recomputation | Gate recomputes decisions, counts, holds, actuals, overshoot estimands, and invariant failures from lifecycle rows rather than manifest claims | all final runs |

No baseline is `validated` until its registry blockers are empty, every required
test has a hash-bound passing report, and an independent reviewer confirms that
the implementation consumed only the specified information set. Frozen-test
execution before those conditions invalidates the affected cell.
