# Q1 Execution Inventory — Article 1

Generated: 2026-07-04. Final technical update: 2026-07-06.

Main-paper empirical evidence is AKS-only. `kind` and KWOK artifacts remain CI,
debug, regression, or scheduler-only context; they must not be presented as
Article 1 cloud security/performance results.

## Repository State

- Branch: `main`.
- Container registry policy: GHCR only, tag `0.5.11`; do not reintroduce ACR.
- Final AKS target: `rg-article1-confidential` / `aks-article1`, `westus2`.
- Confidential node pool: `Standard_DC8as_v6`, four active nodes during the
  final multi-node and B1-B5 performance campaigns under a 32-vCPU DCasv6 quota.

## AKS-Backed Evidence

- Real multi-node SEV-SNP attestation:
  `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`.
- Governed AI workloads:
  `article1/results/raw/aks/ai_workloads.csv`, 3/3 PASS.
- Multi-node node qualification:
  `article1/results/raw/aks/multinode_node_selection.csv`, 4/4 expected outcomes.
- Security campaign:
  `article1/results/raw/aks/security_attacks_A1_A11.csv`; A1-A10 are 300/300
  blocked, A11 is 1/1 fail-closed GPU-scope control.
- B1-B5 performance:
  `article1/results/raw/aks/performance_b1_b5_high_resolution.csv`; 30 measured
  runs per baseline, B5 client-observed median 1167.0 ms,
  B5 scheduler-internal phase median 216.232 ms reported separately,
  anti-quantization PASS.
- B4 vs B5:
  `article1/results/raw/aks/b4_vs_b5.csv`; B4 median external window 1104.5 ms,
  B5 removes that externally schedulable window.
- Identity binding:
  `article1/results/raw/aks/identity_binding.csv`.
- Scheduler RBAC minimality:
  `article1/results/raw/aks/scheduler_security_tests.csv`.

## Scope Boundaries

- Claimed: node-level AMD SEV-SNP attested placement, AI workload placement, and
  offline-verifiable placement decisions.
- Not claimed: pod-level attestation, model confidentiality, OpenAI service
  confidentiality, confidential GPU execution, Intel TDX execution, or
  large-scale real AKS throughput.

## Remaining Non-Technical Work

- Final LaTeX compile and editorial polish.
- Bibliography consistency and venue formatting.
- IP/private artifact review.
- Author approval before submission.
