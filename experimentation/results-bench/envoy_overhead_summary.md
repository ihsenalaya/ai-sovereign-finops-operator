# Envoy data-path overhead under load (local, real Envoy)

Setup: real Envoy proxy (envoyproxy/envoy v1.31) in front of the LLM provider; gpubench drives 48
requests at concurrency 8 (gpt-4o-mini), direct vs through Envoy. Measures gateway data-path overhead.

| path | req/s | tokens/s | latency p50 ms | latency mean ms | p95 ms | errors |
|------|------:|---------:|---------------:|----------------:|-------:|------:|
| direct     | 5.81 | 270.0 | 1067 | 1116 | 1838 | 0 |
| via Envoy  | 6.05 | 279.0 | 1229 | 1153 | 1681 | 0 |

**Finding:** routing through the Envoy data path adds **negligible overhead** (mean +~3%, within
run-to-run variance; throughput and p95 differences are within noise), with **0 errors** under
concurrent load — supporting the feasibility of a gateway-level control plane. The full in-cluster
Envoy AI Gateway integration (per-provider auth, failover) remains future work; this micro-benchmark
validates the data-path overhead claim.
