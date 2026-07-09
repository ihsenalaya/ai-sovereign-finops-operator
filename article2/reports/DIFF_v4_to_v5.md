# Article 2 — v4 → v5 change summary (M1–M4 + minors, publication cycle)

Target: IEEE TNSM. Final PDF: `article2/build/article2.pdf` (25 pages, 0 undefined
citations/references). Every number traces to a run folder under `article2/reports/`
or `article2/experiments/runs/` and to `CLAIMS_TO_EVIDENCE.md`.

## Abstract
- E2 claim reframed from a vague "cost/quality/latency trade-off" to a measured
  **equivalence + cost result**: routed baselines preserve objective exact-match
  and latency (overlapping 95% CIs; 27/28 pairwise comparisons indistinguishable)
  while reducing normalized per-request cost ~35%.
- Removed the "capacity ceiling at ~25 ready applications" (a node-pool artifact);
  replaced with control-plane reconciliation overhead staying low and flat.

## Related Work
- Added an eBPF egress-observability sentence with a verified academic reference
  (Wasm-bpf, arXiv:2408.04856, 2024).

## Design / Quality Gate (Sec. 5)
- "Integration with routing and budget enforcement": the un-evidenced-reroute
  admission is replaced by the **implemented** evidence-gated enforcement (consults
  the matching AIQualityGate; escalates to a Pending AIChangeRequest otherwise).
- Added subsection **"Justification of the composite weights"** (weight sensitivity
  + answer-quality/operational separation + 3 recent references), from an earlier
  cycle, retained.

## Results (Sec. 8)
- **M1 (E1)**: FinOps paragraph now defines `requests` as successful token-producing
  calls; documents that the earlier "660 requests" was an **error-dominated count**
  (660 throttled 429s carrying zero tokens) and that all 1,299 in / 10,882 out
  tokens came from the 85 successful calls. Traces to
  `M1_request_count_semantics/`.
- **Cost removal**: all absolute euro amounts deleted across the paper; consumption
  now reported in tokens/requests, differences as normalized ratios/percentages.
- **M2 (E8)**: new **"Controller overhead"** subsection + two figures
  (`fig_e8bis_reconcile.pdf`, `fig_e8bis_resources.pdf`): reconcile p50 ~20 ms,
  p95 91→66 ms as reconciled objects grow 40→240; workqueue depth 0; CPU
  0.012→0.020 cores; memory ~53 MiB. Traces to `M2_controller_overhead/`.
- **M3 (E2)**: added N=180/baseline, Wilson 95% CIs (all overlapping), Holm-corrected
  pairwise McNemar (only 1/28 pairs significant: B4 vs B8, p=0.027); latency
  134–175 ms with overlapping CIs; **stale 229–325 ms latency numbers (from an old
  run) corrected**. Figure 4 reworked: normalized-cost axis, exact-match with 95%
  CI error bars, de-overlapped labels. Traces to `E2.../processed/e2_significance.json`.
- **M4 (E5)**: new **"Evidence-gated enforcement"** paragraph — escalation path
  demonstrated live on AKS (Pending AIChangeRequest `sov-regulated-france-policy-
  gpt-us-mini`, route unchanged); actuation branch covered by unit test. Traces to
  `M4_quality_gated_enforcement/`.
- **MIN (E7)**: shadow-AI FP/FN reported as **raw counts** (2 FP / 4 negative;
  2 FN / 5 positive) instead of two-decimal rates.
- **Table 3 (status)**: E5 upgraded Partial→**Supported**; E8 row updated to include
  the controller-overhead result.

## Discussion (Sec. 9)
- Removed the thrice-repeated "no route change without adequate evidence is not
  enforced" admission; replaced with the positive result (enforced for residency,
  not yet global — budget fallback remains future work).

## Conclusion
- Added "route actuation gated by quality evidence with human escalation" to the
  demonstrated-capabilities sentence.

## Code / artifact (operator)
- `internal/collectors/aigw`: request counter split success vs error (M1) + unit
  test `TestCollectSplitsSuccessAndError`.
- `internal/qualitygate` (new, pure) + 6 unit tests; `AISovereigntyPolicy` gains
  `requireQualityEvidenceForReroute`; sovereignty controller consults the gate and
  escalates (M4).
- Image `ghcr.io/ihsenalaya/.../controller:0.5.18` built, pushed, deployed on AKS.

## Known residuals (honest)
- M4 actuation branch: live AKS demo not shown (a genuine candidate-safe verdict
  for the specific pair needs a full golden-evidence evaluation); covered by unit
  test. Escalation branch is live.
- Bibliography vendor-doc trimming (MIN-3) only partially done (one academic ref
  added); deeper grouping deferred.
- 34 minor overfull-hbox warnings (pre-existing IEEE narrow-column typography),
  none on the new figures.
