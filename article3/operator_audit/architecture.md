# Operator architecture audit

This audit is generated from the recovery branch source by `article3/tools/operator_inventory.py`. Inventories contain 18 CRDs, 17 controller files/manager registrations, and 40 Go test files. 15 controllers are normally enabled; the two attestation evidence/report reconcilers are conditional to preserve the dedicated verifier's single-writer role.

## Established request and control path

1. The mutating webhook (`internal/webhook/podinjector`) injects `greenops-header-proxy` and `HTTP_PROXY` for annotated workloads.
2. The sidecar adds namespace/application headers; HTTPS `CONNECT` is tunnelled and therefore bypasses request inspection.
3. Envoy AI Gateway routes from `x-ai-eg-model` to a backend.
4. Envoy `gen_ai_*` Prometheus histograms expose aggregate token and latency telemetry.
5. Prometheus collectors poll cumulative counters; requested daily/weekly/monthly ranges are not implemented as real sliding windows.
6. The collector maps model traffic to workload/catalog dimensions. `internal/costengine` combines tokens with a 15-model price book dated 2026-01 and a fixed USD→EUR factor, but the price date is not propagated into cost records.
7. Budget, sovereignty, FinOps, quality, routing, override, and change-request controllers update aggregate status/metrics and may patch `AIGatewayRoute` rules.

This established path has no request-identified cost settlement. “OpenTelemetry” in the existing documentation refers to Envoy metrics; the operator has no end-to-end OpenTelemetry trace path.

## Branch-local GOV-AR scaffold

The Article 3 branch adds `cmd/gov-ar-admission`, `internal/govar`, chart templates, and sidecar calls. The current sequence is sidecar `/v1/admit` → Kubernetes snapshot → partial feasibility filter → cheapest strict-max candidate → in-memory/PostgreSQL reservation → `x-ai-eg-model` → buffered provider response → `/v1/settle`.

It is not the requested algorithm or measured data path. The proxy submits `actual_cost: 0`; the database does not persist provider usage or price it. There is no dispatch transition, expiry worker, late-settlement compensation, window renewal, adaptive quantile, tenant risk allocation, drift fallback, durable queue, approval lookup, custom GOV-AR metrics, or trace propagation.

## Security and enforcement boundary

Tenant, sensitivity, zones, and budget-policy name are caller/workload annotations without authentication. Admission/settlement/cancel/liability endpoints are unauthenticated. The webhook failure policy is `Ignore`; only `HTTP_PROXY` is injected; HTTPS tunnelling bypasses GOV-AR. The service reads catalog/policy objects from the workload namespace although existing demos centralize them in `default`. These are enforcement failures, not manuscript “limitations,” and must be fixed before E0.

## Reproduction

```bash
find operateur/config/crd/bases -maxdepth 1 -name '*.yaml' | wc -l
find operateur/internal/controller -maxdepth 1 -name '*_controller.go' | wc -l
rg -n 'SetupWithManager' operateur/cmd/main.go
rg --files operateur -g '*_test.go' | wc -l
rg -n '^func Test' operateur -g '*_test.go' | wc -l
```
