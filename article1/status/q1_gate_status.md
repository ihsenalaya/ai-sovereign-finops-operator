# Q1 Gate Status — Article 1

Updated: 2026-07-06.

| Gate | Status | Evidence |
|---|---|---|
| G1 Real SEV-SNP attestation | PASS_AKS_REAL | `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`: four active `Standard_DC8as_v6` nodes had `evidenceMode=real`, `verificationStatus=Verified`, `simulated=0` in the multi-node snapshot. |
| G2 Real AI workloads | PASS_AKS_REAL | `article1/results/raw/aks/ai_workloads.csv`: OpenAI chat, OpenAI embedding, and local CPU model all PASS with placement-token verification. |
| G3 Multi-node node qualification | PASS_AKS_REAL | `article1/results/raw/aks/multinode_node_selection.csv`: valid binds; revoked, expired, and no-evidence nodes do not bind. |
| G4 Adversarial campaign | PASS_AKS_REAL | `article1/paper/tables/security_attack_matrix.csv`: A1-A10 are 300/300 blocked; A11 is 1/1 fail-closed GPU-scope control. |
| G5 B4 vs B5 baseline | PASS_AKS_REAL | `article1/results/raw/aks/b4_vs_b5.csv`: B4 median external window 1104.5 ms; B5 removes this externally schedulable window and emits a token. |
| G6 B1-B5 performance | PASS_AKS_REAL | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv`: 30 measured runs per baseline; B5 client-observed median 1167.0 ms; B5 scheduler-internal phase median 216.232 ms reported separately; anti-quantization PASS. |
| G7 Claim evidence traceability | PASS_AKS_ONLY | `article1/paper/tables/claim_evidence_mapping.csv` maps claims to raw AKS artifacts and out-of-scope claims. |
| G8 Scheduler self-security | PASS_SCOPE | `article1/results/raw/aks/scheduler_security_tests.csv`: RBAC minimality audit; not claimed as a full scheduler compromise proof. |
| G9 Manuscript readiness | PASS_TECHNICAL | Manuscript and Overleaf sources updated; final compile/editorial/IP review still required before submission. |

Current submission verdict: technical evidence is complete for the claimed
node-level SEV-SNP AKS scope, subject to final PDF compile, editorial polish,
bibliography/IP checks, and author approval.
