# Operator Integration Progress

## Summary

Article 3 now includes a first operator-executable GOV-AR integration slice
beyond the standalone scaffold.

## Shared integration code added

- `operateur/internal/govar/snapshot.go`
  - builds a GOV-AR policy snapshot from `AIBudgetPolicy` and `AIRoutingPolicy`
  - extracts budget scope, fallback, objective, and routing guardrails
  - derives hard-filtered candidate model sets from `AIModel` and `AIProvider`
  - enforces request target matching, sensitive-data gating, and zone filtering

- `operateur/internal/govar/snapshot_test.go`
  - verifies governance filtering by residency and sensitive-data eligibility
  - verifies budget and routing guardrail extraction from existing CRDs

- `operateur/internal/govar/engine.go`
  - provides a minimal GOV-AR admission engine with:
    - `ADMIT`, `QUEUE`, `REJECT`, `ABSTAIN`, `REQUIRE_APPROVAL`
    - stable reason codes
    - in-memory request reservation tracking
    - idempotent settlement detection
    - tenant liability reporting

- `operateur/internal/govar/engine_test.go`
  - verifies admit, queue, settlement, and liability behaviour

- `operateur/internal/govar/postgres_engine.go`
  - adds an optional PostgreSQL-backed ledger
  - initializes GOV-AR tables automatically
  - persists reservations, settlements, tenant liability, and active counts
  - keeps duplicate settlement detection transactional

- `operateur/internal/govar/postgres_engine_test.go`
  - verifies PostgreSQL engine bootstrap validation

## Research-path integration improved

- `article3/src/admission/decision.go`
  - adds stable `ReasonCode` values for admission outcomes
  - keeps `admit`, `queue`, `reject`, and `abstain` decisions traceable
  - moves the scaffold closer to the prompt requirement for documented stable
    decision reasons

## Service and Helm integration added

- `operateur/cmd/gov-ar-admission/main.go`
  - exposes:
    - `POST /v1/admit`
    - `POST /v1/settle`
    - `POST /v1/cancel`
    - `GET /v1/liability/{tenant}`
    - `GET /healthz`
    - `GET /readyz`
    - `GET /metrics`
  - reads `AIBudgetPolicy`, `AIRoutingPolicy`, `AIModel`, and `AIProvider`
    through the Kubernetes API

- `operateur/Dockerfile.gov-ar-admission`
  - builds the service as a separate non-root distroless image

- `operateur/charts/ai-sovereign-finops-operator/templates/gov-ar-admission-*.yaml`
  - adds an optional Helm-managed Deployment and Service
  - controlled by `values.yaml` under `govArAdmission.enabled`

- `article3/infra/postgres/`
  - provides Docker Compose bootstrap and schema initialization for the
    persistent GOV-AR ledger

- `operateur/internal/sidecarproxy/proxy.go`
  - now supports optional GOV-AR admission hooks from the live HTTP proxy path
  - calls `POST /v1/admit` before upstream request dispatch
  - calls `POST /v1/settle` after successful responses with parsed usage fields
  - calls `POST /v1/cancel` on upstream transport or HTTP failure

- `operateur/internal/sidecarproxy/proxy_test.go`
  - verifies admit plus settle flow
  - verifies cancel on upstream failure

- `operateur/cmd/header-proxy/main.go`
  - exposes environment-based configuration for the GOV-AR hook path

## Current boundary

This is still not full controller-path integration of GOV-AR into the final
live operator admission path. The current progress should be read as:

- shared CRD-to-admission translation: implemented
- standalone GOV-AR decision kernel: implemented
- controller-managed `gov-ar-admission` service in the main operator chart:
  implemented as an optional component
- transactional PostgreSQL-backed ledger: implemented as an optional backend
- gateway-proxy-triggered live admit plus settle plus cancel path:
  implemented through the `header-proxy`
- Envoy-native or ext_proc-native final production path:
  still pending
