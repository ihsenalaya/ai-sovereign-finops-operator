# Operator and GOV-AR risk register

- **R01 (critical)** — Same request can be settled twice with distinct settlement IDs. Impact: budget corruption. Required control: unique effective settlement plus event idempotency, concurrency regression.
- **R02 (critical)** — Proxy settles actual_cost=0. Impact: all live accounting invalid. Required control: price provider usage using immutable pricing snapshot.
- **R03 (critical)** — Admission and settlement are unauthenticated; tenant/policy are caller-controlled. Impact: cross-tenant leakage/bypass. Required control: authenticated workload UID binding and authorization.
- **R04 (critical)** — HTTPS CONNECT and webhook fail-open bypass admission. Impact: unenforced governed path. Required control: native Envoy-compatible synchronous path and fail-closed policy.
- **R05 (major)** — No dispatch/expiry/late-settle/window lifecycle. Impact: stuck or prematurely released liability. Required control: explicit state machine, worker, compensation.
- **R06 (major)** — DOUBLE PRECISION money and weak DB constraints. Impact: rounding/negative/inconsistent ledger. Required control: integer minor units or decimal numeric, constraints, balance tests.
- **R07 (major)** — Readiness ignores PostgreSQL/Kubernetes and production falls back to memory. Impact: silent non-durable operation. Required control: fail readiness/startup when durable dependencies missing.
- **R08 (major)** — Feasibility ignores readiness, quality, approval, sovereignty and routability. Impact: policy-ineligible route. Required control: hard source-of-truth filter with reason codes.
- **R09 (major)** — Model resource name may not match Envoy model name. Impact: failed or wrong route. Required control: validated deployment identity mapping.
- **R10 (major)** — Aggregate Prometheus counters are treated as budget windows. Impact: incorrect daily/weekly/monthly spend. Required control: request ledger windows plus reconciler.
- **R11 (major)** — Release workflow omits GOV-AR, verifier and node-agent images. Impact: unreproducible chart. Required control: build/push all referenced images and resolve digests.
- **R12 (major)** — No real PostgreSQL/gateway/race/fault/upgrade tests. Impact: measured-path bugs. Required control: complete D/E test matrix before pilot.
