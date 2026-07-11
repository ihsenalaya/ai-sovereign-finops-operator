# Operator Architecture Audit

## Executive Summary

The current operator is a Kubernetes control plane for AI FinOps, sovereignty, governance-aware routing, and confidential attestation. It is architected around:

- declarative CRDs in `operateur/api/v1alpha1`
- reconcilers in `operateur/internal/controller`
- pure business engines in `operateur/internal/*engine`
- telemetry collectors decoupled from business logic
- optional data-plane actuation against Envoy AI Gateway routes

This separation is favorable for GOV-AR because a new admission-and-reservation method can be introduced as a new pure decision layer plus a ledger-backed controller path, without rewriting the whole operator.

## Current Telemetry Flows

The operator supports three real telemetry paths:

1. `aigw`
   - source: Envoy AI Gateway or OpenTelemetry metrics
   - expected signals: token usage and optionally latency
   - role: production-grade source for cost, quality, budget and sovereignty logic

2. `prometheus`
   - source: Prometheus queries
   - role: fallback real source for usage aggregation

3. `configmap`
   - source: preloaded usage data in Kubernetes ConfigMaps
   - role: deterministic demos and experiments

The fake collector is opt-in only and is explicitly not the default production path.

## Token Attribution and Cost Computation

- usage samples are collected per request/app/team/namespace/model depending on available labels
- provider pricing is resolved from `AIProvider` or the built-in catalog
- `costengine` computes input and output token cost in EUR-equivalent quantities
- attribution is exposed through:
  - `AIFinOpsReport.status`
  - Prometheus metrics under `ai_finops_*`
  - Markdown and JSON report payloads

## Budget Transitions

`AIBudgetPolicy` tracks spend against thresholds and assigns a phase:

- `ok`
- `warning`
- `critical`
- `hardLimit`

In enforce mode, a guarded fallback may reroute traffic to a cheaper managed model if:

- the fallback is actually cheaper
- the model is not shared beyond the target scope
- no sovereignty enforcement already conflicts
- telemetry is sufficient for guardrails

## Routing and Recommendation Logic

Current routing logic is recommendation-centric:

- `AIRoutingPolicy` computes candidate models using routing score logic
- `recommendationengine` is sovereignty-aware and cost-aware
- `AIChangeRequest` provides an approval path before live actuation
- `AIRouteOverride` provides a manual immediate override path
- `AIQualityGate` is the explicit quality safety check before switching models

This is close to a decision-control loop, but not yet a joint admission, queueing, reservation, and delayed-settlement method.

## Enforcement Logic

`AISovereigntyPolicy.enforcementMode` drives the action level:

- `reportOnly`
- `warn`
- `enforce`

In enforce mode the operator mutates gateway routes and can:

- reroute to a compliant backend
- block via the reserved nonexistent backend `aiops-blocked`

Mutations are reversible using annotations and finalizer-driven cleanup.

## Shadow AI Path

The operator also has a second sovereignty signal path independent of the gateway:

- Tetragon observes egress
- `shadowengine` classifies destination zones via `EndpointToZone`
- metrics such as `ai_finops_shadow_ai_egress` expose off-gateway usage

This is a useful substrate for GOV-AR because it already distinguishes governed and non-governed request paths.

## Granularity and Idempotence

Granularity today is mostly at:

- model
- provider
- namespace
- application
- team

Important limitation:

- gateway enforcement is currently effectively model-scoped rather than fully namespace-scoped in the data plane

Idempotence mechanisms already present:

- controller reconciliation against desired state
- status fields with `observedGeneration`
- reversible route mutations
- evidence ownership separation in the confidential track
- `AIChangeRequest` phase-based workflow

## Double-Counting and Async Risks

Main risks already visible for a future GOV-AR design:

- delayed telemetry can make budget state lag behind in-flight requests
- gateway counters may reset after restarts
- current budget logic is spend-observation-based, not reservation-ledger-based
- concurrent requests can oversubscribe a tenant budget before post-hoc settlement arrives
- governance enforcement and budget fallback can interact unless explicitly serialized

These are precisely the gaps GOV-AR should target.

## Modification Points for GOV-AR

The most promising insertion points are:

- new pure decision package under `article3/src/` and later shared operator package
- admission webhook or gateway-side decision endpoint
- tenant budget ledger reconciler or transactional store
- predictor for output-token reservation
- route-selection path adjacent to `AIRoutingPolicy`
- settlement/idempotence path adjacent to `AIFinOpsReport` and `AIBudgetPolicy`

## Audit Conclusion

The current operator already provides:

- telemetry ingestion
- pricing and catalog semantics
- routing recommendation
- policy enforcement
- quality guardrails
- auditability

It does not yet provide:

- explicit in-flight liability accounting
- atomic reserve-settle semantics
- risk-bounded admission under delayed cost feedback
- queue/reject/abstain as first-class online decisions
