# Takeover Inventory — Article 1 Q1 Finalization

Generated: 2026-07-05. Final technical update: 2026-07-06.

| component | status | path | evidence | issue | next_action |
|---|---|---|---|---|---|
| GHCR image policy | PASS | `README.md`, charts, docs | Images use `ghcr.io/ihsenalaya/ai-sovereign-finops-operator`, version `0.5.11`. | ACR must not be reintroduced. | Keep GHCR-only. |
| AKS runtime scope | PASS_WITH_SCOPE | AKS node pool snapshots | `Standard_DC8as_v6`, node-level SEV-SNP, runtime `runc`. | No pod-level Kata claim. | Preserve node-level wording. |
| Real SEV-SNP evidence | PASS | `article1/results/raw/aks/multinode-node-selection/evidence-initial.json` | Four active confidential nodes had real verified evidence in the multi-node snapshot. | Current evidence can fluctuate as new raw reports arrive. | Use the captured snapshot for paper claims. |
| Governed AI workloads | PASS | `article1/results/raw/aks/ai_workloads.csv` | OpenAI chat, OpenAI embedding, and local CPU model all PASS with verified placement tokens. | No model/service confidentiality claim. | Keep AI scope narrow. |
| Multi-node qualification | PASS | `article1/results/raw/aks/multinode_node_selection.csv` | Valid binds; revoked, expired, and no-evidence cases do not bind. | Harness temporarily patches evidence status and restores it. | Keep raw backup paths. |
| Security campaign A1-A10 | PASS | `article1/results/raw/aks/security_attacks_A1_A11.csv` | 300/300 blocked on AKS. | A11 is separate scope control. | Use attack matrix/heatmap. |
| A11 GPU fail-closed control | PASS_SCOPE_CONTROL | `article1/results/raw/aks/a11_gpu_required_no_evidence.csv` | 1/1 denied. | Not confidential-GPU validation. | Keep as negative control. |
| B4 vs B5 TOCTOU | PASS | `article1/results/raw/aks/b4_vs_b5.csv` | B4 median external window 1104.5 ms; B5 removes that externally schedulable window. | Avoid “B5=0 absolute TOCTOU” wording. | Use safe wording. |
| B1-B5 performance | PASS | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | 30 measured runs per baseline; B5 client-observed median 1167.0 ms; B5 scheduler-internal phase median 216.232 ms reported separately; anti-quantization PASS. | Prototype overhead only. | Use table/figure. |
| Scheduler RBAC minimality | PASS_SCOPE | `article1/results/raw/aks/scheduler_security_tests.csv` | RBAC checks pass. | Not a full scheduler compromise proof. | Keep scope. |
| Claim evidence mapping | PASS | `article1/paper/tables/claim_evidence_mapping.csv` | Updated from final AKS raw. | Scope wording must remain disciplined. | Keep synchronized. |

## Current Verdict

`GO_TECHNICAL_FOR_CLAIMED_SCOPE_AFTER_FINAL_COMPILE`

The Article 1 technical package is ready for final LaTeX compile, editorial,
bibliography, IP, and author review. AKS must be deleted after final
compilation/evidence capture to avoid cost.
