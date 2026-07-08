# Phase 1 — Intermediate Report (Claude Code)
**Article 1 Q1 Hardening Final — suite.txt prompt.**
**Date:** 2026-07-05 · Scope: OFF the paying cluster (local/kind/kwok only). No AKS.

Final note 2026-07-06: this is historical Phase 1 provenance only. The AKS
Phase 2 work described as pending here has since been completed with GHCR
`0.5.11`; see `article1/suite.txt` and
`article1/status/q1_execution_report.md` for the current state.

Per the author rule locked by Codex: **main-paper results come from real AKS;
kind/kwok are CI / debug / regression only.** Everything below is regression/prep
and is NOT presented as a security or performance paper result.

## Phase 1 step status
| # | Step | Status | Evidence |
|---|------|--------|----------|
| 1 | Audit existing results | DONE | Codex already: separated ms phase timings, kind+kwok harness scaffolds, `check_latency_resolution.py` |
| 2 | Adversary model ADV-1..4 | DONE (prior) | article1/paper/sections/threat-model.tex + adversary_attack_mapping.csv |
| 3 | ms instrumentation (separate phases) | DONE | scheduler.go emits prefilter/filter/permit/score/reserve/prebind/reserve_prebind/bind/total (µs, monotonic). Build+vet+tests green |
| 4 | Anti-quantization regression test | DONE + VERIFIED | article1/scripts/check_latency_resolution.py — precise data ratio(multiples-of-1000ms)=0.000 (PASS); coarse data correctly rejected |
| 5 | Baselines B1–B5 on kind | HARNESS FIXED + RUNNING | article1/experiments/harness/measure_baselines.sh (see BUG FIX below); regression CSV article1/results/raw/kind/baselines_b1_b5.csv |
| 6 | Scalability kind + kwok | kind DONE (regression); **kwok NOT_EXECUTED** | kwokctl **segfaults** in this WSL env (binary runtime) — honest NOT_EXECUTED rows, never fabricated |
| 7 | RBAC agent test | DONE (prior) | A10 + scheduler-security (auth can-i), kind 12/12 |
| 8 | Prepare JWKS integration | DONE (prior) | pkg/attestation/maa RS256/JWKS verifier, 11 tests |
| 9 | Intermediate report | THIS FILE | — |

## Key fix this phase (BUG in baselines harness)
The first baselines run **hung** on B1/B2/B3: the admission webhook matched the
`ConfidentialInferencePolicy` (label `app=bench`) and rewrote those pods'
`schedulerName` to our scheduler + added a scheduling gate — so the "default
scheduler" baselines never scheduled. FIX: B1/B2/B3 pods now use label
`app=baseline` (NOT matched by the policy), so the default scheduler handles them;
only B5 uses `app=bench` (matches policy → our scheduler). Validated N=5:
B1/B2/B3 bind immediately; B5 precise total ~40–123 ms.

## What kind (regression) shows — NOT a paper result
- B5 (our scheduler) precise total ~40–125 ms; default-scheduler baselines
  (B1/B2/B3) bind sub-second (below K8s 1s condition resolution).
- ms phase breakdown of B5 is non-quantized (anti-quant test PASS).
- These validate the instrumentation and the scheduler logic in CI; the paper's
  B1–B5 overhead numbers must come from AKS (Phase 2).

## Honest NOT_EXECUTED / carried to Phase 2 (AKS)
- `performance_high_resolution.csv` on **real AKS** — needs re-provisioning with
  the instrumented GHCR image (`attestation-scheduler:0.5.8`). PENDING_AKS_RERUN.
- B1–B5 overhead **on AKS** — Phase 2 (§18 Phase 2 step 1).
- A2/A3/A6/A7/A9 via adversarial ServiceAccounts **on AKS** — Phase 2.
- kwok scheduler-only large-scale — kwokctl unusable in this WSL host; retry on a
  host with a working container runtime, else keep NOT_EXECUTED and omit from paper.

## Absolute-rule compliance
- No fabricated results/measurements/citations/DOIs.
- kind/kwok never presented as security or cloud-performance paper results.
- No `|| true` masking a scheduling failure; hung baselines were diagnosed and
  fixed, not hidden.
- No secrets committed.

## STOP
Per the prompt, execution STOPS after Phase 1. Phase 2 (AKS batched) requires a
fresh confidential cluster (westus2 / DCasv6 / DC8as_v6) and explicit go —
it provisions billable resources. The one-command entry point is
`article1/experiments/aks/run_full_campaign.sh` (with the 0.5.8 instrumented
images).
