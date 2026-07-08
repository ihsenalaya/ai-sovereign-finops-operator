# Figure and Table Provenance

| Artifact | Raw input | Generator |
|---|---|---|
| `article1/paper/tables/security_attack_matrix.csv` | `article1/results/raw/aks/security_attacks_A1_A11.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/paper/figures/security_attack_heatmap.pdf` | `article1/paper/tables/security_attack_matrix.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/paper/tables/b4_b5_comparison.csv` | `article1/results/raw/aks/b4_vs_b5.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/results/tables/performance.csv` | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | `article1/experiments/performance/run_aks_b1_b5_latency.sh` |
| `article1/paper/figures/scheduling_latency_cdf.pdf` | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | `article1/experiments/performance/run_aks_b1_b5_latency.sh` |
| `article1/results/tables/ai_workloads.csv` | `article1/results/raw/aks/ai_workloads.csv` | `article1/experiments/ai-workload/run_ai_workloads_aks.sh` |
| `article1/results/tables/multinode_node_selection.csv` | `article1/results/raw/aks/multinode_node_selection.csv` | `article1/experiments/aks/run_multinode_node_selection.sh` |

Scope rule: AKS raw data are used for paper security/performance/AI evidence.
kind and KWOK artifacts are retained only for CI, debug, regression, or
scheduler-only scalability context.
