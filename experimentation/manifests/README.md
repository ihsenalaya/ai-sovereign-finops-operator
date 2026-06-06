# Live-platform integration (manifests)

The core experiments run as a self-contained Go harness (control-plane-level routing with real LLM
calls), which is sufficient for RQ1–RQ6 on a local machine. This folder documents the **live,
in-cluster** platform for the camera-ready / Azure phase, where routing is enforced in the **Envoy AI
Gateway data path** and metrics come from the deployed operator.

## Target topology
```
client workloads -> Envoy AI Gateway -> {OpenAI, provider#2, self-hosted vLLM}
                          ^
                          | xDS / ext_proc routing decisions
        AI Sovereign FinOps Operator (Cost/Budget/Sovereignty/Break-even/Report engines)
                          |
                  Prometheus + Grafana (ai_finops_* metrics, dashboards/)
```

## How to bring it up
- Operator + CRDs + RBAC: Helm chart in `../../charts/ai-sovereign-finops-operator` or the end-to-end
  GitOps flow in `../../automatisation` (kind + ArgoCD).
- Envoy AI Gateway: install the Envoy Gateway + AI Gateway extension (CNCF), define an `AIGateway` CR
  pointing at it (`telemetry.mode: prometheus`).
- Prometheus/Grafana: scrape the operator `/metrics`; import `../../dashboards/ai-finops-overview.json`.
- Collect live metrics into the analysis with `../scripts/collect_metrics.sh`.

## Deferred (Azure phase)
- Real self-hosted **vLLM on GPU** (replaces the modeled self-hosted entry in `internal/catalog`).
- Data-path enforcement latency of routing decisions inside Envoy (vs the control-plane decision time
  measured today, ~µs).
