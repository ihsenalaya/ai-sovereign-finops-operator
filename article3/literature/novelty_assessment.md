# GOV-AR Literature and Novelty Assessment

Verification date: **2026-07-13**. This assessment is based on primary publisher/proceedings records, accepted OpenReview records, clearly labelled arXiv preprints, and explicitly labelled industry implementations/product documentation. It does not treat an arXiv DOI, a whitepaper, product documentation, or an engineering article as evidence of peer review.

## Review scope and disposition

The review retained 77 relevant works. Sixty-five are peer-reviewed primary research or peer-reviewed journal/conference articles (84.4%); seven are explicitly labelled preprints, RouterBench is explicitly labelled as workshop evidence, and three industry/product documents plus one whitepaper are explicitly non-peer-reviewed. The retained set covers:

- budget-constrained, cost-aware, online, and adaptive LLM routing;
- multi-tenant scheduling, isolation, fairness, and capacity constraints;
- delayed feedback and resource-constrained bandits;
- reserve/refund accounting, escrow transactions, duplicate events, and transactional streams;
- output-length prediction, heavy tails, point and distribution-aware scheduling;
- chance constraints, conformal routing, conformal risk control, and drift adaptation;
- router control-plane integrity and adversarial route manipulation;
- full-matrix benchmarks, selected-model observational feedback, sparse feedback, and leakage prevention;
- multi-turn/global-budget routing, switching costs, and budget-bankruptcy behavior;
- serving-system implementation and performance context;
- Kubernetes operator/controller reliability testing and transaction availability limits.

`search_log.csv` records database searches and backward/forward chasing. The earlier saturation claim was invalidated twice: first by RACER, CONCUR, Selective Deferred Routing, Solo.io quota-management, Keel, and AgentBudget, and again on 13 July by RouterEval, QUARTZ, MESS+, MTRouter, SLARouter, CSCR, causal/observational routing, and other 2025--2026 proceedings work. After incorporating and resolving those sources, expanded exact-liability pass S29 and anchor/reference-chase pass S30 each found `material_new_work=false`. Saturation is claimed only for those two post-refresh passes and only through 13 July 2026.

## Novelty gate conclusion

The broad proposition “a budget-aware, adaptive, risk-controlled LLM router” is **not novel**. Nor are joint model/output-budget selection, output-length or cost prediction, quantile-aware request-cost admission, conformal routing, multi-turn/global-budget state, cost-optimal online routing, sparse or selected-only feedback, virtual-queue SLA control, dynamic model pools, multi-tenant fairness, drift adaptation, escrow reservation, idempotent settlement, hard policy filtering, pre-dispatch monetary reservation, hierarchical budget isolation, PostgreSQL-backed gateway enforcement, provider-usage reconciliation, or approval workflows individually novel.

The defensible paper nucleus is narrower:

> A formal and empirical study of how distribution-aware monetary reservations behave across **current concurrent unresolved liabilities** when output cost is uncertain and usage events are delayed, duplicate, missing, late, or reordered, with conservative state semantics that never interpret missing telemetry as released liability.

R55 and R56 invalidate the earlier systems-integration novelty boundary: public systems already implement gateway-side reserve-before-dispatch, outstanding pending spend, hierarchical isolation, actual-cost reconciliation, prices, periods, approvals, and PostgreSQL. R62 invalidates quantile-aware admission as a novelty claim; R63/R71 invalidate online cost/SLA and virtual-queue novelty; R61/R68 invalidate generic multi-turn/global-budget novelty; R58/R65 invalidate novelty in full-matrix benchmarking or selected-model observational feedback. The retained sources did not demonstrate calibrated **monetary** tail reservations across concurrent unresolved tenant liabilities together with conservative late/missing/duplicate/reordered settlement semantics and a matched risk--utilization/fault evaluation. That is an absence-of-found-evidence statement, not proof of priority or evidence of a new algorithm. The default contribution is a narrow problem/metric formulation and comparative systems measurement study. Optimized risk allocation remains known chance-constraint machinery; no algorithmic novelty is approved.

## Closest prior work

| Rank | Work | Exact overlap | Boundary relative to GOV-AR |
|---|---|---|---|
| 1 | R55, Solo.io quota-management (industry reference) | Trusted gateway identity, hierarchical atomic PostgreSQL reservations, pending spend, provider-usage settlement, prices, periods, approvals, metrics, orphan cleanup | Static estimate/multiplier; no calibrated tail allocation or joint quality routing; audited expiry does not preserve conservative late liability, while a possible concurrent-settlement double-charge race is a source inference pending a faithful pinned-code test |
| 2 | R62, QUARTZ | Conservative conditional request-cost quantiles, backlog-aware routing, admission queueing, online residual calibration, and fairness under output-length uncertainty | Targets TTFT/service-time tail SLOs, not priced provider-token liability; no tenant money windows, reserve--settle ledger, or usage-event faults |
| 3 | R56, Keel budget envelopes (product documentation) | `remaining = total - reserved - spent`, integer microdollars, reserve before provider dispatch, reconcile actual-minus-locked afterward | No public calibration, concurrent risk allocation, joint routing, fault semantics, formal model, or comparative evidence identified |
| 4 | R71/R63, SLARouter and MESS+ | Cost-optimal online model routing, learned satisfaction probabilities, virtual queues, time-average SLA constraints, and sparse/selected feedback in R71 | Quality SLA rather than hard monetary cap; deterministic action cost; no concurrent tenant reservations, settlement ledger, or faulty usage events |
| 5 | R01, *Token Budgets* (preprint) | Pre-flight reservation, refund/reconciliation, cap arithmetic, concurrency/delegation races, adaptive estimation, lightweight formal checks | Explicitly single-process; no model routing or tenant-window ledger; canceled-stream usage and hidden tokens remain open |
| 6 | R61/R68, MTRouter and SeqRoute | Fixed global/session budget state, multi-turn routing, switching behavior, delayed gratification, and budget-bankruptcy measurement | Sequential fixed action costs, not concurrent unknown provider bills or delayed settlement; neither maintains an unresolved-liability ledger |
| 7 | R52/R06/R07, RACER, conformal LLM routing, and LEC | Finite-sample or selection-conditioned routing-risk control and abstention | Risk concerns answer error/model-set inclusion, not aggregate outstanding monetary liability |
| 8 | R02/R04/R05/R08/R21/R53/R64/R72, constrained and cost-aware routers | Dollar/compute pacing, dynamic pools, partial feedback, constrained deployment, and cost-quality frontiers | Costs are action estimates, compute budgets, or realized sequential feedback; no conservative faulty-event liability state |
| 9 | R03/R66, R2-Router and FLARE | Joint model/output-budget choice and output-length-sensitive per-query latency/cost prediction | Control or predict length/cost rather than reserving and settling a priced stochastic tenant liability |
| 10 | R58/R65, RouterEval and Causal LLM Routing | Full counterfactual outcome matrices and selected-model-only observational feedback | Evaluation/training regimes, not financial accounting; they mandate leakage isolation and a selected-feedback baseline |
| 11 | R57, AgentBudget (whitepaper) | Two-phase pre-call estimation/post-call reconciliation, nested budgets, loop circuit breaker, live calls | In-process session boundary; average estimate; no shared pending-reservation ledger, tenants, delayed events, or fault semantics |
| 12 | R30/R10/R70, VTC, H-MAS, and Bifrost | Multi-tenant isolation, burst/drift response, hierarchical budgets, provider/model allowlists, and gateway enforcement | Resource or accumulated-spend controls, not calibrated outstanding monetary exposure across providers |
| 13 | R45/R46/R49, escrow, transactional streams, and HAT | Reservation, recovery, duplicate/reordered events, transaction/availability limits | General database machinery; not probabilistic LLM output cost or governance-aware admission |

## Answers to the six novelty questions

### 1. Which exact element is new?

No individual algorithmic or integration element is established as new. The only candidate scientific boundary is the evaluated formulation that simultaneously distinguishes request under-reservation, fixed-cohort liability exceedance, selected outstanding-set exceedance, and tenant budget-window overshoot while testing **monetary** distribution-aware reservations under faulty settlement. Quantile-aware admission is known from R62, online cost/SLA routing from R63/R71, multi-turn budget state from R61/R68, and selected-only feedback from R65. Risk allocation is known chance-constraint machinery, and reserve--dispatch--reconcile is established practice by R55--R57. The paper may state only that the retained sources did not report this exact monetary metric/fault evaluation; it will make no “first” claim.

### 2. Which elements are known individually?

- Cost/quality routing and cascades: R14–R23, R59, R60, R64, R72, R75.
- Online/bandit, cost-optimal, SLA, and budget-constrained routing: R02, R04, R05, R08, R09, R21, R25, R63, R71.
- Joint model/output or test-time budget and multi-turn budget state: R03, R18, R21, R61, R68.
- Output-length point/quantile prediction and quantile-aware admission: R26–R29, R62, R66, R69.
- Risk-controlled/conformal routing: R06, R07, R41–R43.
- Drift/non-stationary online learning and dynamic model pools: R02, R09, R10, R24, R39, R42, R43, R59, R60, R64, R72, R73, R76.
- Full-matrix routing benchmarks and selected-only/causal feedback: R12, R13, R58, R65, R71, R74.
- Multi-tenant fairness, resource isolation, and hierarchical gateway governance: R10, R30, R47, R55, R70.
- Escrow reservation and transaction recovery: R45, R46.
- LLM/agent pre-dispatch budget reservation and reconciliation: R55--R57.
- Hierarchical gateway budget isolation, pricing, periods, approvals, and PostgreSQL pending-spend accounting: R55.
- Integer monetary budget envelopes with explicit in-flight reserved state: R56.
- Router integrity and expensive-route attacks: R11, R77.
- Dynamic/distributed routing and model availability: R08, R09, R24.

### 3. What is the closest prior work?

R55 is the closest implemented reserve--settle gateway, R62 is the closest peer-reviewed quantile-aware admission system, R56 is the closest documented monetary budget envelope, and R01 is the closest adaptive reservation/concurrency research preprint. R71 and R63 are the closest cost-optimal online/SLA routers; R61 and R68 are the closest fixed-budget multi-turn methods; R65 is the closest selected-only observational router; R52 is the closest calibrated routing-risk method; R03/R66 are the closest joint length/cost routing methods. ParetoBandit, PILOT, StageRoute, CONCUR, ContextualRouter, CSCR, and LLMRec (R02/R04/R08/R53/R60/R72/R64) remain strong constrained or cost-quality comparators. GOV-AR must compare against the R55 fixed-estimate/multiplier design, QUARTZ-style quantile admission, an R01-style adaptive estimator, and at least one R63/R71-style cost/SLA router where assumptions permit—not only weak settled-spend or mean baselines.

### 4. Is the contribution more than systems integration?

No. R55/R56 demonstrate that gateway, PostgreSQL, pending-liability, reserve/settle, hierarchy, pricing, and approvals are implemented practice; R62 demonstrates quantile-aware admission; R63/R71 demonstrate cost-optimal online/SLA routing; R61/R68 demonstrate multi-turn budget state. The work becomes defensible only as a preregistered comparative measurement study with event-specific probability statements, strong practical and academic comparators, leakage-controlled selected feedback, and reproducible late/missing-event differences. If fixed/adaptive reservation, QUARTZ-style quantile admission, or the R55-style envelope matches the full method at matched risk, the result is a null/negative systems study, not a new routing method.

### 5. What experiment would falsify the claimed advantage?

The central advantage is falsified if an adaptive quantile reservation plus cheapest governance-compliant router, a QUARTZ-style quantile admission policy adapted to monetary cost, an R01-style adaptive estimator, or an R55-style atomic fixed-estimate/multiplier envelope matches GOV-AR's utilization, served quality, and refusal rate at the same empirical overshoot risk under matched concurrent delayed-settlement streams.

Additional required falsifiers are:

- settled-spend-only control does not worsen with concurrency × settlement delay;
- removing risk allocation does not change the frontier;
- an R63/R71-style cost/SLA router or R60 retrieval router matches served quality and cost without the proposed joint objective;
- RouterEval full-matrix leakage or counterfactual feedback is required to obtain the reported advantage;
- calibration shift defeats the advertised risk target before fallback becomes visible;
- policy-ineligible models can be selected through adversarial router manipulation;
- duplicate/lost/late settlement causes double release, negative liability, or cross-tenant leakage;
- the full method does not outperform fixed/adaptive quantile reservation in heavy-tail, drift, or noisy-neighbor cases.

Negative or null results must narrow the claim rather than trigger test-set tuning.

### 6. Which claim must be removed or narrowed?

Remove claims of being the first:

- cost-aware, budget-constrained, online, adaptive, or bandit LLM router;
- conformal or risk-controlled LLM router;
- quantile-aware request-cost admission or queueing policy;
- cost-optimal online/SLA router, virtual-queue controller, or sparse/selected-feedback router;
- multi-turn/global-budget router or budget-bankruptcy formulation;
- joint model/token-budget or test-time-compute router;
- output-length/cost predictor or heavy-tail-aware scheduler;
- distributed or dynamically reconfigurable router;
- multi-tenant fair LLM scheduler;
- reservation/refund ledger or idempotent transactional processor.
- gateway-side reserve-before-dispatch and provider-usage settlement;
- hierarchical tenant/team budget isolation, approvals, price versioning, periods, PostgreSQL pending spend, or budget-envelope arithmetic.
- model/provider allowlists, dynamic pool support, or expensive-route adversarial threat identification.

Do not describe nonzero-risk mode as deterministically enforcing a hard budget. Deterministic budget safety is supportable only in strict mode under explicit price, tokenizer, hidden-token, dispatch, and provider-cap assumptions.

## Theory boundary

The default probabilistic result is an unconditional union bound for a prespecified fixed cohort of admitted requests: if every cohort member has a valid tail statement for the selected deployment/configuration and the assigned tail probabilities sum to at most the cohort target, the probability that at least one member exceeds its reservation is at most that sum. This does not bound the probability of ever overshooting a budget window and does not automatically apply after conditioning on which requests remain outstanding.

A fixed-time result for the selected outstanding set would additionally require a stated non-informative-delay/censoring assumption or a selection-valid construction. Ordinary empirical or marginal conformal quantiles do not provide exact distribution-free conditional coverage. Unconditional offline quantiles can also lose coverage after adaptive model selection. Missing telemetry cannot justify releasing a reservation: the record remains charged or settles to an explicitly conservative terminal amount. Provider-billed hidden reasoning/tool tokens, retries, fallbacks, or canceled streams can violate a client-side strict bound unless independently capped and included in the pricing/version assumptions.

## Contribution wording approved by this audit

Subject to experimental validation, the manuscript may make only two scientific contributions:

1. **Problem and safety formulation:** distinguish settled spend, unresolved estimated-cost liability, request under-reservation, fixed-cohort and selected-outstanding-set exceedance, and tenant budget-window overshoot; state only assumption-conditional cohort bounds and explicit ledger invariants.
2. **Comparative measurement evidence:** quantify risk--utilization, isolation, drift, missing/late/duplicate-event behavior, and local overhead against R55/R56-style fixed envelopes, R62-style quantile admission, R01-style adaptive estimation, fixed/adaptive quantiles, selected-feedback cost/SLA and strong quality routers, and strict reservation on matched streams and bounded live providers.

The gateway/operator implementation is the experimental vehicle and reproducibility artifact, not an independent scientific novelty claim. Algorithmic risk allocation is not approved. Kubernetes CRDs, PostgreSQL transactions, Envoy/agentgateway integration, reservation/reconciliation, conformal calibration, and standard routing objectives remain known ingredients.

## Required baseline mapping

- Expected-cost/online/SLA routers: R02, R04, R05, R08, R09, R21, R63, R71.
- Strong quality and dynamic-pool routers: R14–R20, R23, R59, R60, R64, R72.
- Multi-turn/global-budget comparators: R61 and R68 where the trace supports sessions.
- Selected-feedback comparators and leakage controls: R58, R65, and R71.
- Length/quantile-aware routing and admission: R03, R62, R66, and R69.
- Risk-controlled routers: R06 and R07.
- Multi-tenant fairness/QoS and practical governance hierarchy: R10, R30, and R70.
- Reservation/counter comparator: R01 plus strict max, mean, margin, fixed quantile, and adaptive quantile.
- Practical envelope comparator: R55/R56 fixed estimate plus safety multiplier with atomic pending-spend accounting and expiry behavior.
- Oracle and benchmark sanity: R12/R13/R58 with frozen counterfactual outcomes hidden from online methods and served only through a selected-feedback evaluator.

The claims-to-evidence table must point to matched-stream comparisons against these categories. Omission of R01, R02, R03, R04, R06, R08, R12/R58, R21, R30, R52, R53, R60, R62, R63/R71, R65, R72, or an R55/R56-style practical envelope would materially weaken the novelty argument unless the frozen benchmark makes that comparator inapplicable and the exclusion is documented before test evaluation.
