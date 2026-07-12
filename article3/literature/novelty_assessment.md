# GOV-AR Literature and Novelty Assessment

Verification date: **2026-07-12**. This assessment is based on primary publisher/proceedings records, accepted OpenReview records, clearly labelled arXiv preprints, and explicitly labelled industry implementations/product documentation. It does not treat an arXiv DOI, a whitepaper, product documentation, or an engineering article as evidence of peer review.

## Review scope and disposition

The review retained 57 relevant works. Forty-nine are peer-reviewed primary research or peer-reviewed journal/conference articles (86.0%); four are explicitly labelled preprints, RouterBench is explicitly labelled as workshop evidence, and three are explicitly labelled industry/whitepaper evidence. The retained set covers:

- budget-constrained, cost-aware, online, and adaptive LLM routing;
- multi-tenant scheduling, isolation, fairness, and capacity constraints;
- delayed feedback and resource-constrained bandits;
- reserve/refund accounting, escrow transactions, duplicate events, and transactional streams;
- output-length prediction, heavy tails, point and distribution-aware scheduling;
- chance constraints, conformal routing, conformal risk control, and drift adaptation;
- router control-plane integrity and adversarial route manipulation;
- serving-system implementation and performance context.
- Kubernetes operator/controller reliability testing and transaction availability limits.

`search_log.csv` records database searches and backward/forward chasing. The earlier saturation claim was invalidated when later searches found RACER, CONCUR, Selective Deferred Routing, Solo.io quota-management, Keel budget envelopes, and AgentBudget. After adding those works and narrowing the contribution, post-update passes S24 and S25 each found `material_new_work=false`. The search therefore stops only at those two post-update passes.

## Novelty gate conclusion

The broad proposition “a budget-aware, adaptive, risk-controlled LLM router” is **not novel**. Nor are joint model/output-budget selection, output-length prediction, conformal routing, multi-tenant fairness, drift adaptation, escrow reservation, idempotent settlement, hard policy filtering, pre-dispatch monetary reservation, hierarchical budget isolation, PostgreSQL-backed gateway enforcement, provider-usage reconciliation, or approval workflows individually novel.

The defensible paper nucleus is narrower:

> A formal and empirical study of how distribution-aware monetary reservations behave across **current concurrent unresolved liabilities** when output cost is uncertain and usage events are delayed, duplicate, missing, late, or reordered, with conservative state semantics that never interpret missing telemetry as released liability.

R55 and R56 invalidate the earlier systems-integration novelty boundary: public systems already implement gateway-side reserve-before-dispatch, outstanding pending spend, hierarchical isolation, actual-cost reconciliation, prices, periods, approvals, and PostgreSQL. The retained sources did not demonstrate calibrated monetary tail allocation together with conservative late/missing-event liability semantics and a matched risk--utilization/fault evaluation, but this is only an absence-of-found-evidence statement. It is not proof of priority and not evidence of a new algorithm. The default contribution is therefore a narrow problem/metric formulation and a comparative systems measurement study. Optimized risk allocation remains known chance-constraint machinery unless a later derivation establishes otherwise; no algorithmic novelty is approved.

## Closest prior work

| Rank | Work | Exact overlap | Boundary relative to GOV-AR |
|---|---|---|---|
| 1 | R55, Solo.io quota-management (industry reference) | Trusted gateway identity, hierarchical atomic PostgreSQL reservations, pending spend, provider-usage settlement, prices, periods, approvals, metrics, orphan cleanup | Static estimate/multiplier; no calibrated tail allocation or joint quality routing; audited expiry does not preserve conservative late liability, while a possible concurrent-settlement double-charge race is a source inference pending a faithful pinned-code test |
| 2 | R56, Keel budget envelopes (product documentation) | `remaining = total - reserved - spent`, integer microdollars, reserve before provider dispatch, reconcile actual-minus-locked afterward | No public calibration, concurrent risk allocation, joint routing, fault semantics, formal model, or comparative evidence identified |
| 3 | R01, *Token Budgets* (preprint) | Pre-flight reservation, refund/reconciliation, cap arithmetic, concurrency/delegation races, adaptive estimation, lightweight formal checks | Explicitly single-process; no model routing or tenant-window ledger; canceled-stream usage and hidden tokens remain open |
| 4 | R52/R06/R07, RACER, conformal LLM routing, and LEC | Finite-sample or selection-conditioned routing-risk control and abstention | Risk concerns answer error/model-set inclusion, not aggregate outstanding monetary liability |
| 5 | R02/R04/R05/R08/R21/R53, constrained online routers | Dollar/compute pacing, partial feedback, dynamic strategy addition, constrained deployment, joint model/prompt actions | Costs are action estimates, compute budgets, or realized sequential feedback; no conservative faulty-event liability state |
| 6 | R03, *R2-Router* (ICML 2026) | Joint model and output-length-budget choice | Controls requested output length rather than reserving and settling a priced stochastic liability |
| 7 | R57, AgentBudget (whitepaper) | Two-phase pre-call estimation/post-call reconciliation, nested budgets, loop circuit breaker, live calls | In-process session boundary; average estimate; no shared pending-reservation ledger, tenants, delayed events, or fault semantics |
| 8 | R30/R10, VTC and H-MAS | Multi-tenant fairness, burst/drift response, QoS isolation | Allocate GPU service, not tenant financial exposure across providers |
| 9 | R45/R46/R49, escrow, transactional streams, and HAT | Reservation, recovery, duplicate/reordered events, transaction/availability limits | General database machinery; not probabilistic LLM output cost or governance-aware admission |

## Answers to the six novelty questions

### 1. Which exact element is new?

No individual algorithmic or integration element is established as new. The only candidate scientific boundary is the evaluated formulation that simultaneously distinguishes request under-reservation, fixed-cohort liability exceedance, selected outstanding-set exceedance, and tenant budget-window overshoot while testing distribution-aware reservations under faulty settlement. Risk allocation is treated as known chance-constraint machinery, and reserve--dispatch--reconcile is established industry practice by R55--R57. The paper may state only that the retained sources did not report this exact metric/fault evaluation; it will make no “first” claim.

### 2. Which elements are known individually?

- Cost/quality routing and cascades: R14–R23.
- Online/bandit and budget-constrained routing: R02, R04, R05, R08, R09, R21, R25.
- Joint model/output or test-time budget: R03, R18, R21.
- Output-length point and distribution prediction: R26–R29.
- Risk-controlled/conformal routing: R06, R07, R41–R43.
- Drift/non-stationary online learning: R02, R09, R10, R39, R42, R43.
- Multi-tenant fairness and resource isolation: R10, R30, R47.
- Escrow reservation and transaction recovery: R45, R46.
- LLM/agent pre-dispatch budget reservation and reconciliation: R55--R57.
- Hierarchical gateway budget isolation, pricing, periods, approvals, and PostgreSQL pending-spend accounting: R55.
- Integer monetary budget envelopes with explicit in-flight reserved state: R56.
- Router integrity attacks: R11.
- Dynamic/distributed routing and model availability: R08, R09, R24.

### 3. What is the closest prior work?

R55 is the closest implemented gateway system, R56 is the closest documented budget-envelope abstraction, and R01 is the closest adaptive reservation/concurrency research preprint. R52 is the closest calibrated routing-risk method; CONCUR, ParetoBandit, PILOT, and StageRoute (R53/R02/R04/R08) are the closest constrained/dynamic routers; R03 is the closest joint model/output-budget choice; Selective Deferred Routing (R54) is the closest local/remote cost-quality deferral method; and AgentBudget (R57) is the closest in-process two-phase nested-agent budget implementation. GOV-AR must compare against the R55 fixed-estimate/multiplier design and an R01-style adaptive estimator with a correctly locked transactional counter, not only against deliberately weak settled-spend or mean baselines.

### 4. Is the contribution more than systems integration?

No. R55 and R56 demonstrate that the gateway, PostgreSQL, pending-liability, reserve/settle, hierarchy, pricing, and approval composition is already implemented practice. The work becomes a defensible scientific contribution only as a preregistered comparative measurement study if probability statements remain tied to their exact events/assumptions, strong practical and academic comparators are used, and the fault campaign exposes reproducible differences in conservative late/missing-event behavior. If fixed/adaptive reservation or the R55-style design matches the full method at matched risk, the result is a null/negative systems study, not a new routing method.

### 5. What experiment would falsify the claimed advantage?

The central advantage is falsified if an adaptive quantile reservation plus cheapest governance-compliant router, an R01-style adaptive estimator, or an R55-style atomic fixed-estimate/multiplier envelope matches GOV-AR's utilization, served quality, and refusal rate at the same empirical overshoot risk under matched concurrent delayed-settlement streams.

Additional required falsifiers are:

- settled-spend-only control does not worsen with concurrency × settlement delay;
- removing risk allocation does not change the frontier;
- calibration shift defeats the advertised risk target before fallback becomes visible;
- policy-ineligible models can be selected through adversarial router manipulation;
- duplicate/lost/late settlement causes double release, negative liability, or cross-tenant leakage;
- the full method does not outperform fixed/adaptive quantile reservation in heavy-tail, drift, or noisy-neighbor cases.

Negative or null results must narrow the claim rather than trigger test-set tuning.

### 6. Which claim must be removed or narrowed?

Remove claims of being the first:

- cost-aware, budget-constrained, online, adaptive, or bandit LLM router;
- conformal or risk-controlled LLM router;
- joint model/token-budget or test-time-compute router;
- output-length predictor or heavy-tail-aware scheduler;
- distributed or dynamically reconfigurable router;
- multi-tenant fair LLM scheduler;
- reservation/refund ledger or idempotent transactional processor.
- gateway-side reserve-before-dispatch and provider-usage settlement;
- hierarchical tenant/team budget isolation, approvals, price versioning, periods, PostgreSQL pending spend, or budget-envelope arithmetic.

Do not describe nonzero-risk mode as deterministically enforcing a hard budget. Deterministic budget safety is supportable only in strict mode under explicit price, tokenizer, hidden-token, dispatch, and provider-cap assumptions.

## Theory boundary

The default probabilistic result is an unconditional union bound for a prespecified fixed cohort of admitted requests: if every cohort member has a valid tail statement for the selected deployment/configuration and the assigned tail probabilities sum to at most the cohort target, the probability that at least one member exceeds its reservation is at most that sum. This does not bound the probability of ever overshooting a budget window and does not automatically apply after conditioning on which requests remain outstanding.

A fixed-time result for the selected outstanding set would additionally require a stated non-informative-delay/censoring assumption or a selection-valid construction. Ordinary empirical or marginal conformal quantiles do not provide exact distribution-free conditional coverage. Unconditional offline quantiles can also lose coverage after adaptive model selection. Missing telemetry cannot justify releasing a reservation: the record remains charged or settles to an explicitly conservative terminal amount. Provider-billed hidden reasoning/tool tokens, retries, fallbacks, or canceled streams can violate a client-side strict bound unless independently capped and included in the pricing/version assumptions.

## Contribution wording approved by this audit

Subject to experimental validation, the manuscript may make only two scientific contributions:

1. **Problem and safety formulation:** distinguish settled spend, unresolved estimated-cost liability, request under-reservation, fixed-cohort and selected-outstanding-set exceedance, and tenant budget-window overshoot; state only assumption-conditional cohort bounds and explicit ledger invariants.
2. **Comparative measurement evidence:** quantify risk--utilization, isolation, drift, missing/late/duplicate-event behavior, and local overhead against R55/R56-style fixed envelopes, R01-style adaptive estimation, fixed/adaptive quantiles, strong routers, and strict reservation on matched streams and bounded live providers.

The gateway/operator implementation is the experimental vehicle and reproducibility artifact, not an independent scientific novelty claim. Algorithmic risk allocation is not approved. Kubernetes CRDs, PostgreSQL transactions, Envoy/agentgateway integration, reservation/reconciliation, conformal calibration, and standard routing objectives remain known ingredients.

## Required baseline mapping

- Expected-cost/online routers: R02, R04, R05, R08, R09, R21.
- Strong quality routers: R14–R20 and R23.
- Risk-controlled routers: R06 and R07.
- Multi-tenant fairness/QoS: R10 and R30.
- Reservation/counter comparator: R01 plus strict max, mean, margin, fixed quantile, and adaptive quantile.
- Practical envelope comparator: R55/R56 fixed estimate plus safety multiplier with atomic pending-spend accounting and expiry behavior.
- Oracle and benchmark sanity: R12/R13 with frozen counterfactual outcomes hidden from online methods.

The claims-to-evidence table must point to matched-stream comparisons against these categories; omission of R01, R02, R03, R04, R06, R08, R12, R21, R30, R52, R53, or an R55/R56-style practical envelope would materially weaken the novelty argument.
