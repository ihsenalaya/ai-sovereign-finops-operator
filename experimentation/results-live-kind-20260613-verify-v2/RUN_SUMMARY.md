# Live Kind Verification Summary - 2026-06-13

Scope follows `experimentation/prompt.txt`: no invented results, no manual seeding, no AKS, no GPU/vLLM, and no synthetic data presented as measured evidence.

## Command

```bash
BUILD_OPERATOR=true STATUS_REFRESH_WAIT=90 SHADOW_WAIT=20 SHADOW_REFRESH_RETRIES=5 SHADOW_REFRESH_INTERVAL=10 EVIDENCE_DIR=experimentation/results-live-kind-20260613-verify-v2 make -C automatisation real-demo-verify
```

The operator image was rebuilt from the local source and loaded into kind:

```text
ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.7
local image digest: sha256:d815ac6feb96ffc560a5ac2f4b44745fc11420e061341215b5e67e6983ac4a63
```

## Cleanup

The `verify` trap collected evidence, scaled the consumer apps to zero, and deleted the kind cluster.

Final check:

```text
No kind clusters found.
```

## Measured Successes

- Kind cluster `greenops` was created and deleted automatically.
- Operator deployment reached `1/1`.
- Envoy Gateway deployment reached `1/1`.
- Envoy AI Gateway controller reached `1/1`.
- Four bounded consumer apps reached `200` once and then idled:
  - `rh/chatbot-rh` -> `cohere-command-a-latest`
  - `finance/risk-assistant` -> `cohere-command-a-latest`
  - `legal/contract-review` -> `cohere-command-a-latest`
  - `marketing/content-writer` -> `mistral-large-latest`
- Shadow-AI evidence was captured from real Tetragon events:
  - `finance/rogue-script` -> `api.openai.com`
  - `connections=4`
- Envoy AI Gateway exposed real token telemetry by namespace/app/model:
  - `rh/chatbot-rh`: 10 input, 219 output tokens
  - `finance/risk-assistant`: 7 input, 319 output tokens
  - `legal/contract-review`: 10 input, 299 output tokens
  - `marketing/content-writer`: 14 input, 60 output tokens
- `AIFinOpsReport` status reported non-zero measured cost after the precision fix:
  - `totalCostEUR: 7655u`
  - `totalInputTokens: 41`
  - `totalOutputTokens: 897`
- `AISovereigntyPolicy` status reported:
  - `findingsCount: 7`
- Budget statuses remained `WithinBudget`.

## Measured Limits

- The run is a bounded verification run, not a load test.
- It is a local kind validation, not AKS.
- It does not measure GPU/vLLM or self-hosting performance.
- It does not validate OpenAI judged benchmark quality; OpenAI public API smoke previously failed with HTTP 429 in `results-paid-min-20260613`.

## Evidence Files

- `kubectl_aiops.txt`
- `kubectl_pods.txt`
- `yaml/aiops.yaml`
- `metrics/gateway_metrics.txt`
- `shadow-egress.json`
- `logs/*`
