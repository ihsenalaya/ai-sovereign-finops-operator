# AIWorkloadBinding

`AIWorkloadBinding` is the operator-owned, namespaced identity and governance
assignment for one Kubernetes ServiceAccount. GOV-AR resolves it from the
authenticated Pod's live `namespace` and `spec.serviceAccountName`; a request or
Pod annotation cannot choose a binding.

The binding name is deterministic: `metadata.name` must exactly equal
`spec.serviceAccountName`. Its namespace is the ServiceAccount namespace. The
API schema rejects empty identity/policy fields, duplicate zones, non-normalized
zones, and a name that differs from the ServiceAccount.

| Field | Meaning |
| --- | --- |
| `spec.serviceAccountName` | Authenticated Kubernetes workload principal and binding name. |
| `spec.tenantID` | Financial-isolation identity. |
| `spec.team`, `spec.application` | Operator-assigned organizational identity. |
| `spec.budgetPolicyRef` | Same-namespace `AIBudgetPolicy`. |
| `spec.routingPolicyRef` | Same-namespace `AIRoutingPolicy`. |
| `spec.sensitivity` | `low`, `medium`, or `high`. |
| `spec.allowedZones` | Unique normalized lower-case provider zones. |
| `spec.requireGateway` | Whether the authenticated GOV-AR gateway is mandatory; defaults to `true`. |

## RBAC boundary

Application authors and workload ServiceAccounts must not receive `create`,
`update`, `patch`, or `delete` permissions for `AIWorkloadBinding`. Only the
operator/governance administrator role may mutate bindings. Read access should
also be minimized because tenant and policy assignments are security metadata.
The generated API intentionally does not grant workload-facing write RBAC.

Changing a binding creates a new Kubernetes generation. Enforcement must use a
binding only when `status.observedGeneration` equals `metadata.generation` and a
`Ready=True` condition confirms that both policy references resolved. Otherwise
admission fails closed. The status also records the resolved ServiceAccount UID
and each policy's name, UID, and generation. Admission compares these values to
the live objects, so delete/recreate or stale-reference races cannot inherit a
prior binding decision.
