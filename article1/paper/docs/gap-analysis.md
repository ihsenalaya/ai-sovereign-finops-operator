# Gap Analysis

Updated: 2026-07-06.

## What Exists In The Repository

- Confidential CRDs and policy objects.
- Pod mutation and validation webhooks.
- Attestation-aware scheduler with Filter, Reserve, PreBind, and Bind logic.
- Ed25519 placement token and independent `verify-placement` CLI.
- Canonical hashing and signed decision evidence.
- Central-verifier flow from raw node reports to appraised AttestationEvidence.

## What Was Missing And Is Now Resolved

- Shared canonical hashing.
- PreBind re-check against policy and evidence changes.
- Bounded evidence availability behavior.
- A2/A3/A6/A7/A9 deployed AKS adversarial coverage.
- AKS scheduler-path latency with N=30.
- B4 vs B5 TOCTOU comparison.
- Reviewer-facing claim/evidence mapping.

## Remaining Before Submission

- Keep `kind` and KWOK as CI/debug/regression only.
- Keep the final empirical claims tied to AKS real artifacts in
  `article1/results/raw/aks/`.
- Perform final bibliography, editorial, IP, and author review.
- Do not claim pod-level attestation, confidential GPU execution, Intel TDX, or
  real AKS large-scale throughput.
