# GPU self-hosting validation plan (PLANNED — RQ6 currently MODELED)

Status: **future work / not yet validated**. RQ6 break-even in the paper is a **modeled** prediction
using declared GPU/ops costs. This plan describes how to replace the model with **measured** GPU
economics. No GPU numbers in the paper are real until this is executed.

## Infrastructure
- **AKS** cluster with a **GPU node pool** (start with **L40S**, then **H100** for a second point).
- **vLLM** serving an open-weights model (e.g., Llama-3-70B / Mistral) via its OpenAI-compatible API
  (drops into `internal/llm` as another provider, `Real: true`, zone FR/EU).
- **DCGM exporter** + Prometheus for GPU utilization/memory/power; node/pod cost from cloud billing.

## Measurements to collect
- **Throughput** (tokens/s) and **goodput** under SLO (TTFT/TPOT), with batching enabled.
- **GPU utilization** and **memory** (DCGM), **power** if available.
- **Latency** p50/p95/p99 under load (load generator), vs managed APIs.
- **Cost**: GPU node-hours × price + ops; derive **EUR/1M tokens** at the measured utilization.

## Break-even validation
- Feed measured self-hosted EUR/1M tokens + fixed monthly cost into the operator `breakevenengine`.
- Compare **predicted** break-even (current modeled curve) vs **measured** → report **prediction
  error** and sensitivity to utilization, batch size, and cache-hit ratio.

## Variables to sweep
- Tokens/day volume; GPU type (L40S/H100); batch size; cache-hit ratio; model size; utilization.

## Honesty
Until executed: RQ6 stays labeled *modeled*; the paper claims only a *prediction framework*, not a
validated break-even. The figure caption and abstract already mark it modeled.
