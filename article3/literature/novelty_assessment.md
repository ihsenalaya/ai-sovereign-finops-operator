# GOV-AR Literature and Novelty Assessment

Verification date: **2026-07-11**. This assessment is based on primary publisher/proceedings records, accepted OpenReview records, and clearly labelled arXiv preprints. It does not treat an arXiv DOI as evidence of peer review.

## Review scope and disposition

The review retained 51 relevant works. Forty-six are peer-reviewed primary research or peer-reviewed journal/conference articles (90.2%); four are explicitly labelled preprints and RouterBench is explicitly labelled as workshop evidence. The retained set covers:

- budget-constrained, cost-aware, online, and adaptive LLM routing;
- multi-tenant scheduling, isolation, fairness, and capacity constraints;
- delayed feedback and resource-constrained bandits;
- reserve/refund accounting, escrow transactions, duplicate events, and transactional streams;
- output-length prediction, heavy tails, point and distribution-aware scheduling;
- chance constraints, conformal routing, conformal risk control, and drift adaptation;
- router control-plane integrity and adversarial route manipulation;
- serving-system implementation and performance context.
- Kubernetes operator/controller reliability testing and transaction availability limits.

`search_log.csv` records database searches, backward/forward chasing, and four explicit saturation checks. Expanded synonym passes S16 and S18 and citation-chasing passes S17 and S19 each found `material_new_work=false`. These passes added context but did not change the closest-prior-work ordering or the conclusion below.

## Novelty gate conclusion

The broad proposition “a budget-aware, adaptive, risk-controlled LLM router” is **not novel**. Nor are joint model/output-budget selection, output-length prediction, conformal routing, multi-tenant fairness, drift adaptation, escrow reservation, idempotent settlement, or hard policy filtering individually novel.

The defensible paper nucleus is narrower:

> A synchronous multi-provider admission controller that makes hard governance eligibility decisions, selects an eligible model and upper monetary reservation, accounts for the **current concurrent set of unresolved estimated-cost liabilities**, and maintains exactly-once ledger effects under delayed, duplicate, missing, late, and reordered usage events.

No retained primary source implements that complete combination. This is an absence-of-found-precedent conclusion after the documented search, not proof that no such system exists and not evidence that the composition is a new algorithm. The default contribution is therefore a problem formulation, fault-tolerant systems realization, and measurement study. Optimized risk allocation remains an experimental component whose algorithmic contribution is rejected unless it is formally distinct from classical chance-constraint allocation and survives the matched-risk falsifier.

## Closest prior work

| Rank | Work | Exact overlap | Boundary relative to GOV-AR |
|---|---|---|---|
| 1 | R01, *Token Budgets* (preprint) | Pre-flight reservation, refund/reconciliation, cap arithmetic, concurrency/delegation races, double-spend prevention, live provider tests, adaptive estimation, lightweight formal checks | Explicitly single-process; distributed multi-tenant reservation is not implemented; no model routing; missing canceled-stream usage and hidden tokens remain open |
| 2 | R02, *ParetoBandit* (preprint) | Closed-loop dollar pacing, cost/quality drift, partial feedback, model hot-swap | Long-run average cost; sequential realized-cost update; no unresolved concurrent liability, hard tenant window, or settlement ledger |
| 3 | R03, *R2-Router* (ICML 2026) | Joint model and output-length-budget choice | Controls requested output length rather than reserving a stochastic monetary liability and settling actual usage |
| 4 | R04/R08/R21, PILOT/StageRoute/TREACLE | Budget-constrained online routing, constrained deployment, joint model/prompt actions | Costs are action estimates or realized sequential feedback; no atomic reserve–settle state under concurrency |
| 5 | R06/R07, conformal LLM routing and LEC | Finite-sample or selection-conditioned routing-risk control | Risk concerns answer error, not aggregate outstanding monetary liability |
| 6 | R30/R10, VTC and H-MAS | Multi-tenant fairness, burst/drift response, QoS isolation | Allocate GPU service, not tenant financial exposure across providers |
| 7 | R45/R46, escrow and transactional streams | Long-lived reservation, recovery, duplicate/reordered events, transactional invariants | General database machinery; not probabilistic LLM output cost or governance-aware routing |

## Answers to the six novelty questions

### 1. Which exact element is new?

No individual algorithmic element is established as new. The candidate contribution boundary is the evaluated combination of:

1. pre-dispatch monetary upper-tail reservation for unknown output tokens;
2. explicit accounting of the tenant's **currently unresolved concurrent requests**, with any risk allocation treated as known chance-constraint machinery unless proven otherwise;
3. model/reservation choice after a non-bypassable governance feasibility filter; and
4. a cross-process transactional state machine that preserves the liability invariant despite delayed and faulty settlement.

The paper may say only that no retained work demonstrated this complete operational combination. It must not call the combination a new routing or risk-allocation algorithm. A “first” claim is unnecessary and will be omitted unless a later journal-specific review requires and supports it.

### 2. Which elements are known individually?

- Cost/quality routing and cascades: R14–R23.
- Online/bandit and budget-constrained routing: R02, R04, R05, R08, R09, R21, R25.
- Joint model/output or test-time budget: R03, R18, R21.
- Output-length point and distribution prediction: R26–R29.
- Risk-controlled/conformal routing: R06, R07, R41–R43.
- Drift/non-stationary online learning: R02, R09, R10, R39, R42, R43.
- Multi-tenant fairness and resource isolation: R10, R30, R47.
- Escrow reservation and transaction recovery: R45, R46.
- Router integrity attacks: R11.
- Dynamic/distributed routing and model availability: R08, R09, R24.

### 3. What is the closest prior work?

R01 is the closest on financial liability and concurrency; R02 is the closest on dollar-aware adaptive routing; R03 is the closest on joint model/output-budget choice. None alone is an adequate comparator. GOV-AR must compare against an R01-style adaptive estimator with a correctly locked runtime counter, not only against a deliberately weak settled-spend or mean baseline.

### 4. Is the contribution more than systems integration?

Not by default. It becomes a defensible scientific method contribution only if:

- the concurrent risk-allocation/reservation rule is formally distinct;
- any probability statement is restricted to the exact fixed cohort/event its assumptions support;
- it improves the measured risk–utilization frontier over fixed and adaptive quantile reservation and the R01-style counter at matched empirical risk; and
- the ledger/fault path is evaluated as a systems contribution rather than presented as new transaction theory.

If those conditions fail, the honest contribution is a systems study of failure-safe LLM financial admission, not a new routing algorithm.

### 5. What experiment would falsify the claimed advantage?

The central claim is falsified if an adaptive quantile reservation plus cheapest governance-compliant router—or an R01-style adaptive estimator with a correct transactional counter—matches GOV-AR's utilization, served quality, and refusal rate at the same empirical overshoot risk under matched concurrent delayed-settlement streams.

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

Do not describe nonzero-risk mode as deterministically enforcing a hard budget. Deterministic budget safety is supportable only in strict mode under explicit price, tokenizer, hidden-token, dispatch, and provider-cap assumptions.

## Theory boundary

The default probabilistic result is an unconditional union bound for a prespecified fixed cohort of admitted requests: if every cohort member has a valid tail statement for the selected deployment/configuration and the assigned tail probabilities sum to at most the cohort target, the probability that at least one member exceeds its reservation is at most that sum. This does not bound the probability of ever overshooting a budget window and does not automatically apply after conditioning on which requests remain outstanding.

A fixed-time result for the selected outstanding set would additionally require a stated non-informative-delay/censoring assumption or a selection-valid construction. Ordinary empirical or marginal conformal quantiles do not provide exact distribution-free conditional coverage. Unconditional offline quantiles can also lose coverage after adaptive model selection. Missing telemetry cannot justify releasing a reservation: the record remains charged or settles to an explicitly conservative terminal amount. Provider-billed hidden reasoning/tool tokens, retries, fallbacks, or canceled streams can violate a client-side strict bound unless independently capped and included in the pricing/version assumptions.

## Contribution wording approved by this audit

Subject to experimental validation, the manuscript may make at most three contributions:

1. **Problem formulation:** distinguish settled spend, unresolved estimated-cost liability, request under-reservation, cohort exceedance, and budget-window overshoot for governed multi-tenant LLM admission.
2. **Systems realization:** implement and fault-test atomic local reservation, a transactional dispatch outbox, exactly-once settlement effects, unresolved-liability carryover, and a synchronous gateway integrated with aggregate Kubernetes policy state.
3. **Measurement evidence:** quantify risk–utilization, isolation, drift, failure, and overhead trade-offs against strong reservation/routing baselines on matched streams and bounded live providers.

An algorithmic risk-allocation contribution is not approved. It may be reconsidered only after formal distinction and matched-risk evidence. Kubernetes CRDs, PostgreSQL transactions, Envoy integration, conformal calibration, and standard routing objectives remain implementation ingredients rather than independent scientific contributions.

## Required baseline mapping

- Expected-cost/online routers: R02, R04, R05, R08, R09, R21.
- Strong quality routers: R14–R20 and R23.
- Risk-controlled routers: R06 and R07.
- Multi-tenant fairness/QoS: R10 and R30.
- Reservation/counter comparator: R01 plus strict max, mean, margin, fixed quantile, and adaptive quantile.
- Oracle and benchmark sanity: R12/R13 with frozen counterfactual outcomes hidden from online methods.

The claims-to-evidence table must point to matched-stream comparisons against these categories; omission of R01, R02, R03, R04, R06, R08, R12, R21, or R30 would materially weaken the novelty argument.
