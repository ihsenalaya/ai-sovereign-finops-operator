# Typed GOV-AR safety fields

GOV-AR safety decisions consume typed CRD fields rather than treating mutable
annotations as authoritative. Existing annotations remain present only for
backward compatibility; their presence does not establish routability, price
completeness, a verified cap, current calibration, or a theorem-bearing cohort.

## AIWorkloadBinding

One namespaced `AIWorkloadBinding` binds a ServiceAccount to its operator-owned
tenant, budget, routing, sensitivity, and residency assignment. Its name must
equal the ServiceAccount name and its complete spec is update-immutable; an
identity or policy change requires replacement and fresh controller-produced
ServiceAccount/policy UID and generation evidence. Workload principals must not
receive create, update, patch, or delete permission for these bindings.

The admission deployment uses a separate read-only ServiceAccount. It can read
the live Pod, ServiceAccount, binding, policies, models, and providers and can
create TokenReviews; it does not inherit the controller manager's write role.

## AIModel

`spec.govar.routable` is the explicit catalog decision. When true,
`spec.govar.route` is required and provides the provider deployment/model key,
Envoy cluster, HTTP authority, and a closed `pathMode` adapter enum. Admission
returns this route as one unit so a gateway cannot combine a selected model with
an unrelated backend.

`status.govar.verifiedOutputCap` records whether a provider-enforced output cap
was verified, its maximum tokens, observation time, and immutable source
version. `status.govar.latency` records mean/p95/p99 milliseconds, sample count,
and observation time. Consumers must check status freshness and generation.

## AIProvider pricing

`spec.pricing.version`, `observedAt`, and `completeness` identify a versioned
price snapshot. `completeness: complete` is an explicit assertion that all
possible charges are represented. `billableCategories` is a map keyed by a
stable category name and carries an exact decimal `resource.Quantity` price and
unit for charges beyond base input/output tokens.

## AIRoutingPolicy

`spec.govar` contains the reservation method and method-specific token values,
an immutable calibration artifact reference and freshness/support requirements,
the predeclared drift detector and conservative fallback, a frozen cohort
registry snapshot, and a parts-per-billion tenant risk budget/allocation. CEL
validation rejects missing method inputs, missing adaptive calibration, or a
fixed-cohort method without cohort and risk configuration.

`status.govar` is controller-produced evidence. It reports the validated
calibration estimate/support/timestamp and visible drift/conservative-mode
state. Merely populating the spec does not make calibration valid.
