# Article 3 decisions (append only)

## 2026-07-11 — D001 — Remote operator base

Use `origin/main` SHA `07cdd3baad26abfa7248dd69cdd507aab4be8177` as the operator base. Both remote charts and the resolvable controller image identify app version 0.5.11. Fetched tags stop at v0.5.4, and the newer local-main 0.5.17 state is not coherent across the umbrella chart/images, so no tag is claimed for the base.

## 2026-07-11 — D002 — Prior evidence quarantine

All prior Article 3 raw/processed results, figures, tables, reports, PDFs, status declarations, provenance claims, literature outputs, and operator-audit outputs were moved under `archive/codex_20260711/`. They are audit inputs only and are excluded from final analysis.

## 2026-07-11 — D003 — Provisional contribution boundary

Current prior art invalidates broad novelty claims about cost-aware routing, model/token joint routing, conformal routing, output-length prediction, drift adaptation, transactional escrow, and idempotence individually. Until the full novelty gate passes, the only candidate nucleus is predictable per-tenant risk allocation over concurrent outstanding monetary liabilities, coupled to joint eligible-model/reservation selection and a fault-tolerant reserve–dispatch–settle realization. If matched-risk experiments do not distinguish this from adaptive quantile/Khan-style reservation, the algorithmic claim will be removed and the paper reframed as a systems contribution.

## 2026-07-11 — D004 — Safety wording

Nonzero-risk mode cannot claim deterministic hard-window safety. Deterministic safety is restricted to strict reservation/provider-enforced caps under explicit assumptions. Probabilistic mode may claim only the reviewed conditional instantaneous union bound when each tail statement is conditional on pre-dispatch history and predictable allocated risks sum within the tenant target. Missing telemetry never releases liability optimistically.

## 2026-07-11 — D005 — Red-team narrowing to systems and measurement

The fresh scientific red team found that classical chance-constraint risk allocation, online uncertain allocation, and selection effects make the provisional algorithmic novelty and conditional instantaneous claim unsafe. The paper will therefore treat the following as its default contribution set: (1) precise formulation of tenant-scoped outstanding estimated-cost liabilities under delayed/failed settlement and hard eligibility, (2) a fault-tolerant synchronous reserve/outbox/dispatch/settle systems realization, and (3) a reproducible risk–utilization/failure study. Risk allocation is not claimed as novel unless a complete rule is formally distinct and beats the fixed/adaptive-quantile matched-risk falsifier.

## 2026-07-11 — D006 — Probability event and inferential unit

The design must distinguish request under-reservation, fixed-cohort aggregate liability exceedance, fixed-time selected outstanding-set exceedance, and tenant budget-window overshoot. No theorem may transfer a bound between them. The default defensible probabilistic result is a fixed-cohort union bound under explicit dispatch-time tail assumptions; conditioning on the still-outstanding set requires a non-informative-delay assumption or a separate selection-valid method. Seeds/streams, budget windows, Azure windows, and cluster recreations—not individual requests—are the independent inferential units.

## 2026-07-11 — D007 — Unresolved liability and external delivery semantics

Expiry, client disconnect, timeout, or missing telemetry does not release a potentially billable liability. It moves to an unresolved/quarantined state charged at the prespecified conservative amount until authoritative cancellation or late settlement. Window renewal carries the originating liability/debt. PostgreSQL can provide atomic reserve and exactly-once ledger effects; it cannot make external provider execution atomic. Dispatch uses an outbox/inbox protocol, provider idempotency only when verified, and separately reserved chargeable attempts otherwise.

## 2026-07-12 — D008 — Industry prior art removes the integration contribution

Post-update searching found Solo.io's public `day0ops/quota-management` reference implementation, Keel budget envelopes, and AgentBudget. They establish gateway-side trusted identity, atomic pending-spend reservation, `total - reserved - spent`, reserve-before-dispatch, provider-usage reconciliation, hierarchical isolation, periods, pricing, approvals, PostgreSQL, and two-phase agent-call enforcement as prior art. The paper will not claim this integration as novel. Its candidate scientific contribution is limited to (1) a precise liability/overshoot/fault formulation with assumption-conditional results and (2) a preregistered comparative measurement study. The operator implementation is an experimental vehicle and reproducibility artifact.

## 2026-07-12 — D009 — Fixed-cohort theorem, correction finality, and inferential hierarchy

This decision supersedes D004's “conditional instantaneous” wording and narrows D006's inferential-unit wording. The only theorem-bearing nonzero-risk construction uses a cohort and weights fixed by a common earlier sigma-field; predictable per-request tail allocations sum to the cohort target pathwise. It establishes only a fixed-cohort union bound, never an active-set or budget-window guarantee. Provisional usage retains residual hold `R-Y` until authoritative finality; versioned corrections move deltas between cost and residual, and strict mode assumes no upward correction after finality. Origin-window attribution is immutable, while unresolved/residual holds and signed carried adjustments persist in the active enforcement view until finality, repayment, or authorized budget adjustment. In trace studies, seed/stream is the primary independent block and tenant windows are nested unless the frozen design proves otherwise. Azure windows are sampling clusters whose independence must be justified, not asserted.

## 2026-07-12 — D010 — Adaptive admissions and non-reusable historical credit

This decision tightens D009 after an independent theory counterexample. The theorem is indexed by a `G`-fixed cohort of arrival opportunities, not a supposedly fixed admitted set. Admission indicator `A_i`, route, reservation, and risk are predictable at `H_i`; rejected slots have an empty under-reservation event. The pathwise slot-risk sum yields only the fixed-opportunity-cohort bound. D009's signed-credit enforcement wording is superseded: carried enforcement debt is non-negative. When an on-time provisional settlement crosses rollover, a temporary guard plus residual retains total exposure `R` until finality. A downward historical correction is an origin-window audit credit only and cannot expand any later window's admission availability. Exact correction replay is acknowledged as an idempotent no-op by immutable event payload identity, even after later correction versions.
