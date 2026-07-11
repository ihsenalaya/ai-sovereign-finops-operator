# Reuse Plan

## Reuse As-Is

- `internal/catalog`
  - provider pricing defaults
  - endpoint-to-zone mapping

- `internal/collectors`
  - real telemetry ingestion paths

- `internal/costengine`
  - post-settlement cost computation

- `internal/qualityengine` and `internal/qualityeval`
  - model quality safety checks

- `internal/recommendationengine`
  - recommendation structure and sovereignty-aware filtering ideas

- `internal/enforcementengine`
  - action vocabulary and enforcement reasoning patterns

- `internal/controller/gatewayactuator.go`
  - reversible route actuation path

- `AIChangeRequest` and `AIRouteOverride`
  - audited human approval and manual intervention primitives

## Reuse With Extension

- `AIBudgetPolicy`
  - extend from observed-spend budgeting to reserved-budget semantics

- `AIRoutingPolicy`
  - extend from recommendation-only scoring to online admission and route choice

- `AIFinOpsReport`
  - extend to expose GOV-AR ledger state, reservation accuracy, and delayed settlement metrics

- `AIQualityGate`
  - keep as guardrail for candidate model switches chosen by GOV-AR

## New Components Needed for Article 3

- reservation ledger
- in-flight liability tracker
- output token predictor
- risk-budget allocator
- queue manager and timeout policy
- settlement reconciler
- fault and drift injectors for experiments

## Code Placement Proposal

- early research code:
  - `article3/src/admission`
  - `article3/src/ledger`
  - `article3/src/predictor`
  - `article3/src/trace_gateway`
  - `article3/src/fault_injector`

- later shared operator integration:
  - new pure packages under `operateur/internal/`
  - new CRD or extensions only after the algorithm stabilizes
