# Experiment Report

Status as of 2026-07-08 UTC: partial live AKS evidence collected; claim set not yet finalized.

## Current Live Run

- AKS cluster `aks-article2-260708` was rebuilt and the operator, Envoy Gateway, Envoy AI Gateway, provider catalog, demo workloads, budget policies, sovereignty policy, and quality gates were deployed on the real cluster.
- Operator deployment is healthy in `greenops-system`; consumer workloads now run with the GHCR-backed sidecar image after GHCR pull secrets were propagated to the demo namespaces.
- Live evidence has been captured under `article2/experiments/evidence/smoke/` and `article2/experiments/runs/`.

## RQ Status

- RQ1 / E1 FinOps attribution: `PARTIALLY SUPPORTED`
  Live `AIFinOpsReport` objects contain non-zero cost, per-app attribution, top models, routing scores, and sovereignty recommendations on real AKS gateway traffic. Raw gateway metrics were captured independently in the experiment evidence. A stricter attribution-error comparison against independently recomputed ground truth is still pending before this can become a full paper claim.

- RQ2 / E2 Routing trade-offs: `BLOCKED BY LIVE AZURE RATE LIMIT`
  A new live run now exists at `article2/experiments/runs/E2_routing_tradeoffs/aks-live-20260708T185351Z/` and captured 512 real gateway requests across B1-B8 after we repaired stale OpenAI FR/US backend hostnames and refreshed the stale Kubernetes API-key secrets. This unblocked the gateway path itself, but the scientific comparison is still not claim-safe: 441 of the 512 requests failed with upstream `429` rate-limit responses, overwhelmingly on `gpt-france-mini` in `francecentral`, so the observed exact-match, latency, and cost deltas are dominated by live provider throttling rather than stable baseline behavior. B9 also still leans on the separate E4 hard-limit transition artifact because the current catalog does not yet provide a truthful managed fallback/block path for a full live routing matrix.

- RQ3 / E3 Quality gate: `SUPPORTED FOR CURRENT FOUR-APPLICATION SCOPE`
  The 2026-07-08 AKS quality campaign now satisfies the prompt threshold for E3 on real infrastructure. The run at `article2/experiments/runs/E3_quality_gate/aks-live-20260708T215509Z/` used four application-specific golden datasets with 20 curated prompts each, enforced `minSamples=20`, and executed five repetitions per gate. The resulting stability summary is mixed but scientifically usable: `finance-risk-assistant-quality` was `stable-safe` with mean score `73.219` and standard deviation `1.017`; `marketing-content-quality` was `stable-safe` with mean `73.832` and standard deviation `0.418`; `rh-chatbot-quality` was `stable-risk` with mean `69.495` and standard deviation `0.345`; and `legal-contract-quality` was explicitly `unstable`, producing one `candidate-risk` verdict followed by four `candidate-safe` verdicts with mean `66.013` and standard deviation `1.534`. Two auxiliary gates remained `Pending` / `insufficient-data`: one because `spec.evidenceRef` was intentionally omitted, and one because the candidate lacked evidence, telemetry, and zone compliance. This moves E3 beyond a controller smoke test into a measured stability study, while still leaving external validity limited to four applications on one cluster day.
  Detailed extracted table: `article2/experiments/runs/E3_quality_gate/aks-live-20260708T215509Z/processed/e3_qualitygate_detailed.tex`.

- RQ4 / E4 Budget enforcement: `PARTIALLY SUPPORTED`
  Live budget accounting is working on AKS and we now captured the full `Warning -> Critical -> Exceeded` phase progression for `finance-budget`. The progression was induced by controlled threshold and budget tightening against a real observed spend of 0.017104 EUR, yielding `Critical` at 86% of a 0.02 EUR budget and `Exceeded` at 101% after temporarily lowering the budget to 0.017 EUR. This supports live phase-transition behavior, but not budget fallback actuation, because no cheaper observed managed fallback model exists in the current catalog.

- RQ5 / E5 Declared-residency enforcement: `PARTIALLY SUPPORTED`
  `AISovereigntyPolicy` now has both report-only and live enforce-mode evidence on AKS. In `reportOnly`, it produced real findings for `finance/risk-assistant` traffic routed to the US deployment. In `enforce`, we captured a real gateway reroute from `gpt-us-mini` to `gpt-france-mini` after temporarily narrowing `allowedZones` to `FR`, and we verified rollback when restoring `reportOnly`. This is still partial rather than full because the broader `FR+EU` policy first exposed a real implementation limitation: the controller selected the cheapest compliant model (`mistral-small`) without checking that the target was routable in the live gateway catalog.

- RQ6 / E6 Human workflow: `PARTIALLY SUPPORTED`
  A live `AIRouteOverride` reroute and rollback sequence was executed on AKS after publishing operator image `ghcr.io/ihsenalaya/ai-sovereign-finops-operator/controller:0.5.12`. A live `AIChangeRequest` remained non-actuated in `Pending`, then actuated after `Approved`, and the route was manually restored afterward because the current `AIChangeRequest` controller does not implement automatic rollback on deletion. A follow-up rejected-path run also showed that `finance-reroute-rejected` stayed non-actuating both while `Pending` and after `spec.approval=Rejected`, while `greenops-openai-us` kept the same backend throughout. Separately, the source tree now includes a tested `AIRoutingPolicy -> AIChangeRequest` creation path, but that rebuilt controller image has not yet been redeployed and remeasured on AKS.

- RQ7 / E7 Shadow-AI detection: `PARTIALLY SUPPORTED`
  Tetragon is now deployed on the rebuilt AKS cluster, a rogue workload produced direct `api.openai.com` egress, `shadow-egress` was populated from live Tetragon events, and the operator emitted a `ShadowAI` warning event for the bypass. A second rogue workload produced direct `api.anthropic.com` egress; Tetragon export logs captured that path, the reconstructed bridge payload contained `shadow/rogue-direct-llm-2`, and the operator later emitted `ShadowAI` warnings for the Anthropic path as well. A fuller precision/recall evaluation and continuous bridge-automation validation are still pending.

- RQ8 / E8 Overhead/scalability: `PARTIALLY SUPPORTED`
  A new AKS performance run now exists at `article2/experiments/runs/E8_performance/aks-live-20260708T191147Z/`. The 12-application profile completed with 12/12 successful requests and `p50=165 ms`, while the 50-application profile completed with 50 requests, 4 errors, `p50=255.5 ms`, and a very heavy long tail (`p95≈25.2 s`, `p99≈26.7 s`). The 100-application profile did not fit on the current single-node AKS cluster: Kubernetes left 74 pods Pending and only 26 Running, with scheduler events stating `0/1 nodes are available: 1 Too many pods`. This is useful real AKS evidence for cluster-capacity limits, but it is still short of the full prompt because we do not yet have a completed 100-application run, direct-path overhead comparison, or a cleaner CPU/memory view.

## Coverage Tables

- Application matrix: `article2/experiments/tables/application_experiment_matrix.md`
- Detailed E3 gate table: `article2/experiments/tables/e3_qualitygate_detailed.md`

## Claim Impact

- Claims currently safe in the paper draft:
  Real AKS deployment of the operator and gateway control plane.
  Live FinOps reporting with per-workload attribution signals.
  Live quality-gate execution producing both pass and fail decisions.
  Live budget phase transitions through `Warning`, `Critical`, and `Exceeded`.
  Live sovereignty enforcement by gateway reroute under a claim-safe FR-only policy.
  Live declared-residency findings in report-only mode.
  Live human approval gating, explicit rejection no-op behavior, and route actuation after approval.
  Live shadow-AI detection on rogue direct-egress workloads across OpenAI and Anthropic endpoint families.

- Claims still forbidden in the paper draft:
  Full routing-tradeoff superiority claims.
  Automatic budget fallback actuation claims.
  Broad enforcement-mode sovereignty claims that ignore routability limits.
  Automatic human-workflow rollback claims.
  Full shadow-AI precision/recall or fully automated bridge-operation claims.
  Scalability claims.
