# Q1 Execution Report — Article 1

Updated: 2026-07-06.

All principal empirical results below are from real AKS SEV-SNP
(`aks-real-sevsnp`). Local `kind`/KWOK artifacts are CI/debug/regression only.

| Experiment | Raw artifact | Result |
|---|---|---|
| Real attestation | `article1/results/raw/aks/multinode-node-selection/evidence-initial.json` | 4 active confidential `Standard_DC8as_v6` nodes with real verified SEV-SNP evidence in the final multi-node snapshot. |
| Real AI workloads | `article1/results/raw/aks/ai_workloads.csv` | 3/3 PASS: OpenAI chat, OpenAI embedding, local CPU model; all placement tokens verify. |
| Multi-node qualification | `article1/results/raw/aks/multinode_node_selection.csv` | 4/4 expected outcomes: valid binds; revoked, expired, no-evidence do not bind. |
| Security A1-A10 | `article1/results/raw/aks/security_attacks_A1_A11.csv` | 300/300 blocked. |
| A11 GPU-scope fail-closed | `article1/results/raw/aks/a11_gpu_required_no_evidence.csv` | 1/1 denied as expected; not a GPU evaluation. |
| B1-B5 performance | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | 30 measured runs per baseline; B5 client-observed median 1167.0 ms; B5 scheduler-internal phase median 216.232 ms reported separately; anti-quantization PASS. |
| B4 vs B5 | `article1/results/raw/aks/b4_vs_b5.csv` | B4 median external gate-to-bind window 1104.5 ms; B5 removes that window from the schedulable path. |
| Identity binding | `article1/results/raw/aks/identity_binding.csv` | Correct token verifies; bound-field mutations fail. |
| Scheduler RBAC minimality | `article1/results/raw/aks/scheduler_security_tests.csv` | PASS for required/forbidden RBAC checks; scoped as RBAC minimality. |

Final AKS environment for the new runs: westus2, confidential pool `conf`,
`Standard_DC8as_v6`, four active confidential nodes under a 32-vCPU DCasv6
quota, node-level SEV-SNP with `runtimeClassName=runc`.
