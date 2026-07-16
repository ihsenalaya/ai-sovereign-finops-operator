# GOV-AR approval migration

Article 3 prereleases remove the request-level `AIAdmissionApproval` and
`AIAdmissionApprovalDecision` APIs. They also remove the associated controller,
one-use Lease, admission write permission, and Helm CRD files. Existing
`AIChangeRequest` reroute objects remain compatible.

This is intentionally not an automatic data conversion. A request approval
binds one historical request and cannot establish the broader policy-level
scope required by `authorize-gov-ar-route` without inventing reviewer intent.
Before upgrading a cluster that used the experimental request-level APIs:

1. Stop new traffic that depends on a canary-enabled routing policy.
2. Inventory the old proposal, decision, and `govar-consumed-*` Lease objects;
   retain their YAML and audit logs according to the deployment's evidence
   policy. They are not accepted by the new admission service.
3. For every route that should remain authorized, obtain the live routing
   policy, model, provider, and complete GOV-AR route snapshot. Create a new
   `AIChangeRequest` with action `authorize-gov-ar-route`, the exact object
   UIDs/generations and route-snapshot hash, an explicit `validUntil`, and the
   computed `scopeDigest`. A separately authorized reviewer sets
   `spec.approval: Approved`.
4. Wait for controller status `phase: Approved`, matching
   `observedGeneration`, matching `approvedScopeDigest`, `approvedAt`, and the
   exact `expiresAt`. A dry-run admission must either discover this object or
   accept its name through `approval_ref` before traffic resumes.
5. Old custom resources and CRDs may be deleted only after the retained audit
   export and rollback window are complete. Helm does not delete CRDs during an
   upgrade, so their continued presence does not mean the new binary uses them.

Admission is read-only for `AIChangeRequest`; it never creates or updates this
governance state. Deleting, replacing, changing the generation of, or expiring
any bound policy/model/provider/change object makes the approval unusable. A
new bounded change request is required; copying controller-owned status is not
valid migration evidence.
