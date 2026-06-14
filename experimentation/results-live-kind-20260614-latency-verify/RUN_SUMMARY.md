# Live kind latency verification - 2026-06-14

Command:

```bash
BUILD_OPERATOR=false EVIDENCE_DIR=experimentation/results-live-kind-20260614-latency-verify make -C automatisation real-demo-verify
```

Result: `status=0`.

Key facts:

- Operator image: `ghcr.io/ihsenalaya/ai-sovereign-finops-operator:0.3.8`
- Kind cluster: created by the automation and deleted on exit.
- Apps: `rh/chatbot-rh`, `finance/risk-assistant`, `legal/contract-review`, `marketing/content-writer`.
- Shadow-AI: `finance/rogue-script` produced 6 observed connections to `api.openai.com`.
- Gateway latency source: real Envoy AI Gateway histogram `gen_ai_server_request_duration_seconds`.
- Report status: `latencyTelemetryAvailable=true`.
- Operator metrics present: `ai_finops_latency_score`, `ai_finops_measured_latency_millis`, `ai_finops_latency_telemetry_available`, `ai_finops_routing_score`.

Observed routing scores from `yaml/aiops.yaml`:

| Namespace | Application | Model | Score | Latency score | Observed latency ms | Source |
|---|---|---|---:|---:|---:|---|
| finance | risk-assistant | cohere-command-a-latest | 0.412 | 0.058 | 10247.126 | observed |
| legal | contract-review | cohere-command-a-latest | 0.428 | 0.000 | 10813.706 | observed |
| marketing | content-writer | mistral-large-latest | 1.000 | 1.000 | 965.029 | observed |
| rh | chatbot-rh | cohere-command-a-latest | 0.632 | 0.448 | 6402.542 | observed |

Evidence files:

- `metrics/gateway_metrics.txt`: gateway token and duration histograms.
- `metrics/operator_metrics.txt`: operator `ai_finops_*` metrics, including latency/routing score gauges.
- `yaml/aiops.yaml`: CR status snapshot with routing scores and latency telemetry availability.
- `shadow-egress.json`: shadow-AI egress captured from Tetragon.
