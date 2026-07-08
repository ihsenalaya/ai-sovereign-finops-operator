# Implementation Plan

## Current repository baseline

- Existing operator scope already covers FinOps, sovereignty, routing, reporting, break-even and quality gate.
- Current API group is `aiops.imperium.io`.
- Current stack is Kubebuilder + controller-runtime + Helm/Kustomize + Prometheus metrics.

## Planned evolution

1. Keep the existing governance operator behavior stable and backward-compatible.
2. Add shared packages under `pkg/` for cryptography and statistical quality decisions.
3. Introduce the new confidential-governance CRDs without renaming the API group.
4. Extend the quality gate with auditable statistical verdicts, dataset hashing and hysteresis.
5. Scaffold simulated confidential-execution building blocks for kind and future AKS/private deployment.
6. Prepare evidence, revocation, placement and key-release APIs so later controllers can be added without CRD churn.

## Rules for the migration

- Preserve all existing CRDs and controllers.
- Additive API changes only in `v1alpha1` during this phase.
- No fake success paths: simulated behavior must stay explicit and opt-in.
- New platform components land as foundations first, then controllers/webhooks/scheduler logic in follow-up phases.
