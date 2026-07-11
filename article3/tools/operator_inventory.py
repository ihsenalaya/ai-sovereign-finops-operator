#!/usr/bin/env python3
"""Generate source-derived operator inventories and architecture audit."""

from __future__ import annotations

import csv
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
OP = ROOT / "operateur"
OUT = ROOT / "article3" / "operator_audit"


def write_csv(path: Path, fields: list[str], rows: list[dict[str, object]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


def git(*args: str) -> str:
    p = subprocess.run(["git", *args], cwd=ROOT, text=True, stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, check=False)
    return p.stdout.strip()


def crd_inventory() -> list[dict[str, object]]:
    rows = []
    controllers = {p.stem.removesuffix("_controller").replace("_", "").lower(): p
                   for p in (OP / "internal" / "controller").glob("*_controller.go")}
    for path in sorted((OP / "config" / "crd" / "bases").glob("*.yaml")):
        text = path.read_text(encoding="utf-8")
        def get(pattern: str, default: str = "") -> str:
            m = re.search(pattern, text, re.M)
            return m.group(1).strip() if m else default
        kind = get(r"^\s{4}kind:\s*(\S+)")
        plural = get(r"^\s{4}plural:\s*(\S+)")
        scope = get(r"^\s{2}scope:\s*(\S+)")
        versions = ";".join(re.findall(r"^\s{4}- name:\s*(\S+)", text, re.M))
        key = kind.replace("AI", "ai").replace("Raw", "raw").replace("Attestation", "attestation").lower()
        controller = ""
        for normalized, candidate in controllers.items():
            if kind.lower() in candidate.read_text(encoding="utf-8", errors="ignore").lower():
                controller = str(candidate.relative_to(ROOT)); break
        rows.append({"kind": kind, "plural": plural, "scope": scope, "versions": versions,
                     "crd_path": str(path.relative_to(ROOT)), "controller_path": controller,
                     "article3_role": "policy/catalog/aggregate governance" if kind in {
                         "AIGateway", "AIProvider", "AIModel", "AIBudgetPolicy", "AISovereigntyPolicy",
                         "AIQualityGate", "AIRoutingPolicy", "AIRouteOverride", "AIChangeRequest", "AIFinOpsReport"}
                         else "outside core paper; feasibility input only"})
    return rows


def controller_inventory() -> list[dict[str, object]]:
    manager = (OP / "cmd" / "main.go").read_text(encoding="utf-8", errors="ignore")
    rows = []
    for path in sorted((OP / "internal" / "controller").glob("*_controller.go")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        structs = re.findall(r"type\s+(\w+Reconciler)\s+struct", text)
        watches = sorted(set(re.findall(r"For\(&\w+\.(\w+)\{\}", text)))
        owns = sorted(set(re.findall(r"Owns\(&\w+\.(\w+)\{\}", text)))
        requeues = len(re.findall(r"RequeueAfter", text))
        controller = structs[0] if structs else path.stem
        registered = controller in manager
        conditional = controller in {"AttestationEvidenceReconciler", "RawAttestationReportReconciler"}
        rows.append({"controller": controller, "path": str(path.relative_to(ROOT)),
                     "primary_watch": ";".join(watches), "owns": ";".join(owns),
                     "manager_registered": str(registered).lower(),
                     "enabled_by_default": str(registered and not conditional).lower(),
                     "requeue_after_sites": requeues})
    return rows


def test_inventory() -> list[dict[str, object]]:
    rows = []
    for path in sorted(OP.rglob("*_test.go")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        direct = len(re.findall(r"(?m)^func\s+Test\w+", text))
        ginkgo = len(re.findall(r"\bIt\(", text))
        lower = text.lower()
        kind = "unit"
        if "envtest" in lower or "ginkgo" in lower: kind = "envtest/controller"
        if "/e2e" in str(path).replace("\\", "/"): kind = "e2e scaffold"
        rows.append({"path": str(path.relative_to(ROOT)), "test_functions": direct,
                     "ginkgo_cases": ginkgo, "category": kind, "executed_in_current_audit": "false"})
    return rows


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    crds = crd_inventory()
    controllers = controller_inventory()
    tests = test_inventory()
    write_csv(OUT / "crd_inventory.csv", list(crds[0]), crds)
    write_csv(OUT / "controller_inventory.csv", list(controllers[0]), controllers)
    write_csv(OUT / "test_inventory.csv", list(tests[0]), tests)

    enabled = sum(r["enabled_by_default"] == "true" for r in controllers)
    direct_tests = sum(int(r["test_functions"]) for r in tests)
    ginkgo = sum(int(r["ginkgo_cases"]) for r in tests)
    base = git("rev-parse", "origin/main")
    tags = git("tag", "--sort=-version:refname").splitlines()

    architecture = f"""# Operator architecture audit

This audit is generated from the recovery branch source by `article3/tools/operator_inventory.py`. Inventories contain {len(crds)} CRDs, {len(controllers)} controller files/manager registrations, and {len(tests)} Go test files. {enabled} controllers are normally enabled; the two attestation evidence/report reconcilers are conditional to preserve the dedicated verifier's single-writer role.

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
"""
    (OUT / "architecture.md").write_text(architecture, encoding="utf-8")

    telemetry = """# Telemetry, cost, budget, and routing flow

| Stage | Source | Identity/granularity | Current limitation for GOV-AR |
|---|---|---|---|
| request decoration | `internal/sidecarproxy`, pod injector | workload headers | caller annotations are unauthenticated; HTTPS CONNECT bypasses |
| route selection | Envoy AI Gateway route | model header/backend | model resource name and `spec.modelName` can differ |
| token telemetry | Envoy `gen_ai_*` histograms | aggregate label series | no request ID, errors, exact tail latency, or settlement event |
| collection | Prometheus collectors | cumulative process counters | time-window argument is ignored |
| price calculation | `internal/costengine` | model aggregate | fixed book/FX; pricing version not propagated |
| budget control | AIBudgetPolicy controller | aggregate settled projection | in-flight liabilities absent from established path |
| recommendation | AIRoutingPolicy/FinOps engines | policy/workload aggregate | objective does not drive GOV-AR; recommendation is not admission |
| actuation | route actuator, sovereignty/budget/override controllers | route rule | override/change controllers can pass backend where model is expected |
| GOV-AR settlement | sidecar `/v1/settle` | request ID | current proxy sends zero cost and ignores settlement failures |

The Article 3 implementation must add request/workload UID identity, immutable pricing snapshot, known input usage, probabilistic/strict output liability, transactional lifecycle/event log, and aggregate reconciliation without treating aggregate Prometheus counters as request settlement.
"""
    (OUT / "telemetry_and_cost_flow.md").write_text(telemetry, encoding="utf-8")

    version = f"""# Version audit

- Fetched remote base: `origin/main` = `{base}`.
- Operator chart and app version at that SHA: 0.5.11.
- Umbrella chart and app version at that SHA: 0.5.11.
- Remote controller 0.5.11 resolves as index digest `sha256:abc591624156aa2a6dc958221d3c0968a1fed11475d6a8e2b38c6b6644a62e88`.
- OCI umbrella chart 0.5.11 pulls and its `.tgz` SHA-256 is `7a21d883b7725b8321af24ad612078d6f952712134697f362a768f966d795db9`.
- Fetched tags: {', '.join(tags[:8])}. No fetched v0.5.11 tag exists; the coherent remote release is untagged.
- Local `main` is not selected: its operator subchart reached 0.5.17 while the umbrella chart/image/README remained 0.5.11.
- GOV-AR is branch-local, introduced principally in commit `51f1124`, with no chart/app version bump and no published `gov-ar-admission:0.5.11` image.

The experimental release must use a new SemVer prerelease and immutable commit/run tag; it must not overwrite or claim operator 0.5.11 artifacts.
"""
    (OUT / "version_audit.md").write_text(version, encoding="utf-8")

    reuse = """# Reuse and change plan

## Reuse after regression testing

- provider/model/budget/routing/sovereignty/quality/change-request CRDs and generated schemas;
- catalog resolution and Kubernetes quantity representations;
- pure cost/sovereignty engines and Prometheus registration patterns;
- Envoy metric parser for aggregate reconciliation, not request settlement;
- route mutation logic after fixing controller call sites;
- envtest harness, webhook framework, chart security contexts, and generated CRDs.

## Replace

- in-memory and PostgreSQL GOV-AR ledgers/schema;
- sidecar pricing/settlement behavior and unauthenticated API surface;
- candidate feasibility and name-to-route resolution;
- budget-window accounting and price-version handling;
- queue/approval/readiness/expiry semantics;
- Article 3 experiment engine, raw schema, statistics, and release declarations.

## Extend

- deployment routability/availability, provider readiness, quality-gate linkage, policy versioning;
- authenticated workload UID/tenant identity and authorization;
- atomic event-sourced reserve–dispatch–settle/cancel/expire/late-settle state;
- Envoy-compatible synchronous admission with retries and structured traces;
- migrations, PostgreSQL packaging, NetworkPolicy, least-privilege RBAC, metrics and diagnostics;
- immutable image/chart CI and clean install/upgrade/rollback/uninstall validation.
"""
    (OUT / "reuse_and_change_plan.md").write_text(reuse, encoding="utf-8")

    risks = [
        ("R01", "critical", "Same request can be settled twice with distinct settlement IDs", "budget corruption", "unique effective settlement plus event idempotency, concurrency regression"),
        ("R02", "critical", "Proxy settles actual_cost=0", "all live accounting invalid", "price provider usage using immutable pricing snapshot"),
        ("R03", "critical", "Admission and settlement are unauthenticated; tenant/policy are caller-controlled", "cross-tenant leakage/bypass", "authenticated workload UID binding and authorization"),
        ("R04", "critical", "HTTPS CONNECT and webhook fail-open bypass admission", "unenforced governed path", "native Envoy-compatible synchronous path and fail-closed policy"),
        ("R05", "major", "No dispatch/expiry/late-settle/window lifecycle", "stuck or prematurely released liability", "explicit state machine, worker, compensation"),
        ("R06", "major", "DOUBLE PRECISION money and weak DB constraints", "rounding/negative/inconsistent ledger", "integer minor units or decimal numeric, constraints, balance tests"),
        ("R07", "major", "Readiness ignores PostgreSQL/Kubernetes and production falls back to memory", "silent non-durable operation", "fail readiness/startup when durable dependencies missing"),
        ("R08", "major", "Feasibility ignores readiness, quality, approval, sovereignty and routability", "policy-ineligible route", "hard source-of-truth filter with reason codes"),
        ("R09", "major", "Model resource name may not match Envoy model name", "failed or wrong route", "validated deployment identity mapping"),
        ("R10", "major", "Aggregate Prometheus counters are treated as budget windows", "incorrect daily/weekly/monthly spend", "request ledger windows plus reconciler"),
        ("R11", "major", "Release workflow omits GOV-AR, verifier and node-agent images", "unreproducible chart", "build/push all referenced images and resolve digests"),
        ("R12", "major", "No real PostgreSQL/gateway/race/fault/upgrade tests", "measured-path bugs", "complete D/E test matrix before pilot"),
    ]
    write_csv(OUT / "risk_register.csv", ["risk_id", "severity", "risk", "impact", "required_control"],
              [dict(zip(["risk_id", "severity", "risk", "impact", "required_control"], r)) for r in risks])
    risk_md = "# Operator and GOV-AR risk register\n\n" + "\n".join(
        f"- **{a} ({b})** — {c}. Impact: {d}. Required control: {e}." for a,b,c,d,e in risks) + "\n"
    (OUT / "risk_register.md").write_text(risk_md, encoding="utf-8")

    test_md = f"""# Test audit summary

Static inventory contains {len(tests)} `_test.go` files, {direct_tests} direct `Test*` functions, and {ginkgo} Ginkgo `It` cases. This generator does not mark any current pass; execution status belongs only in machine-readable test results produced by the final test matrix.

Missing measured-path coverage at takeover: real PostgreSQL integration; race/concurrent duplicate reserve/settle; duplicate distinct-ID settlement; expiry/late settlement; gateway→provider→usage→settlement; fault campaign; multi-replica settlement; Kind Helm upgrade/rollback/uninstall; streaming/timeouts/429s; native Envoy admission.
"""
    (OUT / "test_inventory.md").write_text(test_md, encoding="utf-8")


if __name__ == "__main__":
    main()
