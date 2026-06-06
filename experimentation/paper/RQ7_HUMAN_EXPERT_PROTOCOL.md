# RQ7 — Human FinOps expert vs automated control plane (PLANNED — future work)

**Research question.** How close are the control plane's routing and hosting recommendations to the
decisions made by experienced FinOps / cloud experts?

Status: **future work**. No human study has been conducted; no results may be reported until collected.

## Design
- **Participants:** N ≥ 6–10 practitioners (cloud/FinOps/platform engineers) with LLM-ops experience.
- **Materials:** a set of decision scenarios drawn from the testbed — for each: workload profile,
  model/provider catalog with prices, sovereignty policy, budget state, and recent usage telemetry.
- **Task:** for each scenario, the expert chooses (a) which model/provider to route to, and (b) the
  managed-vs-self-host recommendation, with a short rationale.
- **System:** the operator produces the same decisions (routing pick + break-even recommendation).

## Metrics
- **Agreement** between expert decisions and the system: exact-match rate on model/provider choice;
  agreement on the host/keep-managed recommendation (Cohen's Kappa across experts and vs system).
- **Decision time** (human vs automated) and **rationale overlap** (which signals experts cite:
  cost, sovereignty, budget, quality) vs the system's stated reasons.
- **Disagreement analysis:** where and why experts and the system diverge (qualitative).

## Threats
- Small expert pool; scenario realism; experts may lack full price visibility. Mitigate with diverse
  recruitment, realistic telemetry, and pre-registration of the analysis.

## Honesty
RQ7 is listed as future work in the paper; the protocol is published for transparency. Do not
fabricate agreement scores or expert decisions.
