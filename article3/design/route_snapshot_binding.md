# Immutable admission-to-dispatch route binding

Status: implementation design, not evidence of implementation or verification
Finding addressed: critical admission/dispatch TOCTOU and provider-route misbinding
Scope: the synchronous GOV-AR admission, reservation, Envoy `ext_proc`, and PostgreSQL measured path

## Failure in the current source

Admission constructs candidates from one Kubernetes read of `AIModel` and
`AIProvider` and reserves against the resulting model, prices, compliance state,
and route fields. The reservation persists only the selected model name, a
concatenated resource-version string, prices, and pricing version. After the
reservation commits, `govarextproc.Server.admitAndClaim` calls `resolveRoute`,
and the production resolver performs a second `AIModel` lookup by name. Envoy
therefore actuates the route that exists at dispatch time, not necessarily the
route for which liability and governance feasibility were checked.

The unsafe source boundaries are:

- `operateur/internal/govar/snapshot.go`: `Candidate` carries route strings but
  not immutable object identities, and `BuildCandidates` takes cluster,
  authority, path adapter, and provider deployment from the model object.
- `operateur/internal/govar/engine.go`: `Reservation` and `AdmitResponse` omit
  the exact route and model/provider identities.
- `operateur/internal/govar/postgres_engine.go` and
  `article3/infra/postgres/init.sql`: the reservation row cannot reconstruct the
  route admitted for a request.
- `operateur/internal/govarextproc/server.go`: `admitAndClaim` re-resolves the
  selected model after admission.
- `operateur/cmd/gov-ar-admission/main.go`: `extProcRouteResolver` performs the
  second Kubernetes read and returns its current route.
- `operateur/api/v1alpha1/aimodel_types.go`: the model owns the complete route,
  so a model referring to provider `P` can independently name a cluster and
  authority belonging to provider `Q`.

`metadata.generation` alone does not close the race. A delete/recreate can reuse
the name with a new UID, and status-only changes can change readiness or
observations without changing generation. The frozen record must contain UID,
generation, and resourceVersion for both objects.

## Safety objective

For every admitted provider attempt, the exact upstream route actuated by Envoy
must equal the route atomically persisted with the reservation. That persisted
route must be the provider-owned binding resolved from the exact model and
provider objects used for feasibility and reservation. Kubernetes catalog
creation, deletion, recreation, or mutation after the reservation commits must
have no effect on that provider attempt.

This is a binding and auditability property. It does not assert that an Envoy
cluster name is correctly configured in a separate Envoy bootstrap. Cluster
configuration conformance remains a deployment gate.

## Provider-owned typed API

Move every actuation field out of `AIModel`. Use a provider-owned list whose
entries are complete exact routes:

```go
// GOVARRoutePathMode replaces the model-scoped type name. The enum remains
// openai-body, azure-deployment-path, anthropic-body, google-generate-path.
type GOVARRoutePathMode string

type AIProviderGatewayRouteBinding struct {
    // Name is the list-map key referenced by an AIModel.
    Name string `json:"name"`
    // ProviderDeployment is the exact provider-side deployment/model id.
    ProviderDeployment string `json:"providerDeployment"`
    // Cluster is the exact Envoy cluster key.
    Cluster string `json:"cluster"`
    // Authority is the exact upstream HTTP authority.
    Authority string `json:"authority"`
    // PathMode selects the closed request adapter.
    PathMode GOVARRoutePathMode `json:"pathMode"`
}

type AIProviderGOVARSpec struct {
    // +listType=map
    // +listMapKey=name
    GatewayRoutes []AIProviderGatewayRouteBinding `json:"gatewayRoutes"`
}

type AIProviderSpec struct {
    // existing fields ...
    GOVAR *AIProviderGOVARSpec `json:"govar,omitempty"`
}

type AIModelGOVARSpec struct {
    Routable bool `json:"routable"`
    // RouteBindingRef selects a binding only from the AIProvider named by the
    // model's existing spec.providerRef.
    RouteBindingRef string `json:"routeBindingRef,omitempty"`
}
```

The model no longer supplies provider deployment, cluster, authority, or path
mode. The existing uncommitted `AIModelGOVARSpec.Route` must not be treated as
safety authority. Because this typed route API has not been released, replace
it in the generated CRDs, chart CRD copies, samples, approval structures, tests,
and documentation. If a compatibility field is temporarily retained for API
decoding, mark it deprecated and make `BuildCandidates` ignore it.

This structural ownership also needs an RBAC ownership boundary: workloads and
model-catalog writers must not be able to edit `AIProvider.spec.govar`; only the
provider-catalog controller/administrator role may do so. The admission and
gateway service accounts remain read-only. The typed relation prevents a model
writer from injecting a cluster or authority; it does not make a
misconfigured/malicious provider administrator trustworthy.

Required validation is fail closed:

1. a routable model has a non-empty `routeBindingRef`;
2. provider route names are unique list-map keys and every field satisfies the
   existing closed enum and character constraints;
3. `BuildCandidates` looks up the provider by `model.spec.providerRef`, then
   looks up the binding only in that provider's `spec.govar.gatewayRoutes`;
4. a missing or duplicate binding produces `ReasonNotRoutable` and never falls
   back to annotations, model fields, URLs, or name heuristics;
5. model UID/resourceVersion and provider UID/resourceVersion must be non-empty,
   their generations must be positive, and each Ready status must observe the
   same generation before the candidate is feasible;
6. provider-type/path-mode compatibility is checked explicitly. At minimum,
   `azure-openai` requires `azure-deployment-path`, `anthropic` requires
   `anthropic-body`, and `vertex` requires `google-generate-path`. Other allowed
   provider types must have an enumerated compatibility rule; `custom` must not
   bypass adapter validation.

Approval is policy/change level, never request level. An
`AIChangeRequest` with action `authorize-gov-ar-route` binds the exact routing
policy UID/generation, model UID/generation, provider UID/generation,
provider-owned route-snapshot digest, and absolute expiry. Its immutable scope
digest changes when any bound identity, route, or expiry changes. The
controller writes the independently recomputed digest to status only after a
human approval. Admission only reads this status: it neither creates a
Kubernetes object nor consumes a Lease for a request. The same approval may be
reused within its exact scope while every live identity and the complete route
snapshot still match; otherwise admission returns `REQUIRE_APPROVAL`.

## Frozen internal snapshot

Add one JSON-safe value type in `operateur/internal/govar` and use the same JSON
shape in `AdmitResponse` and `govarextproc`:

```go
type RouteSnapshot struct {
    Namespace                 string `json:"namespace"`
    ModelName                 string `json:"model_name"`
    ModelUID                  string `json:"model_uid"`
    ModelGeneration           int64  `json:"model_generation"`
    ModelResourceVersion      string `json:"model_resource_version"`
    ProviderName              string `json:"provider_name"`
    ProviderUID               string `json:"provider_uid"`
    ProviderGeneration        int64  `json:"provider_generation"`
    ProviderResourceVersion   string `json:"provider_resource_version"`
    PricingVersion            string `json:"pricing_version"`
    PricingComplianceHash     string `json:"pricing_compliance_hash"`
    RouteBindingName          string `json:"route_binding_name"`
    ProviderDeployment        string `json:"provider_deployment"`
    Cluster                   string `json:"cluster"`
    Authority                 string `json:"authority"`
    PathMode                  string `json:"path_mode"`
    SnapshotHash              string `json:"snapshot_hash"`
}
```

`Candidate` should contain this value instead of the four loose `Route*`
strings. `Reservation` gets a `RouteSnapshot` field. `AdmitResponse` gets
`RouteSnapshot *RouteSnapshot` with JSON name `route_snapshot`; it is required
when and only when the decision is `ADMIT`. Keep `selected_deployment` as the
catalog `AIModel` name for wire compatibility, and require it to equal
`route_snapshot.model_name`. Keep the top-level pricing version and require it
to equal `route_snapshot.pricing_version`.

`PricingComplianceHash` is SHA-256 over a versioned, fixed-field canonical
record containing all provider inputs that affect cost or governance:

- provider type, region, data residency, and managed flag;
- currency, pricing version, observed-at value, completeness, base input and
  output prices after exact integer-micro conversion, and fixed monthly price
  when present;
- every billable category as `(name, unit, exact canonical quantity)`, sorted
  by name;
- sensitive-data permission and normalized, sorted allowed countries.

Use a domain separator such as `govar-pricing-compliance-v1`. Reject duplicate
billable names or countries rather than letting ordering or last-write behavior
affect the digest. Do not hash Go maps, floating-point renderings, or raw JSON.

`SnapshotHash` is SHA-256 over all preceding `RouteSnapshot` fields in their
declared order, excluding `SnapshotHash`, with domain separator
`govar-route-snapshot-v1`. Use one implementation for construction and
validation. The existing `CandidateSnapshotVersion` may remain as a compatibility
column but must equal `SnapshotHash`; it must no longer be the ambiguous
`modelResourceVersion + "|" + providerResourceVersion` string.

`BuildCandidates` constructs and validates this snapshot before setting a
candidate feasible. Both in-memory and PostgreSQL `Admit` validate it again
before changing tenant liability. A reservation loaded from storage must be
validated before it can be returned, claimed, delivered, settled, canceled, or
reconciled. A digest mismatch fails closed and leaves liability held.

### Policy-level approval scope digest

`GOVARRouteApprovalScope` uses typed references, not a free-form map. Its
`ScopeDigest` is SHA-256 over the routing-policy, model, and provider names,
UIDs, and generations, the complete `RouteSnapshot.SnapshotHash`, and
`ValidUntil`, with the digest field cleared during recomputation and domain
separator `govar-route-approval-scope-v1`. The
`AIChangeRequest` CRD makes the scope immutable after creation. Controller
status records the observed change-request generation, approved scope digest,
approval time, and exact expiry. Admission requires all of them and validates
the live complete route snapshot again. Tests mutate every scope category and
prove that the digest or live-identity comparison invalidates the approval.

The derived ledger policy-version string additionally includes the approved
change-request UID, generation, and full scope digest. This preserves the exact
governance authorization used by the reserve transaction without introducing
request-level Kubernetes state or a non-atomic consumption marker.

The admission fingerprint must also bind the same `SnapshotHash`. Assigning the
validated hash to `Candidate.SnapshotVersion` is sufficient only if every
candidate construction path enforces that equality; directly supplied test or
internal candidates must not bypass snapshot validation.

## PostgreSQL schema and migration

Schema version 4 adds the following non-null columns to
`govar_reservations`:

```sql
route_namespace TEXT NOT NULL,
selected_model_uid TEXT NOT NULL,
selected_model_generation BIGINT NOT NULL CHECK (selected_model_generation > 0),
selected_model_resource_version TEXT NOT NULL,
selected_provider_name TEXT NOT NULL,
selected_provider_uid TEXT NOT NULL,
selected_provider_generation BIGINT NOT NULL CHECK (selected_provider_generation > 0),
selected_provider_resource_version TEXT NOT NULL,
pricing_compliance_hash TEXT NOT NULL CHECK (pricing_compliance_hash ~ '^[0-9a-f]{64}$'),
route_binding_name TEXT NOT NULL,
route_provider_deployment TEXT NOT NULL,
route_cluster TEXT NOT NULL,
route_authority TEXT NOT NULL,
route_path_mode TEXT NOT NULL CHECK (route_path_mode IN
  ('openai-body','azure-deployment-path','anthropic-body','google-generate-path')),
route_snapshot_hash TEXT NOT NULL CHECK (route_snapshot_hash ~ '^[0-9a-f]{64}$')
```

`selected_deployment` remains the model name. The insert, row loader, schema
layout checks, fresh `init.sql`, clean migration script, database documentation,
and PostgreSQL tests must move together to v4. The outbox need not duplicate
the route because its request FK points to exactly one reservation; the outbox
invariant check must additionally require a valid reservation route snapshot
before claim/delivery.

There is no defensible backfill for an existing reservation: current Kubernetes
state cannot prove the route that was admitted earlier. Therefore the v3-to-v4
migration must run in a transaction and abort if `govar_reservations` is
non-empty. It must also abort if either frozen-cohort table is non-empty. A v1
cohort signs data, config, protocol, risk, and pre-outcome slot hashes, but the
current runtime does not prove that its config/protocol hashes designate the v4
ledger layout and route-snapshot implementation. Silently retaining such a
cohort would allow a pre-v4 allocation registry to authorize admissions under
different safety code.

Operators must preserve a non-empty v3 database as read-only audit evidence and
start a clean v4 ledger. Do not fabricate route snapshots from live catalog
objects and do not recompute or rewrite a frozen cohort's registry digest or
authority proof. The runtime must require the exact v4 layout identifier and
all columns; a partially altered database is not Ready.

For v4 fixed-cohort admission, introduce registry digest domain
`govar-frozen-cohort-v2`. Its canonical payload retains all v1 fields and adds
an explicit `ledger_layout_id`, `route_snapshot_schema`, and immutable software
hash. Registration and admission require those values to equal the running
engine's configured expected values. Register fresh v2 cohorts only after the
new code/config/protocol hashes are frozen. V1 cohort rows remain verifiable
audit evidence but are ineligible to authorize v4 admissions.

Do not put a per-request route snapshot hash into `OpportunityDigest` or the
cohort slot. The opportunity is frozen before online selection, whereas a route
snapshot is produced at admission. The cohort registry allocates risk across
pre-outcome opportunities; the reservation and approval digests separately
bind the route selected for each realized opportunity.

## Admission and Envoy flow

The repaired flow is:

1. The admission handler reads the model and its referenced provider once,
   resolves the provider-owned binding, computes both hashes, and passes the
   complete candidate snapshot to the ledger.
2. The serializable admission transaction persists the complete snapshot in
   the same reservation insert that increases outstanding liability and creates
   the pending outbox row.
3. Only after commit, `responseForReservation` returns the persisted snapshot.
   PostgreSQL response construction must use the just-persisted/loaded
   reservation, not the pre-insert candidate object.
4. `govarextproc.admitResult` decodes `route_snapshot`. It verifies all required
   fields, both equality checks, the closed path-mode enum, and the snapshot
   hash before request rewrite or dispatch claim.
5. `admitAndClaim` converts that validated snapshot directly into `RouteTarget`,
   rewrites the body/path, and issues the claim. It performs no Kubernetes read
   and no route-registry lookup.
6. Add `RouteSnapshotHash` to the internal `DispatchRequest`; both CLAIMED and
   DELIVERED events echo the hash held in `streamState`, and the ledger requires
   exact equality with the reservation before changing outbox state. Include it
   in the inbox event payload hash. This makes a stale/mismatched gateway claim
   fail closed and leaves an auditable binding to the returned snapshot.
7. Remove `Server.RouteRegistry`, `Server.ResolveRoute`, `resolveRoute`, the
   production `extProcRouteResolver`, and their RBAC rationale. Retain only the
   principal resolver because workload identity is established before admit.
8. Emit the route snapshot hash on structured dispatch/settlement audit events
   and metrics labels only where cardinality is deliberately bounded. Never put
   provider credentials or endpoint URLs in the snapshot.

The exact route remains in `streamState` for response settlement correlation.
Dispatch, settle, and cancel continue to bind to the provider attempt ID. If
snapshot validation or request-adapter validation fails after reservation but
before claim, no provider request is sent; the pending reservation remains held
for the existing reconciliation/expiry path. It must not be released merely
because route validation failed unless a trusted worker proves no dispatch was
possible.

## Required regression tests

Each counterexample needs both in-memory and PostgreSQL coverage where it
crosses the ledger, plus a real-Envoy path test for actuation:

1. **Route mutation between reserve and dispatch.** Admit model `M` through
   provider binding `(P, deployment-a, cluster-a, authority-a)`, mutate the
   catalog to route `b`, then process the request body. Envoy must send only to
   route `a`; route `b` receives zero requests.
2. **Delete/recreate with the same model name.** Admit UID `u1`, delete/recreate
   `M` as UID `u2` with another route, then dispatch. The response and database
   retain `u1` and the original target, and Envoy uses the original target.
3. **Provider-route misbinding.** Model `M` references provider `P`; a binding
   name that exists only under provider `Q` is infeasible. A model has no field
   capable of injecting `Q`'s cluster or authority.
4. **Provider mutation after reserve.** Change price, compliance, generation,
   and route on `P` after commit. The active attempt retains the original
   pricing/compliance and route hashes and actuates the original route. A later
   request sees the new version or abstains; the old request is not rewritten.
5. **Snapshot tampering.** Change each returned route field without recomputing
   the hash, and separately change the hash. `ext_proc` denies before claim and
   no backend receives the request.
6. **Cross-field mismatch.** A validly hashed snapshot whose model name or
   pricing version conflicts with the top-level response is denied.
7. **Duplicate request/replay.** Loading the original reservation produces
   byte-equivalent route snapshot fields and hash; it never recomputes from the
   current catalog. If duplicate admission remains non-dispatchable by policy,
   test the ledger response directly.
8. **Migration refusal and cohort preservation.** v3 with at least one
   reservation or one frozen cohort/slot must fail v4 migration atomically.
   Empty v3 migrates to the exact v4 layout. Failed migration leaves every v1
   cohort digest and authority proof byte-identical. V1 is rejected for v4
   admission; a freshly signed v2 cohort with matching layout, route-snapshot
   schema, and software hash succeeds. A partial or mislabeled v4 schema makes
   `/readyz` fail.
9. **Real Envoy.** Pause after successful `/v1/admit`, mutate/delete/recreate
   the catalog, resume ext_proc, and assert the captured upstream cluster,
   authority, protocol path/body deployment, provider attempt ID, and snapshot
   hash all match the reservation row.

Tests must also show that malformed, missing, and unknown provider bindings are
infeasible; model annotations and the deprecated model route cannot restore
feasibility.

## Formal-model change

Extend the formal state with immutable `reservation.routeSnapshot`, mutable
`catalog`, and `dispatchTarget`. Add catalog mutation and same-name
delete/recreate transitions between reserve and claim. Check at least these
invariants:

- **ProviderRouteOwnership:** at reserve time, the snapshot route binding is an
  entry of the exact provider object referenced by the exact model object, and
  the stored model/provider UID, generation, and resourceVersion equal those
  objects.
- **RouteSnapshotIntegrity:** every reservation eligible for claim has a valid
  pricing/compliance digest and route snapshot digest.
- **DispatchUsesReservedSnapshot:** for every claimed or delivered attempt,
  `dispatchTarget == reservation.routeSnapshot.route`; current catalog state is
  absent from the dispatch transition.
- **ReplayStableRoute:** a semantic duplicate can return only the original
  snapshot and cannot replace it.
- **CatalogMutationCannotRetarget:** arbitrary mutation, deletion, or
  same-name recreation after reserve does not change any reservation snapshot
  or dispatch target.

Add these scenarios to the exhaustive checker and update the result schema and
source-bound theory review only after implementation. Until the new checker is
run and its hashes are recorded, the current formal pass does not establish
route-snapshot safety.

## Acceptance boundary

This finding is resolved only when the generated API/CRDs, both ledger engines,
PostgreSQL v4 schema and guarded migration, admission response, ext_proc path,
approval digest, unit/PostgreSQL/real-Envoy regressions, and formal checker all
implement the same snapshot shape and digest. A test that mutates the catalog
between committed reserve and actual Envoy upstream selection is mandatory.
Merely comparing resource versions during a second lookup is insufficient: it
would turn a safe immutable actuation requirement into avoidable post-reserve
denial and would still leave no durable record of the admitted route.
