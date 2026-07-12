# Phase D remaining production systems design

Status: implementation contract, not evidence of implementation or test success
Source basis: recovery branch `article3-q1-recovery-20260711`, inspected at HEAD
`a355b339c785442abce4406a4318effc9f5f815f` with the uncommitted Phase D
route, approval, typed-API, and ledger repair work present on 2026-07-12. Because
those files were concurrently changing, implementations must bind their tests and
reviews to a later committed source manifest. Nothing in this document relaxes
`article3/Q1_GATE_CONTRACT.md`.

## 1. Source-bound gap statement

The following are safety or reproducibility gaps in the inspected source, not
inferences from status files:

| ID | Current source fact | Consequence | Classification |
|---|---|---|---|
| M4.1 | `internal/govar.Candidate` carries only input and output token rates, and `chooseAdmission` calls `costFromPriceMicros` with only those two rates. | A provider marked `pricing.completeness=complete` can still have configured request, cached-token, reasoning, tool, media, time, cancellation, or retry charges that are absent from the reservation. | Must fix before measured traffic. |
| M4.2 | `/v1/settle` accepts token counts and the ledger derives cost from the two rates frozen on the reservation. There is no typed per-category authoritative usage. | Settlement cannot reproduce the complete price snapshot or distinguish included from separately billed usage. | Must fix before measured traffic. |
| M4.3 | `AIProviderReconciler` marks every structurally valid object Ready; `AIModelReconciler` resolves the provider but does not produce `status.govar.verifiedOutputCap`. | `complete`, freshness, source identity, and provider-enforced output caps have no production producer/validator. | Must fix. |
| M5.1 | `reservationTokens` selects the method and estimates from `AIRoutingPolicy` annotations. Typed `spec.govar` and controller-owned `status.govar` are not the executable authority. | A stale or drifted estimate can remain advertised and annotation writers can affect monetary liability. | Must fix. |
| M5.2 | `AIRoutingPolicyReconciler` computes legacy route recommendations but does not produce calibration artifacts, support, coverage, drift, or conservative-mode status. | Adaptive and GOV-AR risk claims have no online evidence producer. | Must fix before non-strict reservations. |
| M7.1 | PostgreSQL exposes `ReconcileExpired` and `PendingReconciliation`, but `cmd/gov-ar-admission` starts no durable worker. | Expired and ambiguous records are not automatically driven to their documented safe state. | Must fix. |
| M8.1 | The admission binary exposes the default Prometheus registry only. No GOV-AR ledger, decision, worker, calibration, drift, or latency collectors are registered. | E0/E4/E5 cannot observe the measured mechanism. | Must fix for experiments and operations. |
| M8.2 | Inbox/outbox rows provide idempotence but there is no append-only transition record with a before/after state digest, policy/pricing snapshot digest, actor class, and chain verification. | Claims of immutable or tamper-evident request audit are unsupported. | Must fix or remove the claim. |
| M8.3 | There is no OpenTelemetry dependency or W3C context propagation in the GOV-AR service. `trace_id` is currently a request identifier, not a recorded distributed trace. | A structured trace claim is unsupported. | Must fix for the required tracing path; otherwise explicitly report traces unsupported and do not use them as evidence. |
| M11.1 | `internal/govarextproc/real_envoy_test.go` exercises a real pinned Envoy against a stub admission HTTP server; the Kind test harness separately runs the operator E2E suite. | No one test proves Envoy, the actual admission binary, PostgreSQL, controller-produced status, typed routes, and settlement together. | Must fix before E0 passes. |
| M12.1 | An earlier moving-tree Docker review pinned a Go 1.21.13 builder even though that toolchain line is end-of-life. During preparation of this design the dirty Dockerfile changed concurrently to a Go 1.26.5 digest while `go.mod` still declares Go 1.21; neither a digest nor this observation proves compatibility or security. | A reproducible image can still contain a vulnerable/EOL toolchain or dependencies, and a concurrently edited image has no source-bound release evidence. | Must verify on the final committed tree. |

These gaps are independent of the still-active route/approval/ledger review. The
interfaces below must be rebased onto the accepted versions of those changes.

## 2. Cross-cutting safety rules

1. All money remains signed integer micro-currency units with checked arithmetic.
   Admission rejects overflow, negative quantities, non-EUR price snapshots, or
   non-integral conversion to micros. It must never round a liability down.
2. A safety input is executable only when it is typed, controller-produced where
   specified below, current for the referenced object generation, fresh under the
   policy, and included in an immutable snapshot digest stored with the
   reservation. An annotation is never monetary or calibration authority.
3. `complete` means every category that the provider can bill for the selected
   deployment and request mode is either (a) represented with a computable upper
   quantity and authoritative settlement mapping, or (b) explicitly declared
   inapplicable by the versioned provider adapter. Unknown applicability is
   incomplete.
4. Missing/late workers, metrics, or tracing must not release a hold. Ambiguity
   retains liability and becomes visible as `UNRESOLVED`/conservative mode.
5. Background processing is at-least-once. Safety comes from transactional claims,
   immutable payload hashes, compare-and-swap transitions, and idempotent event
   IDs, never from an exactly-once delivery claim.
6. No metric label contains request ID, reservation ID, trace ID, prompt, raw
   workload UID, or unbounded error text. Per-request detail belongs in the
   protected audit table and trace backend.

## 3. M4: typed price, output-cap, and complete liability

### 3.1 Required API contract

Extend `api/v1alpha1/aiprovider_types.go` without silently changing the meaning
of existing fields:

* Replace free-form `ProviderBillableUnit` use in GOV-AR with a closed enum and a
  closed `ProviderBillableBasis`: `input_tokens`, `cached_input_tokens`,
  `output_tokens`, `reasoning_tokens`, `request`, `tool_call`, `media_unit`,
  `billable_second`, `cancellation`, and `retry_attempt`. A provider adapter must
  define whether a detail is included in a base token count; included quantities
  cannot be charged twice.
* Each category has `applicability` (`always`, `request_declared`, or
  `provider_response`), exact `priceMicrosPerUnit` or a decimal price that is
  converted upward once by the controller, `settlementUsageField`, and either an
  enforced `maximumQuantity` or a request field from which that bound is derived.
  A response-only category without a pre-dispatch upper bound makes the candidate
  infeasible; its observed mean is not a bound.
* Add controller-owned `AIProviderStatus.GOVAR.PricingSnapshot` containing
  `specGeneration`, `version`, `observedAt`, `validUntil`, `currency`,
  `completeness`, canonical `snapshotSHA256`, adapter version, and one normalized
  row per billable basis. Status Ready is false if normalization, completeness,
  uniqueness, exact conversion, source freshness, or adapter validation fails.
* Do not call an administrator-entered number provider-verified. The status must
  name its evidence mode: `provider_catalog`, `provider_api`, or
  `admin_attested`. Only the first two may support wording such as
  “provider-verified”; `admin_attested` is allowed for local tests but the article
  must label it accordingly.

Extend `api/v1alpha1/aimodel_types.go`:

* `AIModelVerifiedOutputCapStatus` additionally binds provider UID and generation,
  deployment/model version, capability adapter version, evidence mode, evidence
  digest, and `validUntil`.
* The cap producer records whether the provider demonstrably enforces the exact
  request parameter used by the selected path adapter. A catalog context limit is
  not by itself an output cap.
* Admission requires `requested max_output_tokens <= verified maxOutputTokens` and
  a fresh status bound to the same provider/deployment snapshot. It rejects or
  follows the policy's explicit conservative fallback; it never clamps the client
  request without also rewriting and proving the provider request.

Generated CRDs, deepcopy code, chart CRDs, RBAC, examples, and migration notes are
updated in the same checkpoint. Existing objects decode, but remain GOV-AR
infeasible until current evidence is produced.

### 3.2 Producers and internal interfaces

Implement normalization in a pure package so it can be independently tested:

```go
// internal/govarpricing
type SnapshotProducer interface {
    Produce(context.Context, AIProvider) (NormalizedPricingSnapshot, Evidence, error)
}
type CapabilityVerifier interface {
    VerifyOutputCap(context.Context, AIModel, AIProvider, NormalizedPricingSnapshot) (VerifiedCap, Evidence, error)
}
type Adapter interface {
    Family() string
    Normalize(AIProvider) (NormalizedPricingSnapshot, error)
    ApplicableCharges(RequestChargeContext) ([]ChargeBound, error)
    ParseUsage(ProviderUsageEnvelope) (UsageVector, Finality, error)
}
```

Exact files to change or add:

* `internal/controller/aiprovider_controller.go`: select the closed provider
  adapter, produce normalized status, make Ready conditional on evidence.
* `internal/controller/aimodel_controller.go`: verify and publish the bound cap;
  watch provider status changes and clear stale status before Ready becomes true.
* `internal/govar/snapshot.go`: replace the two scalar prices in `Candidate` with
  a canonical `PricingSnapshot` and carry the verified cap value/evidence digest.
* `internal/govar/admission_policy.go`: compute each applicable upper charge with
  checked ceiling arithmetic, sum all components, and store the component vector.
* `internal/govar/engine.go`, `postgres_engine.go`, and schema v4 migration:
  persist price snapshot hash, reserved component vector, cap evidence digest,
  request charge bounds, and the eventual actual component vector.
* `internal/govarextproc/server.go`: extract the provider-specific complete usage
  envelope, including usage details needed by the selected adapter. If a billed
  category is missing, settlement is provisional/ambiguous and retains the
  corresponding residual hold.
* `cmd/gov-ar-admission/main.go`: resolve the adapter from the immutable selected
  route snapshot; clients cannot select an adapter or submit `actual_cost_micros`.

The reservation formula is mechanically reproducible:

`R = sum_i ceil(price_micros_i * upper_quantity_i / unit_denominator_i)`.

Settlement recomputes the same vector with authoritative quantities and the
reservation's frozen snapshot, never the current provider CR. A price change
during a request therefore cannot change that request's cost. A separately
authorized correction names the predecessor usage event and uses the same frozen
snapshot unless provider billing evidence explicitly identifies a versioned
retroactive price correction.

### 3.3 M4 fail-closed matrix

| Condition | Required result |
|---|---|
| Pricing missing, partial, stale, unknown adapter, duplicate category, unknown unit/basis, negative price, arithmetic overflow | Candidate infeasible with stable pricing reason; no reservation. |
| Separately billed response-only quantity has no defensible upper bound | Candidate infeasible even in mean/quantile mode. |
| Output cap absent, stale, for another deployment, or not enforced by the path adapter | Strict fallback unavailable; return the policy's non-dispatch fallback. |
| Provider response omits a required usage field | Provisional or unresolved settlement; retain the affected component's hold. |
| Provider reports an unlisted category | Mark adapter/snapshot incomplete, preserve the full hold, enter conservative mode, and enqueue reconciliation. |
| Retry/cancellation can be billed | Reserve its prespecified maximum attempts/cancellation charge before dispatch; no unreserved retry. |
| Actual component exceeds its bound | Settle the observed amount as debt/overshoot, emit violation evidence, and invalidate the calibration/adapter; never truncate actual cost. |

### 3.4 M4 tests

Add table, fuzz, property, PostgreSQL, and controller tests:

* `internal/govarpricing/*_test.go`: canonical digest stability; every closed
  basis; included-versus-separate reasoning/cached token semantics; ceiling and
  overflow; stale and admin-attested evidence; unknown provider response field.
* `internal/controller/aiprovider_controller_test.go` and
  `aimodel_controller_test.go`: status producer, stale clearing, provider watch,
  provider/deployment binding, Ready false for all incomplete evidence cases.
* `internal/govar/admission_policy_test.go`: component sums for input, output,
  request, cached, reasoning, tool, media, time, cancellation, and bounded retry;
  a property test verifies reservation equals the sum of persisted components.
* `internal/govar/postgres_engine_test.go`: price changes after reserve do not
  affect settlement; missing detail preserves a hold; duplicate/reordered complete
  usage is idempotent; actual-over-bound creates debt and invalidation evidence.
* Adapter golden fixtures contain only synthetic/public payloads and are hashed.

## 4. M5: calibration and drift producer

### 4.1 One executable authority

Delete executable reads of `AnnotationReservationMethod`,
`AnnotationAdaptiveTokens`, `AnnotationCalibrationSupport`, and
`AnnotationCalibrationDrift` from `internal/govar/admission_policy.go` after a
documented migration period. `routing.Spec.GOVAR` selects the method.
`routing.Status.GOVAR` is evidence, not a user override.

For `adaptive_quantile` and `govar_fixed_cohort`, admission accepts status only if
all of the following hold atomically in the policy snapshot:

* status observed generation equals metadata generation;
* artifact ref, version, immutable SHA-256, feature/schema version, price/cap
  regime, and cohort registry digest match spec and the request opportunity;
* eligible final sample support meets both calibration and revalidation minima;
* `observedAt` is not in the future and is within `maxAgeSeconds`;
* the drift detector and threshold equal spec, the evaluated window is current,
  `detected=false`, and `conservativeMode=false`;
* the advertised upper output tokens and allocated risk are those persisted in
  the immutable artifact, not recomputed from mutable status fields.

Any mismatch invokes exactly `spec.govar.drift.fallback`. A strict fallback is
allowed only when M4's cap and complete charge bounds are valid. Otherwise queue,
reject, abstain, or require approval without dispatch. A fallback response has
`allocated_risk_ppb=0`, does not claim calibrated coverage, and records the
specific invalidation reason.

### 4.2 Durable evidence producer

Add a `internal/govarcalibration` package and PostgreSQL schema v4 tables:

* `govar_usage_observations`: append-only authoritative-final usage vectors keyed
  by request/provider attempt and feature/regime digests;
* `govar_calibration_artifacts`: immutable method, support, quantile, risk,
  empirical coverage, interval, feature/schema version, split/window bounds,
  source observation digest, software digest, and artifact digest;
* `govar_drift_windows`: immutable detector inputs/output and reason;
* `govar_policy_evidence_publications`: idempotent mapping from artifact digest to
  policy UID/generation/status resource version.

The producer runs only over authoritative-final observations. Provisional,
corrected-provisional, excluded, cross-regime, and current frozen-test outcome
matrix rows are never calibration input. Deterministic ordering and a pure
`BuildArtifact` function make the artifact reproducible from exported rows.

The admission service already has PostgreSQL and Kubernetes access, so the
minimal deployment is a leader-elected worker in `cmd/gov-ar-admission`, with a
separate ServiceAccount permission limited to patching `AIRoutingPolicy/status`.
If a separate worker Deployment is used, it must use the same pinned image but a
distinct command and RBAC identity. Multiple admission replicas must not race to
publish status: claim by PostgreSQL advisory lock plus Kubernetes resourceVersion
compare-and-swap. Controller-owned Conditions use stable reasons
`CalibrationValid`, `CalibrationInsufficient`, `CalibrationStale`,
`DriftDetected`, and `ConservativeMode`.

For the article's first production implementation, `coverage-gap` is the required
detector because its semantics directly match upper-tail reservation. PSI, KS,
and ADWIN remain typed but **unsupported** until their implementations and frozen
thresholds are tested; selecting one makes non-strict admission conservative.

### 4.3 M5 tests

* Golden reconstruction: exported eligible rows reproduce the exact artifact
  digest and upper quantile on two processes.
* Leakage: development/calibration observations are accepted; frozen-test hidden
  counterfactual outcomes and non-selected model outcomes are rejected.
* Status matrix: missing, stale, future, wrong generation, wrong hash, insufficient
  support, drift, and price/cap regime changes all enter visible conservative mode.
* Coverage-gap detector: nominal, +50%, +100%, +200%, heavy-tail, and delayed
  settlement streams; assert detection and recovery delay from immutable events.
* Race: two producers publish one artifact/status mapping; a stale resourceVersion
  cannot overwrite a newer conservative state.
* Regression: fixed-cohort strict fallback advertises zero allocated risk and
  `strict_provider_cap`, never the cohort allocation.

## 5. M7: durable background workers

### 5.1 Worker set

Add `internal/govarworker` with a narrow ledger interface rather than placing
timers inside HTTP handlers:

```go
type Ledger interface {
    ClaimWork(ctx context.Context, kind string, limit int, lease time.Duration) ([]WorkItem, error)
    CompleteWork(ctx context.Context, item WorkItem, result WorkResult) error
    RenewWork(ctx context.Context, item WorkItem, lease time.Duration) error
}
```

Required worker kinds:

1. `expiry`: calls server-clock-derived reconciliation for expired pending and
   ambiguous requests. An undispatched pending outbox may be atomically canceled;
   any possibly delivered attempt retains liability and becomes unresolved.
2. `delivery-reconciliation`: resolves `CLAIMED`, ambiguous dispatch, timeout, and
   response-before-settlement records from authoritative gateway/provider events.
   Lack of evidence never means unbilled.
3. `outbox-repair`: repairs ledger-to-outbox publication/notification after a
   crash. It does not independently send a second provider request. Provider
   retries require a newly reserved, uniquely identified attempt under M4.
4. `calibration-drift`: runs the M5 producer.
5. `audit-checkpoint`: verifies and checkpoints the M8 chain; it cannot mutate
   prior events.

Use `FOR UPDATE SKIP LOCKED`, `lease_owner`, `lease_until`, attempt count, next
attempt time, and last bounded reason code. Use bounded exponential backoff with
jitter. Poison work moves to `DEAD_LETTER` while liability remains held and a
high-severity metric/Condition is raised. Readiness fails if the schema is wrong,
the DB cannot be queried, or every worker heartbeat is stale; a mere backlog does
not restart healthy pods.

`cmd/gov-ar-admission/main.go` starts workers only after `engine.Ready`, cancels
them on process shutdown, waits for bounded drain, and exposes a worker-state
readiness component. `PostgresEngine` implements claims and completion in
`internal/govar/worker_postgres.go`; the in-memory engine is development-only and
must reject multi-replica mode as the chart already intends.

### 5.2 M7 tests

* Fake-clock unit tests for lease, renewal, retry/backoff, dead letter, and clean
  shutdown; no `time.Sleep` assertions.
* PostgreSQL concurrency test with two worker processes proving one effective
  transition and safe lease takeover after a killed claimant.
* Crash tests at claim/before transition/after transition/before acknowledgement.
* An ambiguous item stays charged through every retry and dead letter.
* A pending never-delivered outbox is released only with the exact documented
  authoritative state; duplicate worker execution has no second release.
* Retry attempt 2 cannot be dispatched unless it has a separate reservation and
  provider-attempt ID whose maximum billable categories were reserved.

## 6. M8: metrics, traces, and immutable audit

### 6.1 Prometheus

Create `internal/govarobservability/metrics.go` with an explicit Registry injected
into the service; do not use package-global registration in tests. At minimum:

* `govar_admission_decisions_total{decision,reason,method}`;
* `govar_transition_total{from,to,reason}`;
* `govar_reserved_micros`, `govar_settled_micros`,
  `govar_outstanding_liability_micros`, and `govar_carried_debt_micros` as
  snapshots with a bounded tenant-profile label for experiments, not raw tenant;
* `govar_reservation_component_micros{basis}` and
  `govar_settlement_component_micros{basis}`;
* `govar_decision_duration_seconds`, `govar_transaction_duration_seconds`, and
  `govar_settlement_delay_seconds` histograms with frozen buckets;
* `govar_worker_claims_total{kind,result}`, `govar_worker_backlog{kind,state}`,
  `govar_worker_oldest_age_seconds{kind}`, and heartbeat age;
* `govar_calibration_support`, `govar_calibration_coverage_ppb`,
  `govar_drift_detected`, and `govar_conservative_mode` keyed only by policy
  profile/detector;
* overflow, bound violation, pricing incomplete, audit verification, and
  unresolved-attempt counters.

Metrics are updated only after the corresponding transaction commits. A scrape
test compares gauges against SQL aggregates at a synchronization barrier.

### 6.2 Distributed traces

Add the OpenTelemetry Go SDK and OTLP exporter at pinned versions. Configuration
is environment/Helm controlled, disabled safely when no exporter is configured.
Extract and propagate W3C `traceparent`/`tracestate` across workload gateway,
Envoy ext_proc, admission HTTP, gateway transition calls, and the synthetic/live
provider adapter. Create spans for feasibility, reservation transaction, route
actuation, provider attempt, settlement, and reconciliation. Record only bounded
reason codes, model/provider aliases, snapshot digests, quantities, and state;
never prompts, credentials, private endpoints, or raw response bodies.

`AdmitResponse.TraceID` is the actual trace ID when tracing is active and a
separate `RequestID` remains the ledger idempotency key. If tracing is disabled,
the trace field is omitted rather than populated with a misleading request ID.

### 6.3 Append-only transactional audit

Add `govar_audit_events` in schema v4. Every reserve, dispatch, settlement,
correction, cancellation, expiry, rollover, budget adjustment, cohort
registration, calibration publication, drift change, and reconciliation writes an
event in the **same database transaction** as the state change. Each row contains:

* monotone per-tenant sequence allocated while the tenant row is locked;
* globally unique event ID and immutable event/payload kind;
* tenant, request, workload, provider-attempt, actor class, and bounded reason;
* before-state and after-state canonical SHA-256;
* policy, pricing, route, cap, cohort, software, and calibration digests;
* previous event hash and canonical event hash; server commit timestamp.

The runtime database role has INSERT/SELECT but no UPDATE/DELETE on this table.
A trigger owned by a different migration role rejects UPDATE/DELETE. PostgreSQL
superuser ability is explicitly outside the immutability assumption; the article
must say “append-only and hash-chained under role separation,” not absolutely
immutable. `article3/tools/verify_govar_audit.py` recomputes every chain and emits
a checksummed JSON result. Export redacts private endpoint and credential data.

### 6.4 M8 tests

* Metric name/label allowlist and bounded-cardinality test; duplicate requests do
  not increment effective-transition metrics twice.
* SQL-versus-scrape accounting test for a matched synthetic lifecycle.
* Trace-context integration test asserting one trace joins ext_proc, reservation,
  selected backend, and settlement; an exporter outage does not alter safety.
* Audit transaction rollback leaves neither state nor audit event; commit leaves
  both. UPDATE/DELETE fail for the runtime role.
* Duplicate/reordered events preserve one effective audit transition while still
  allowing a separately typed replay-observation event if prespecified.
* Tampering, deletion, reordering, and wrong checkpoint are all detected by the
  independent verifier.

## 7. M11: combined PostgreSQL + real Envoy Kind E0

### 7.1 What the gate must execute

Add a dedicated test, not an extension of the stub-only real Envoy unit test:

* `article3/infra/kind/e0/deploy.sh`: installs pinned PostgreSQL 16, the committed
  operator and GOV-AR admission images, pinned Envoy 1.31.10, a trace-driven
  backend, Prometheus, and (when M8 tracing is enabled) a pinned OpenTelemetry
  collector into the validation cluster. Images are digests or locally loaded
  images bound to the source manifest. Generated test certificates and secrets
  stay in a mode-0700 temporary directory and are deleted after collection.
* `article3/infra/kind/e0/fixtures/`: dedicated governed namespace, ServiceAccount,
  `AIWorkloadBinding`, budget/routing/provider/model objects, provider route
  binding, pricing/cap evidence fixture, and NetworkPolicies. No secret values.
* `operateur/test/e2e/govar_e0_test.go`: drives requests from a workload Pod
  through downstream mTLS to Envoy and the ext_proc service; it never calls
  `/v1/admit` directly as the publication-path success case.
* `article3/infra/kind/e0/verify.py`: reads API responses, Prometheus exposition,
  trace export, Kubernetes status, and a read-only PostgreSQL export; recomputes
  the expected component reservation and state transitions. It emits one raw
  manifest with source/config/image/fixture hashes and exact attempted/successful/
  failed counts.
* `article3/infra/kind/e0/collect.sh` and `destroy.sh`: collect before cleanup and
  are idempotent. Logs are redacted and all outputs are checksummed.

The ADMIT lifecycle assertion is:

`workload TLS identity -> Envoy -> ext_proc identity resolution -> current
AIWorkloadBinding -> typed candidate and complete liability -> PostgreSQL
RESERVED/outbox PENDING -> dispatch CLAIMED -> selected typed backend only ->
dispatch DELIVERED -> complete provider usage parse -> final SETTLED`.

The test recomputes and asserts exact tenant settled spend, zero outstanding hold
after authoritative finality, one reservation, one provider attempt, one effective
settlement, matching pricing/route/cap/policy digests, a valid audit chain, matching
metrics, and a joined trace. It also proves the non-selected backend received zero
requests and direct provider egress from the governed namespace fails.

Run negative cases through the same deployed binary and database for `QUEUE`,
`REJECT`, `ABSTAIN`, and `REQUIRE_APPROVAL`, plus stale price, stale cap, drift,
incomplete usage, duplicate final response, and a crash after reserve. Each case
asserts its exact reason, no unauthorized backend dispatch, and exact retained or
released liability. If a decision is intentionally unsupported by the current
policy semantics, it is a Phase D defect rather than a skipped E0 case.

### 7.2 Isolation and reproducibility

Use the validation profile because its Calico test proves NetworkPolicy
enforcement. PostgreSQL and admission run outside the governed workload namespace;
only the authenticated Envoy gateway is reachable from workloads and only Envoy
can reach provider backends. The test records node image, Kubernetes, Kind,
Calico, Envoy, PostgreSQL, Prometheus, OTel collector, project image IDs/digests,
chart rendering digest, CR resource versions, DB schema/layout ID, and host
contention metadata.

The E0 report is a gate artifact, not manuscript performance evidence. It must be
regenerated after any change to the measured path and its source manifest must
match the test report.

## 8. Sequencing and acceptance matrix

| Order | Checkpoint | Acceptance |
|---|---|---|
| 1 | Accepted route/approval/ledger repairs | Fresh independent source-bound review has no critical/major finding in those scopes. |
| 2 | M4 API + pure pricing/cap producers | API generation/manifests and all M4 unit/controller tests pass; old objects fail closed. |
| 3 | M4 ledger schema v4 | Fresh PostgreSQL and reviewed v3-to-v4 migration pass; complete component reserve/settle tests and race tests pass. |
| 4 | M5 producer | Typed-only admission, artifact reconstruction, no-leakage, drift/fallback, concurrency, and status tests pass. |
| 5 | M7 workers + M8 observability/audit | Crash/race/lease, SQL-versus-metrics, joined trace, DB role, and chain-verifier tests pass. |
| 6 | M11 E0 | Clean validation-cluster recreation executes the combined path and all decisions; independent verifier recomputes DB/metrics/trace/audit facts. |
| 7 | Full Phase D matrix and release security | Format, Go tests, race, vet, lint, envtest, PostgreSQL, gateway E2E, Helm lifecycle, supported-toolchain check, SBOM, vulnerability, and secret scan all pass on one source hash. |
| 8 | Independent implementation review | A fresh reviewer hashes every reviewed source and returns no unresolved critical or major finding. |

The following are bounded unsupported features, not safety blockers if they are
rejected explicitly and omitted from claims/experiments: streaming response
settlement (the current service rejects streaming), provider families without a
closed tested adapter, unbounded provider retries, PSI/KS/ADWIN before their
implementations are frozen, and universal audit immutability against a PostgreSQL
superuser. They must never silently fall back to heuristic parsing or a mean-cost
estimate.

## 9. M12: release toolchain and image evidence

Digest pinning is necessary for reproducibility, but it is not evidence that a
builder is supported or vulnerability-free. The release checkpoint must use a
currently supported, immutable Go builder digest that can compile the module's
declared language version. If `go.mod` remains `go 1.21`, CI must demonstrate that
the selected newer toolchain passes the complete test/race/vet/lint matrix; if the
module directive is raised, that is a reviewed compatibility change with the same
full rerun. Do not downgrade to the earlier Go 1.21.13 image merely to match the
directive.

Exact required evidence on the final committed source hash:

* `article3/provenance/build_base_images.json` records builder/runtime index and
  platform-manifest digests, declared and actual `go version`, target platform,
  module directive, and resolution commands. A moving-tree build is explicitly
  insufficient.
* `article3/tools/check_govar_dockerfile_pins.py` rejects tags without digests,
  known EOL toolchain lines, an unapproved target platform, and a provenance file
  whose Dockerfile/source hash does not match.
* Build the GOV-AR binary and image from a clean checkout. Record the binary hash,
  image ID, image config digest, runtime UID, entrypoint, and absence of the Go
  toolchain/source tree from the runtime image. Rebuild once in the release
  verifier environment; do not claim bit reproducibility unless hashes actually
  match under a documented deterministic build contract.
* Generate a committed CycloneDX or SPDX SBOM for the final image and run Trivy
  vulnerability and secret scans against the immutable image digest. The report
  includes scanner/database versions and timestamps. Any unresolved critical or
  high finding in the measured GOV-AR path blocks release unless the immutable
  gate contract's independent review process explicitly classifies a false
  positive with evidence; a pin alone never waives it.
* `go version -m` evidence from the compiled binary is reconciled
  with `go.mod`/`go.sum`, and dependency vulnerabilities are checked in addition
  to OS-package vulnerabilities. The distroless base does not remove Go module
  vulnerabilities embedded in the static binary.

Tests must include a negative fixture for the old Go 1.21.13 builder and an
unpinned builder, provenance/source mismatch, SBOM schema validation, scan failure
propagation, non-root execution, read-only-root-filesystem execution, and startup
failure when required identity/database configuration is missing. Release image
and scan evidence is regenerated after every code, module, Dockerfile, or base
digest change.
