# Gap Analysis

## What the Existing Operator Already Solves

- telemetry ingestion from real sources
- cataloged provider and model metadata
- cost attribution and FinOps reporting
- budget tracking with threshold phases
- governance-aware model recommendation
- gateway reroute and block enforcement
- quality gate before route changes
- human approval workflow for route mutation
- shadow AI detection outside the main gateway path
- confidential attestation and placement track

## What GOV-AR Still Needs

GOV-AR requires capabilities that are not first-class in the current codebase:

1. Online admission decisions
   - current operator is mostly post-hoc reporting and policy enforcement
   - no joint `admit / queue / reject / abstain` decision engine today

2. In-flight financial liability
   - current budgets are based on observed spend
   - no explicit reservation per in-flight request

3. Delayed settlement handling
   - telemetry delay is acknowledged as a limitation
   - there is no reserve-settle ledger for unknown output token cost

4. Multi-tenant risk budgeting
   - policy targets are tenant-like, but there is no probabilistic risk allocation layer

5. Atomic transaction semantics
   - current reconciler model is idempotent
   - it is not yet a transactional reservation and settlement pipeline

6. Queueing as a first-class action
   - current actuation choices focus on reroute, block, and approved change
   - queueing is not a native CRD-level outcome

## Architectural Implication

GOV-AR should reuse the current operator as the enforcement and observability substrate, while adding:

- a budget ledger
- an output-token predictor
- an admission controller or equivalent front-door decision point
- a delayed-settlement reconciliation path
- a risk-bounded allocator across tenants

## Scientific Positioning

This supports the target framing for Article 3:

- not "a paper about all operator features"
- but "a new scientific method implemented on top of the operator"
