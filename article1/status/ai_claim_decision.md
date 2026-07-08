# AI Claim Decision

Updated: 2026-07-06.

```text
decision = KEEP_AI_WITH_REAL_WORKLOADS
evidence_path = article1/results/raw/aks/ai_workloads.csv
table_path = article1/results/tables/ai_workloads.csv
```

## Decision

The title may keep `AI` only if the AKS campaign includes real AI workloads
governed by `ConfidentialInferencePolicy` and bound to placement evidence.

## Required Evidence

The AI workload evidence must include:

- at least one OpenAI text-generation workload from a governed pod;
- at least one OpenAI embedding workload from a governed pod;
- one local CPU minimal inference workload if feasible without heavy images;
- model/model-identifier digest;
- container image digest as observed by Kubernetes;
- `AIPlacementDecision` status;
- offline `verify-placement` result;
- request count, success count and median latency;
- raw pod logs with no API key material.

## Paper Wording

Allowed:

```text
Confidential AI inference is the motivating use case. Our AKS experiment runs
minimal real AI workloads from attestation-governed pods and binds placement to
model and image digests. We evaluate node-level attested placement and offline
placement verification, not model confidentiality, GPU execution, or large-model
throughput.
```

Forbidden:

- model confidentiality;
- confidential GPU execution;
- OpenAI service confidentiality;
- local LLM confidentiality;
- large-model throughput;
- pod-level attestation.
