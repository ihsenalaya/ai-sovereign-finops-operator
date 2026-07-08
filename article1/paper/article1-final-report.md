# Article 1 Final Technical Report

Updated: 2026-07-06.

Main-paper empirical claims now come from real AKS SEV-SNP only. `kind` and
KWOK artifacts remain CI/debug/regression artifacts and are not used as cloud
security or cloud performance evidence.

## AKS Results

- Real node-level SEV-SNP attestation on AKS westus2 `Standard_DC8as_v6`:
  `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`
  records four active confidential nodes with real verified evidence.
- Governed AI workloads:
  `article1/results/raw/aks/ai_workloads.csv`, 3/3 PASS with placement-token
  verification.
- Multi-node node qualification:
  `article1/results/raw/aks/multinode_node_selection.csv`, valid/revoked/
  expired/no-evidence outcomes as expected.
- Security campaign:
  `article1/results/raw/aks/security_attacks_A1_A11.csv`; A1-A10 are 300/300
  blocked and A11 is 1/1 fail-closed GPU-scope.
- B1-B5 performance:
  `article1/results/raw/aks/performance_b1_b5_high_resolution.csv`, 30 measured
  runs per baseline, B5 client-observed median 1167.0 ms; B5
  scheduler-internal phase median 216.232 ms is reported separately.
- B4 vs B5:
  `article1/results/raw/aks/b4_vs_b5.csv`; B4 median external gate-to-bind
  window 1104.5 ms; B5 removes that window from the schedulable path and emits
  an offline-verifiable placement token.
- Identity binding:
  `article1/results/raw/aks/identity_binding.csv`, expected PASS/FAIL outcomes.

## Scope

The paper claims node-level AMD SEV-SNP attested placement, governed AI workload
placement, and verifiable placement decisions. It does not claim pod-level
attestation, model confidentiality, OpenAI service confidentiality,
confidential GPU evaluation, Intel TDX evaluation, or large-scale real AKS
throughput.

## Verdict

Technical evidence is complete for the claimed Article 1 scope. Remaining work
before submission is final LaTeX compile, editorial polish, bibliography
consistency, IP review, and author approval.
