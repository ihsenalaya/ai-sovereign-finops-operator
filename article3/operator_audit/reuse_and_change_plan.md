# Reuse and change plan

## Reuse after regression testing

- provider/model/budget/routing/sovereignty/quality/change-request CRDs and generated schemas;
- catalog resolution and Kubernetes quantity representations;
- pure cost/sovereignty engines and Prometheus registration patterns;
- Envoy metric parser for aggregate reconciliation, not request settlement;
- route mutation logic after fixing controller call sites;
- envtest harness, webhook framework, chart security contexts, and generated CRDs.

## Replace

- in-memory and PostgreSQL GOV-AR ledgers/schema;
- sidecar pricing/settlement behavior and unauthenticated API surface;
- candidate feasibility and name-to-route resolution;
- budget-window accounting and price-version handling;
- queue/approval/readiness/expiry semantics;
- Article 3 experiment engine, raw schema, statistics, and release declarations.

## Extend

- deployment routability/availability, provider readiness, quality-gate linkage, policy versioning;
- authenticated workload UID/tenant identity and authorization;
- atomic event-sourced reserve–dispatch–settle/cancel/expire/late-settle state;
- Envoy-compatible synchronous admission with retries and structured traces;
- migrations, PostgreSQL packaging, NetworkPolicy, least-privilege RBAC, metrics and diagnostics;
- immutable image/chart CI and clean install/upgrade/rollback/uninstall validation.
