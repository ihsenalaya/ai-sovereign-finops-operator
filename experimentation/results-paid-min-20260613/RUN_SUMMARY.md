# Minimal Paid/Live Run Summary - 2026-06-13

Scope follows `experimentation/prompt.txt`: do not invent results, do not relabel failures, and keep MEASURED/CACHED/SIMULATED/MODELED separate.

Excluded by instruction:
- AKS cluster: not used.
- GPU/vLLM validation: not run.
- Synthetic or sample data as real evidence: not used.

## OpenAI Smoke

Command:

```bash
go run ./experimentation/cmd/experiment -smoke -results experimentation/results-paid-min-20260613 -key docs/openaikey.txt
```

Result: FAILED.

Observed error:

```text
openai gpt-4o-mini failed after retries: openai gpt-4o-mini: status 429
```

Interpretation: no OpenAI benchmark, judge-vs-truth, or inter-judge run was launched after this failure, to avoid unnecessary paid retries.

Evidence files:
- `TEST_STATUS.md`
- `journal.jsonl`

## Kind Live Demo

Command:

```bash
BUILD_OPERATOR=false RESET_CLUSTER=true STATUS_REFRESH_WAIT=20 SHADOW_WAIT=10 SHADOW_REFRESH_RETRIES=3 SHADOW_REFRESH_INTERVAL=5 make -C automatisation real-demo-reset
```

Result: FAILED at the shadow-AI evidence step.

Real observations before cleanup:
- Azure Foundry provider preflight passed.
- Mistral Foundry preflight passed.
- Kind cluster `greenops` was created.
- Operator image `ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.7` was loaded into kind.
- Operator deployment reached `1/1`.
- Envoy Gateway deployment reached `1/1`.
- Envoy AI Gateway controller reached `1/1`.
- Consumer deployments reached `1/1`: `rh/chatbot-rh`, `finance/risk-assistant`, `legal/contract-review`, `marketing/content-writer`.
- Prometheus and Grafana demo deployments reached `1/1`.
- Tetragon daemonset rolled out.
- CRDs were present for `AIGateway`, `AIProvider`, `AIModel`, `AISovereigntyPolicy`, `AIBudgetPolicy`, and `AIFinOpsReport`.
- Last observed marketing app logs included successful `200` calls to `mistral-large-latest`.

Real negative observations:
- `shadow-egress` remained `[]` after 3 refresh attempts.
- The automation exited with `make: *** [Makefile:25: real-demo-reset] Error 1`.
- Last observed RH app log showed `rh/chatbot-rh(cohere-command-a-latest): 000`; this was not converted into a success claim.
- `aifinopsreport ai-report-all` showed `TOTALCOST(EUR)=0` at the instant collected; this is not enough evidence for a final cost-attribution claim.

Cleanup:
- `kind delete cluster --name greenops` was run.
- `kind get clusters` returned `No kind clusters found.`

Conclusion: this run provides real evidence that the kind-based deployment path reaches a ready control plane and apps, and that Mistral traffic succeeded. It does not provide a successful shadow-AI detection result and does not complete the OpenAI paid-evaluation steps.
