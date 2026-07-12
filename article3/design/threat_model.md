# GOV-AR threat and failure model

## Trust boundary

The evaluated path begins at an authenticated gateway that overwrites caller identity and ends at ledger reconciliation of provider-reported usage. The study does not establish legal compliance, provider honesty, invoice equality, or Byzantine security. Token-derived values are estimated token cost unless billing records are reconciled.

## In-scope failures

- burst concurrency that defeats settled-spend-only checks;
- unknown/heavy-tailed output cost and duration-dependent censoring;
- stale calibration, task/price/model/policy drift, and incomplete support;
- duplicate response, duplicate settlement with the same or different event ID, duplicate gateway event, and reordered events;
- lost or late settlement, timeout followed by late response, response without usage, telemetry loss, and censored latent billable cost `C_i*`;
- crash before/after reserve, dispatch, response, settlement, and acknowledgement;
- ambiguous outbox delivery, separately billable retry/fallback attempts, and provider-idempotency mismatch;
- PostgreSQL outage/latency, leader eviction, two settlers, network partition/recovery, and route rollback;
- budget-window rollover and policy/price change while requests are in flight;
- workload deletion/recreation with UID change and cross-namespace tenant collision;
- provider/model unreadiness, unroutability, quality failure, missing approval, and gateway bypass;
- hidden/cached/reasoning/tool/media tokens, cancellation charges, and any billing dimension omitted from a strict bound.

## Required controls

- gateway-authenticated workload UID/tenant binding and fail-closed governed route;
- versioned hard feasibility before optimization;
- integer-money serializable reservation with transactional outbox/inbox;
- one effective provisional ledger settlement per request/attempt, residual correction hold until authoritative finality, and monotone-versioned correction deltas;
- separately reserved billable attempts unless provider idempotency is verified;
- unresolved/quarantined liability for ambiguous delivery, timeout, expiry, missing usage, or late telemetry;
- immutable policy/pricing/billable-category snapshot;
- visible drift fallback and calibration revalidation;
- invariant recomputation and complete diagnostics for every final fault trial.

## Safety/liveness trade-off

Conservative unresolved liability can block service when telemetry never arrives. The fault campaign measures backlog, refusal, queue growth, intervention, and recovery alongside budget safety. A safe but permanently unavailable system is not described as operationally superior.

## Out of scope

- cryptographic compromise or malicious falsification by the provider;
- correctness of provider invoices not reconciled in the study;
- prompt-injection/content safety except where it changes eligibility or cost;
- general Byzantine PostgreSQL/Kubernetes behavior;
- universal cloud performance or legal sufficiency.
