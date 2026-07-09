# Final Submission Checklist

Status: READY FOR RESUBMISSION WITH NARROWED CLAIM SET

## Manuscript Integrity

- [x] PDF compiles from local sources
- [x] Bibliography is no longer empty
- [x] Results section is aligned with measured AKS evidence
- [x] Unsupported claims are explicitly excluded in the manuscript
- [x] Reference depth is journal-ready

## Experiment Integrity

- [x] Real AKS deployment evidence exists
- [x] Live FinOps attribution evidence exists
- [x] Live FinOps cost attribution has been independently rechecked against raw gateway token telemetry
- [x] Live quality-gate pass/fail evidence exists
- [x] Live budget phase-transition evidence exists
- [x] Live sovereignty report-only evidence exists
- [x] Live sovereignty enforce-mode reroute evidence exists under an FR-only claim-safe policy
- [x] Live workflow approval and reroute evidence exists
- [x] Live signed-admission evidence exists for `AIChangeRequest` and opt-in evidence ConfigMaps
- [x] Live shadow-AI true-positive evidence exists
- [x] A multi-tenant AKS portfolio with more than 10 normal applications is now deployed and labeled
- [x] Two rogue applications are now deployed on AKS
- [x] A rejected AIChangeRequest no-op path is now captured on AKS
- [x] A second shadow-AI true-positive path is now captured for `api.anthropic.com`
- [x] A detailed extracted E3 QualityGate table now exists
- [x] An explicit application-to-experiment coverage matrix now exists
- [x] Routing trade-off baseline family is complete within the bounded eight-baseline live design
- [x] Statistical analysis plan has been executed on the measured E2/E3 evidence used in the paper
- [x] Shadow-AI precision/recall study is complete for the bounded nine-scenario live matrix
- [x] Scalability / overhead campaign is complete for the bounded 12/50/100-app controlled AKS profile set

## Reproducibility

- [x] Operator image tag and digest are recorded
- [x] Live Helm release version is recorded
- [x] Raw experiment evidence directories are present
- [x] Claims-to-evidence traceability exists
- [x] End-to-end automation is complete enough for the bounded Article 2 reproduction path

## Validation

- [x] Overleaf ZIP has been regenerated from the current paper tree
- [x] Reproducibility ZIP has been regenerated with secret filtering
- [x] Forbidden-phrase check has been rerun on the current manuscript sources
- [x] Claims-to-evidence check has been rerun on the current audit/report mapping

## Submission Decision

The article is now appropriate for a resubmission whose claim set stays aligned with the measured AKS evidence. The paper should still avoid universal routing, whole-cluster security, and production-scale scalability claims, but T0--T5 of the current prompt are backed by live artifacts, updated tables, generated figures, and claims-to-evidence traceability.

## v5 cycle (M1–M4 + minors) — completed 2026-07-09

- [x] M1: request-count semantics clarified (error-dominated 660 vs 85 billed); collector fix + unit test
- [x] Absolute euro costs removed article-wide; consumption reported in tokens/requests
- [x] M2: controller-overhead campaign (3 tiers) with reconcile/workqueue/CPU/memory + 2 figures
- [x] M3: E2 pairwise significance (Wilson + Holm-McNemar); Figure 4 reworked with 95% CIs; stale latency numbers corrected
- [x] M4: evidence-gated residency enforcement; escalation demonstrated live on AKS; actuation branch unit-tested
- [x] MIN: shadow-AI raw counts; verified eBPF academic reference; Table 3 (E5→Supported); coherence pass
- [x] Operator image `controller:0.5.18` built, pushed to GHCR, deployed on AKS
- [x] Final PDF: 25 pages, 0 undefined citations/references; `article2/build/article2.pdf`
- [x] v4→v5 change summary: `reports/DIFF_v4_to_v5.md`

### Honest residuals (each needs a decision, not silent completion)
- [ ] M4 actuation branch not demonstrated *live* (a genuine candidate-safe verdict for the
      specific pair needs a full golden-evidence evaluation; hand-crafting evidence would be
      fabrication). Covered by unit test; escalation branch is live.
- [ ] Bibliography vendor-doc trimming only partial (one verified academic reference added);
      further academic references not added to avoid citing unverified sources.
- [ ] 34 pre-existing overfull-hbox warnings (IEEE narrow-column typography), none on new figures.
