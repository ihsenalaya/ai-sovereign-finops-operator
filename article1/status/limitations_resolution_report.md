# Limitations Resolution Report — Article 1

Generated: 2026-07-05. Final technical update: 2026-07-06.

## L1 — Node-Level, Not Pod-Level

Status: `EXPLAINED_NOT_RESOLVED`

AKS DCasv6 provides confidential VM isolation for the worker node. The evidence
attests the node/VM, not a per-pod TEE. The manuscript must keep node-level
language and must not claim Kata/CoCo pod-level attestation.

## L2 — AI Workload Scope

Status: `RESOLVED_FOR_PLACEMENT_SCOPE`

The final AKS campaign includes real governed AI workloads in
`article1/results/raw/aks/ai_workloads.csv`. This supports AI placement and
digest-binding claims only. It does not establish model confidentiality, OpenAI
service confidentiality, confidential GPU execution, or large-model throughput.

## L3 — No GPU / No TDX

Status: `RESOLVED_BY_SCOPE`

A11 proves safe denial when `requireConfidentialGPU=true` but no GPU evidence
path exists. It does not validate confidential GPU execution. Intel TDX is also
outside the evaluated hardware scope.

## L4 — Latency

Status: `RESOLVED_FOR_CLAIMED_SCOPE`

The final AKS B1-B5 run produced 30 measured observations per baseline in
`article1/results/raw/aks/performance_b1_b5_high_resolution.csv`. The B1-B5
table and CDF use one client-observed scheduling-path instrument; B5 median is
1167.0 ms under that instrument. B5 scheduler-internal phase median latency is
216.232 ms and is reported separately, not mixed with B1-B4. The manuscript may
claim prototype scheduler-path overhead for this workload, not universal
Kubernetes latency or broad cloud-scale throughput.

## L5 — Scale

Status: `PARTIALLY_RESOLVED_FOR_NODE_QUALIFICATION`

The final AKS campaign used four active confidential nodes for multi-node
qualification and B1-B5 performance. This is not a large-scale throughput study.
kind/KWOK may support CI/regression/scheduler-only context but must not be
described as real AKS cloud performance.

## L6 — MAA Trust Anchor

Status: `RESOLVED_FOR_CLAIMED_SCOPE`

The AKS evidence snapshots confirm MAA-backed SEV-SNP evidence, and A10 validates
the trust-chain separation between node-agent raw reports and central-verifier
appraised evidence. The paper should avoid claiming a formal RATS proof beyond
the implemented verifier checks.

## L7 — IP / Private Artifact

Status: `EXPLAINED_NOT_RESOLVED`

The placement-token and signed-decision mechanisms remain IP-sensitive. The
paper should promise reproducibility through raw traces and scripts in a private
review package unless the authors approve public release.

## L8 — DCASv5 / DCasv6 Consistency

Status: `RESOLVED`

The final Article 1 target is westus2 `Standard_DC8as_v6`. DCASv5/eastus2 is
historical diagnostic material only and must not be used as final evaluation
context.

## Overall

The technical blockers identified in the 2026-07-05 audit are resolved for the
claimed scope. Remaining risks are editorial precision, bibliography/IP review,
final PDF compilation, and author approval.
