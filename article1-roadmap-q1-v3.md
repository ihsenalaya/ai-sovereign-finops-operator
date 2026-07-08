# Article 1 — Roadmap Q1 v3 (reviewer-gate logic)

Replaces the old checklist logic with **reviewer gates G0–G7**. A gate is
`PASS` only when a real reviewer objection is answered with code + tests + raw
results + manuscript text. Statuses: `PASS | PARTIAL | FAIL | BLOCKED | NOT_STARTED`.

Title (honest, SEV-SNP node-level): *Attestation-Aware Scheduling for Verifiable
AI Placement on SEV-SNP Confidential Kubernetes Nodes*.

| Gate | Meaning | Required artifacts | Status |
|------|---------|--------------------|--------|
| G0 | Evidence trust chain | RawAttestationReport CRD, node-agent, central-verifier, RBAC separation, envtest, A10 | NOT_STARTED |
| G1 | Real AKS SEV-SNP attestation | real cluster, MAA/guest attestation called + verified, evidenceMode=real, raw in results/raw/aks | NOT_STARTED |
| G2 | Real adversarial campaign | A1–A10 vs deployed system, distinct adversarial SAs, ADV-1..4, raw CSV | PARTIAL (5 on-cluster kind attacks exist, not distinct SAs) |
| G3 | Strong baseline | B4 schedulingGate+external verifier, B4 vs B5, TOCTOU measured | NOT_STARTED |
| G4 | Performance/scalability | AKS perf if G1, N≥30, p50/p95/p99/CI95, scheduler CPU/mem | NOT_STARTED |
| G5 | Manuscript readiness | formal threat model, verified related work, no stubs, LaTeX compiles | PARTIAL (compiles, but stubs) |
| G6 | Reviewer attack matrix | ≥15 objections each answered | NOT_STARTED |
| G7 | Claim→evidence traceability | claim_evidence_mapping.csv complete, no unsupported claim | NOT_STARTED |

## Execution order (build software before paying for AKS)
1. G0 trust chain (CRD + agent + verifier + RBAC + envtest + A10).
2. Formal adversary model (ADV-1..4) + threat-model.tex.
3. G1 software: `pkg/attestation/maa` (real MAA verify) + tests.
4. G3 software: B4 baseline (schedulinggate + external verifier).
5. G1 infra: AKS campaign scripts → apply → real attestation → collect → destroy.
6. G2/G4 experiments (Exp1–Exp6, N≥30) on AKS + kind/kwok.
7. G4 stats + figures (Python, bootstrap CI, Mann-Whitney).
8. G5 related work (verified) + full 16-section manuscript.
9. G6 reviewer_attack_matrix.md, G7 claim_evidence_mapping.csv.
10. Q1 checklist, validation commands, final report.

## Honesty escape valves (per roadmap §4, §5.G1)
- If real MAA verification is not achievable in this environment → G1 = FAIL,
  document precisely, NEVER mark evidence real.
- Any experiment lacking a raw file must NOT appear as a result.
- kind/kwok always labelled; never presented as real cloud perf.
