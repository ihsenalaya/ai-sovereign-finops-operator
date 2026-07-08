# Limitations Resolution Matrix — Article 1

Generated: 2026-07-05. Final technical update: 2026-07-06.

| id | limitation | status | raw_evidence | resolution | remaining_risk | impact_on_q1 |
|---|---|---|---|---|---|---|
| L1 | Node-level, not pod-level | EXPLAINED_NOT_RESOLVED | `article1/results/raw/aks/aks-nodepool-conf-multinode-scale-request.json` | The article claims node-level AKS SEV-SNP placement only. Pod-level Kata/CoCo attestation is not claimed. | Reviewer wording must stay precise: node/VM evidence, not per-pod evidence. | low |
| L2 | AI workload absent | RESOLVED_FOR_PLACEMENT_SCOPE | `article1/results/raw/aks/ai_workloads.csv` | Real governed AI workloads run and verify placement tokens. | No model/OpenAI service confidentiality claim. | low |
| L3 | No confidential GPU / no TDX | RESOLVED_BY_SCOPE | `article1/results/raw/aks/a11_gpu_required_no_evidence.csv` | GPU and TDX execution are out of scope; A11 verifies fail-closed behavior for GPU-required policy without evidence. | A11 must not be described as GPU validation. | low |
| L4 | B1-B5 latency unavailable | RESOLVED_FOR_CLAIMED_SCOPE | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | Final AKS run produced 30 measured runs per baseline; B5 client-observed median 1167.0 ms; B5 scheduler-internal phase median 216.232 ms reported separately. | Do not overclaim universal latency or cloud throughput. | low |
| L5 | Single-node scheduling | RESOLVED_FOR_NODE_QUALIFICATION | `article1/results/raw/aks/multinode_node_selection.csv` | Four-node AKS snapshot plus valid/revoked/expired/no-evidence outcomes. | Not a large-fleet scale study. | low |
| L6 | Scale | EXPLAINED_NOT_MAIN | `article1/results/raw/kind/`, `article1/results/raw/kwok/` | kind/KWOK are retained as CI/regression/scheduler-only signals. | No real AKS large-scale throughput claim. | medium |
| L7 | MAA trust anchor | RESOLVED_FOR_CLAIMED_SCOPE | `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`, `article1/results/raw/aks/security_attacks_A1_A11.csv` | Real evidence exists and A10 demonstrates trust-chain separation. | Full RATS/MAA formal validation is not claimed beyond implemented checks. | low |
| L8 | IP/private artifact | EXPLAINED_NOT_RESOLVED | `article1/status/ip_artifact_strategy.md` | Keep artifact private/anonymized until IP review. | Public artifact promises need author approval. | medium |
| L9 | DCASv5/DCasv6 consistency | RESOLVED | `article1/results/raw/aks/aks-nodepool-conf-multinode-scale-request.json` | Final Article 1 evaluation uses westus2 `Standard_DC8as_v6`; DCASv5/eastus2 is historical diagnostics only. | Historical logs may mention older quota paths. | low |

## Exit Criterion

No technical blocker remains for the claimed Article 1 scope. Remaining work is
final compile, editorial review, bibliography/IP review, and author approval.
