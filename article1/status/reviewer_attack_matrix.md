# Reviewer Attack Matrix — Article 1

Generated: 2026-07-05. Final technical update: 2026-07-06.

Every major Q1-reviewer objection is mapped to a response section, an
experiment/test, raw evidence, status, and residual scope boundary.

Legend: `ADDRESSED` means code, test, raw evidence, and manuscript text are
present for the claimed scope.

| # | reviewer_objection | response_section | experiment_or_test | raw_file | status | residual_scope |
|---|---|---|---|---|---|---|
| 1 | Who verifies the attestation? | threat-model.tex; related-work.tex | central-verifier and A10 trust-chain separation | `article1/results/raw/aks/a10/` | ADDRESSED | node-level verifier scope |
| 2 | Can the node self-attest or write its own appraised evidence? | threat-model.tex | A10 RBAC and forged-evidence campaign | `article1/results/raw/aks/security_attacks_A1_A11.csv` | ADDRESSED | none for claimed scope |
| 3 | Is the evidence real or simulated? | evaluation.tex; limitations.tex | current-node MAA summary | `article1/results/raw/aks/attestation-real-summary.json` | ADDRESSED | kind remains regression-only |
| 4 | Is this node-level or pod-level attestation? | threat-model.tex; limitations.tex | node/VM evidence mapping | `article1/paper/tables/claim_evidence_mapping.csv` | ADDRESSED | pod-level not claimed |
| 5 | Why is this not just nodeSelector plus YAML? | related-work.tex | A1 label-forgery attack | `article1/results/raw/aks/security_attacks_A1_A11.csv` | ADDRESSED | none |
| 6 | Why does RuntimeClass alone not suffice? | related-work.tex; design.tex | A5 and A9 production-runtime attacks | `article1/results/raw/aks/security_attacks_A1_A11.csv` | ADDRESSED | do not frame `runc` as non-confidential |
| 7 | Why not schedulingGates plus external controller? | design.tex; evaluation.tex | B4 vs B5 TOCTOU comparison | `article1/results/raw/aks/b4_vs_b5.csv` | ADDRESSED | none |
| 8 | Why not C8s? | related-work.tex | qualitative positioning | `article1/paper/manuscript/related-work.tex` | ADDRESSED | no fabricated empirical comparison |
| 9 | Why not dstack-capsule? | related-work.tex | positioning against pod-level attestation | `article1/overleaf/references.bib` | ADDRESSED | orthogonal design scope |
| 10 | Why not Confidential Containers / Trustee? | related-work.tex | runtime key-release vs scheduling-time signal | `article1/paper/manuscript/related-work.tex` | ADDRESSED | composable, not a replacement |
| 11 | Are attacks adversarial or just unit tests? | threat-model.tex; evaluation.tex | A1-A10 on AKS N=30 plus RBAC principals | `article1/results/raw/aks/security_attacks_A1_A11.csv` | ADDRESSED | none for claimed scope |
| 12 | Are kind results representative? | limitations.tex; threats-to-validity.tex | evidence rule and table separation | `article1/status/q1_execution_report.md` | ADDRESSED | kind is CI/debug/regression only |
| 13 | Are AKS results real? | evaluation.tex | AKS MAA and confidential node snapshots | `article1/results/raw/aks/multinode-node-selection/evidence-initial.json` | ADDRESSED | four-node snapshot; still one cloud/region/SKU family |
| 14 | Is the TOCTOU window measured? | evaluation.tex | B4 vs B5 timing plus PreBind re-check | `article1/results/tables/b4_vs_b5_stats.csv` | ADDRESSED | none |
| 15 | Is the placement token verifiable offline? | design.tex; evaluation.tex | e2e verify-placement and token mutation checks | `article1/results/raw/aks/identity_binding.csv` | ADDRESSED | none |
| 16 | Are results statistically solid? | methodology.tex | N=30 attacks, N=30 latency, bootstrap/CI scripts | `article1/results/tables/security.csv` | ADDRESSED | large-scale AKS throughput not claimed |
| 17 | Is raw data retained? | reproducibility.tex | raw AKS files under `article1/results/raw/aks/` | `article1/results/raw/aks/` | ADDRESSED | private artifact pending IP review |
| 18 | Are limitations honest? | limitations.tex; threats-to-validity.tex | claim/evidence table and scope wording | `article1/paper/tables/claim_evidence_mapping.csv` | ADDRESSED | no pod-level/GPU/TDX claims |

## Summary

All reviewer objections are addressed for the claimed Article 1 scope: real AKS
node-level SEV-SNP attested placement, governed AI workload placement,
multi-node qualification, adversarial scheduling safety, verifiable placement
tokens, B4/B5 TOCTOU comparison, B1-B5 overhead, and scheduler RBAC minimality.
