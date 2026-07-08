# Thesis Bench

Scaffold for reproducible experiments supporting the thesis:

**Attestation-Aware Kubernetes Scheduling for Confidential AI Inference:
Verifiable Placement, Conditional Key Release and Auditable Evidence**

## Current repository assets

- Existing experimentation assets under `experimentation/`
- Quality and routing measurement scripts already present
- New platform primitives now available in CRDs and controllers

## Planned benchmark extensions

- Placement-verification scenarios
- Conditional key-release success/failure scenarios
- Revocation latency experiments
- Audit-chain continuity and checkpoint verification
- Simulated-vs-production mode safety checks

## Notes

- Local `kind` runs remain explicitly simulated.
- Real Confidential GPU claims must stay disabled until validated on private AKS.
