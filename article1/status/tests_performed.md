# Tests Performed — Article 1

Generated: 2026-07-05. Final technical update: 2026-07-06.

Evidence rule: main-paper empirical results come from real AKS SEV-SNP.
`kind` and KWOK artifacts are CI/debug/regression or scheduler-only context and
must not be cited as Article 1 cloud security/performance evidence.

## Environments

- `local`: Go unit/envtest.
- `kind-live-simulated`: local regression cluster only.
- `aks-real-sevsnp`: real AKS on `westus2` `Standard_DC8as_v6`.

## 1. Go Unit and Envtest

The codebase includes unit/envtest coverage for scheduler filtering, MAA
verification, placement tokens, canonical hashing, admission webhook, CRD
validation, and controller logic. Representative tests cover expired evidence,
revoked evidence, wrong TEE, PreBind policy/revocation races, placement-token
tampering, and simulated-runtime rejection.

Final local validation commands run on 2026-07-06:

- `make test-unit`: PASS.
- `go test -race ./internal/scheduler ./pkg/token ./pkg/crypto ./pkg/audit -count=1 -timeout 120s`: PASS.
- `make lint`: PASS.
- `make helm-lint`: PASS.
- `make helm-template-kind`: PASS.
- `make helm-template-aks-private`: PASS.

`go test ./...` is not used as Article 1 evidence. It reaches the Kubebuilder
E2E package and requires a local `kind` cluster named `kind`; after creating
that regression cluster, the run was blocked by Docker Desktop/WSL instability
(`error getting credentials`, then Kubernetes scheduler leader-election
timeouts). This is a local CI/regression environment issue, not an AKS
evaluation result.

## 2. AKS Real Attestation and Multi-Node Qualification

Raw files:

- `article1/results/raw/aks/multinode-node-selection/evidence-initial.json`
- `article1/results/raw/aks/multinode_node_selection.csv`

The final AKS multi-node snapshot contains four active
`Standard_DC8as_v6` confidential nodes with real verified SEV-SNP evidence. The
qualification test records four expected outcomes: valid evidence binds with a
verified token; revoked evidence does not bind; expired evidence does not bind;
and a non-confidential system node selected by `nodeSelector` but lacking
`AttestationEvidence` does not bind.

## 3. Governed AI Workloads

Raw file: `article1/results/raw/aks/ai_workloads.csv`.

| Workload | Result |
|---|---|
| `openai-chat-minimal` | PASS, 3/3 requests, placement token verified |
| `openai-embedding-minimal` | PASS, 3/3 requests, placement token verified |
| `local-cpu-minimal` | PASS, 5/5 local model requests, placement token verified |

These support the AI placement claim only: no model confidentiality, OpenAI
service confidentiality, confidential GPU, or large-model throughput is claimed.

## 4. AKS Real Security Campaign — A1-A11

Raw file: `article1/results/raw/aks/security_attacks_A1_A11.csv`.
Summary table: `article1/paper/tables/security_attack_matrix.csv`.

| Attack | AKS result |
|---|---|
| A1 label forgery | 30/30 blocked |
| A2 expired evidence | 30/30 blocked |
| A3 revoked evidence | 30/30 blocked |
| A4 wrong TEE | 30/30 blocked |
| A5 forbidden runtime | 30/30 blocked |
| A5b missing model digest | 30/30 blocked |
| A6 policy race | 30/30 blocked |
| A7 revocation race | 30/30 blocked |
| A8 token tamper | 30/30 blocked |
| A9 simulated runtime in production | 30/30 blocked |
| A10 forged appraised evidence | 30/30 blocked |
| A11 GPU evidence unavailable | 1/1 denied as a scope-control |

## 5. B4 vs B5 Baseline

Raw file: `article1/results/raw/aks/b4_vs_b5.csv`.

B4, implemented with scheduling gates and an external verifier, leaves a
gate-removal-to-bind window with median 1104.5 ms. B5 performs the check in the
scheduler PreBind path, removes that externally schedulable window, and emits an
offline-verifiable placement token. The summary statistics are in
`article1/results/tables/b4_vs_b5_stats.csv`.

## 6. B1-B5 Performance

Raw file: `article1/results/raw/aks/performance_b1_b5_high_resolution.csv`.
Summary table: `article1/results/tables/performance.csv`.

The final AKS performance run contains two warmups and 30 measured runs per
baseline. All 150 measured pods succeed. The anti-quantization check is PASS.
The B1--B5 table and CDF use the same client-observed scheduling-path metric
for every baseline; under that metric B5 median latency is 1167.0 ms. B5 also
records a scheduler-internal phase median of 216.232 ms, reported separately and
not mixed into the B1--B5 latency axis.

## 7. Identity Binding and Scheduler RBAC Minimality

- `article1/results/raw/aks/identity_binding.csv`: correct binding verifies;
  bound-field mutations fail.
- `article1/results/raw/aks/scheduler_security_tests.csv`: RBAC minimality
  checks pass for required/forbidden operations. This is not claimed as a full
  scheduler compromise proof.

## 8. Not Claimed

- Pod-level attestation.
- Model confidentiality or OpenAI service-side confidentiality.
- Confidential GPU execution.
- Intel TDX execution.
- Large-scale real AKS throughput beyond the four-node Article 1 campaign.
