# Telemetry, cost, budget, and routing flow

| Stage | Source | Identity/granularity | Current limitation for GOV-AR |
|---|---|---|---|
| request decoration | `internal/sidecarproxy`, pod injector | workload headers | caller annotations are unauthenticated; HTTPS CONNECT bypasses |
| route selection | Envoy AI Gateway route | model header/backend | model resource name and `spec.modelName` can differ |
| token telemetry | Envoy `gen_ai_*` histograms | aggregate label series | no request ID, errors, exact tail latency, or settlement event |
| collection | Prometheus collectors | cumulative process counters | time-window argument is ignored |
| price calculation | `internal/costengine` | model aggregate | fixed book/FX; pricing version not propagated |
| budget control | AIBudgetPolicy controller | aggregate settled projection | in-flight liabilities absent from established path |
| recommendation | AIRoutingPolicy/FinOps engines | policy/workload aggregate | objective does not drive GOV-AR; recommendation is not admission |
| actuation | route actuator, sovereignty/budget/override controllers | route rule | override/change controllers can pass backend where model is expected |
| GOV-AR settlement | sidecar `/v1/settle` | request ID | current proxy sends zero cost and ignores settlement failures |

The Article 3 implementation must add request/workload UID identity, immutable pricing snapshot, known input usage, probabilistic/strict output liability, transactional lifecycle/event log, and aggregate reconciliation without treating aggregate Prometheus counters as request settlement.
