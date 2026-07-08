# Unsupported Claims Report — Article 1

Generated: 2026-07-05. Final technical update: 2026-07-06.

## Now Supported by AKS Real Evidence

| claim | environment | raw_file | status |
|---|---|---|---|
| Real AI workloads are governed by attestation-aware placement. | `aks-real-sevsnp` | `article1/results/raw/aks/ai_workloads.csv` | SUPPORTED: 3/3 PASS |
| Multi-node confidential-node qualification was exercised. | `aks-real-sevsnp` | `article1/results/raw/aks/multinode_node_selection.csv` | SUPPORTED: valid/revoked/expired/no-evidence cases |
| A1-A10 adversarial attacks are blocked on AKS. | `aks-real-sevsnp` | `article1/paper/tables/security_attack_matrix.csv` | SUPPORTED: 300/300 |
| B1-B5 performance is measured on AKS. | `aks-real-sevsnp` | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | SUPPORTED: 30 measured runs per baseline |
| Placement-token identity binding is verified. | `aks-real-sevsnp` | `article1/results/raw/aks/identity_binding.csv` | SUPPORTED |
| B4 vs B5 externally schedulable TOCTOU-window reduction is measured. | `aks-real-sevsnp` | `article1/results/raw/aks/b4_vs_b5.csv` | SUPPORTED |

## Still Not Claimed

| claim | reason | required wording |
|---|---|---|
| Pod-level attestation exists. | The AKS evidence is node/VM-level SEV-SNP. | State node-level attested placement only. |
| Model confidentiality or OpenAI service confidentiality is provided. | AI workloads use real APIs/models to test governed placement, not service-side confidentiality. | State AI workload placement and digest binding only. |
| Confidential GPU execution is validated. | A11 is only a fail-closed negative control without GPU evidence. | State no confidential-GPU evaluation. |
| Intel TDX execution is validated. | Final hardware is AMD SEV-SNP DCasv6. | State no TDX evaluation. |
| kind/KWOK results are main security or cloud-performance evidence. | User-required evidence rule and Q1 rigor require AKS real results. | State kind/KWOK are CI/debug/regression or scheduler-only context. |
| Large-scale real AKS throughput is evaluated. | The final AKS campaign uses four confidential nodes, not a large fleet. | Keep scalability out of main cloud-throughput claims. |

## Required Discipline Before Submission

- Do not cite historical failed or pre-fix runs as final evidence.
- Do not describe `runc` as pod-level confidential execution; the claim is
  node-level SEV-SNP CVM placement.
- Do not upgrade A11 into a GPU evaluation claim.
- Do not publish public artifact promises until IP review is complete.
