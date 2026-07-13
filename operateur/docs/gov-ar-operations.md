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
| `govar_http_requests_total` | Requests by bounded endpoint, method, and status class. |
| `govar_http_request_duration_seconds` | Decision-path latency histogram. |
| `govar_admission_decisions_total` | Admission decisions by closed decision and reason. |
| `govar_ledger_transitions_total` | Dispatch, settlement, and cancellation attempts and outcomes. |
| `govar_reconciliation_runs_total` | Successful and failed worker passes. |
| `govar_reconciliation_records_total` | Records changed by expiration reconciliation. |
| `govar_reconciliation_pending_records` | Pending rows in the bounded observation scan. |
| `govar_reconciliation_last_success_unixtime` | Last successful pass time. |

Metric labels never contain tenant, workload, request, reservation, trace, or
prompt identity. Per-request investigation must use the protected ledger/audit
path rather than Prometheus labels.

## Traces

Production rendering requires `govArAdmission.tracing.enabled=true` and an
OTLP/HTTP collector endpoint. The service uses W3C Trace Context and Baggage,
continues a trusted downstream `traceparent` through Envoy `ext_proc`, and
injects that context into the signed internal admission, dispatch, settlement,
and cancellation calls. Spans record bounded HTTP routes, status, decisions,
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
