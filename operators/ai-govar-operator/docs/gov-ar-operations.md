# GOV-AR admission operations

The production admission deployment requires PostgreSQL. The in-memory ledger is
explicitly development-only and the chart enforces a single replica in that
mode.

## Durable reconciliation

Every production admission replica runs an idempotent expiration reconciliation
pass. PostgreSQL row locks, transition preconditions, deterministic event IDs,
and inbox uniqueness provide at-least-once safety when replicas overlap or a
process restarts. The worker never releases a potentially billable dispatched
request. A reserved request is released only while its dispatch outbox is still
pending; ambiguous dispatched requests retain their residual liability and enter
reconciliation.

The Helm values `govArAdmission.reconciliation.interval` and
`govArAdmission.reconciliation.queryLimit` control the pass interval and the
bounded pending-record observation scan. Valid runtime bounds are one second to
one hour and 1 to 10,000 rows respectively. Invalid environment overrides are
ignored in favor of conservative defaults.

## Metrics

`GET /metrics` exposes the standard Go/process collectors plus these GOV-AR
collectors:

| Metric | Meaning |
|---|---|
| Metric | Labels | Meaning |
|---|---|---|
| `govar_http_requests_total` | `endpoint`, `method`, `status_class` | Requests by bounded endpoint, method, and status class. |
| `govar_http_request_duration_seconds` | `endpoint`, `method` | Decision-path latency histogram. |
| `govar_admission_decisions_total` | `decision`, `reason`, `method` | Admission decisions. `decision` is **uppercase**: `ADMIT`, `QUEUE`, `REJECT`, `ABSTAIN`, `REQUIRE_APPROVAL`. |
| `govar_transition_total` | `from`, `to`, `reason` | Effective ledger state transitions, recorded **only after transaction commit**. |
| `govar_decision_duration_seconds` | — | End-to-end admission decision duration. |
| `govar_transaction_duration_seconds` | — | Duration of committed GOV-AR database transactions. |
| `govar_settlement_delay_seconds` | — | Delay from provider completion to committed settlement. |
| `govar_worker_claims_total` | `kind`, `result` | Durable worker claims by work kind and result. |
| `govar_worker_backlog` | `kind`, `state` | Durable worker backlog by work kind and state. |
| `govar_worker_oldest_age_seconds` | `kind` | Age of the oldest durable worker item. |
| `govar_worker_heartbeat_age_seconds` | `kind` | Age of the last successful worker heartbeat. |
| `govar_reserved_micros` / `govar_settled_micros` | tenant profile | Committed reserved / settled monetary snapshots. |
| `govar_outstanding_liability_micros` / `govar_carried_debt_micros` | tenant profile | Committed liability and carried-debt snapshots. |
| `govar_calibration_support` / `govar_calibration_coverage_ppb` | policy profile, detector | Calibration support and empirical coverage. |
| `govar_drift_detected` / `govar_conservative_mode` | policy profile | Drift detection and conservative-mode state. |
| `govar_arithmetic_overflow_total` / `govar_bound_violation_total` | operation / basis | Rejected overflows and reservation-bound violations. |
| `govar_audit_verification_total` | — | Append-only audit-chain verification outcomes. |

> The `govar_http_*` family is registered by the admission service itself
> (`cmd/gov-ar-admission/metrics.go`); the rest come from the shared
> `internal/govarobservability` registry. There is **no** `govar_ledger_transitions_total`
> nor any `govar_reconciliation_*` metric — durable-worker progress is observed through
> the `govar_worker_*` family above.

Metric labels never contain tenant, workload, request, reservation, trace, or
prompt identity. Per-request investigation must use the protected ledger/audit
path rather than Prometheus labels.

## Traces

Production rendering requires `govArAdmission.tracing.enabled=true` and an
OTLP/HTTP collector endpoint. The service uses W3C Trace Context, continues a
trusted downstream `traceparent` through Envoy `ext_proc`, and injects that
context into the signed internal admission, dispatch, settlement, and
cancellation calls. The process may extract W3C Baggage on trusted internal
requests, but `ext_proc` deliberately forwards no arbitrary baggage to a model
provider. Spans record bounded HTTP routes, status, decisions,
reason codes, and the immutable route-snapshot digest; prompts and tenant,
workload, request, or reservation identifiers are not span attributes. The
admission response returns the actual OpenTelemetry trace ID when a valid span
context exists.

`govArAdmission.tracing.sampleRatio` accepts a value in `(0,1]`. The Article 3
measured path uses `1` so attempted and completed observations can be reconciled;
lower operational sampling rates must not be used to infer request counts.

Health and readiness remain separate: `/healthz` reports process liveness and
`/readyz` verifies that the ledger can serve transactions. Repeated worker errors
are visible in metrics and logs, while ambiguous holds remain charged.
