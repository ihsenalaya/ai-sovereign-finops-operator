# Resubmission Changelog

Date: 2026-07-08

## Manuscript Scope

- Reframed the paper as a bounded JNCA-oriented systems paper on governance-aware
  routing for networked LLM applications.
- Replaced the title, abstract, introduction, contributions, methodology,
  results, discussion, threats to validity, data availability, and conclusion.
- Removed reviewer-facing text and draft-style roadmap framing.
- Removed the planned human-evaluation study from the manuscript and artifact
  package.

## Removed or Weakened Claims

- Removed the GPU/self-hosted/break-even comparison from the main paper.
- Removed RQ6, the break-even figure, break-even CSVs, and break-even
  sensitivity rows from the paper outputs.
- Removed claims that self-hosting is cheaper or has a measured payback point.
- Weakened sovereignty language to declared-policy compliance under declared
  provider metadata.
- Added the explicit non-certification sentence: the operator enforces declared
  technical routing policies and does not independently certify legal compliance
  or contractual data residency.
- Removed general quality-preservation claims.
- Reframed N=30 statistics as fixed-matrix stability rather than population-level
  enterprise evidence.
- Reframed zero configured-policy violations as expected by construction from
  hard filtering.

## Added or Updated Evidence

- Added `paper_claims_audit.md`.
- Added prompt-level bootstrap analysis: `results/bootstrap_items.csv` and
  `figures/fig_bootstrap_items.png`.
- Added objective-task guardrail recomposition:
  `results-bench/rq_guardrail.csv`.
- Regenerated figures without the old break-even plot.
- Updated `results/summary.md` to use only paper-safe strategies and policy
  scenarios.
- Updated inter-judge agreement interpretation to weak-to-moderate.

## Code and Artifact Changes

- Updated the experiment harness so the modeled self-hosted stub is opt-in legacy
  support and excluded from default paper runs.
- Removed self-hosted stub entries from the main workload allow-lists.
- Updated router tests to match the real-provider default catalog.
- Removed generated `operateur/attestation-scheduler` from Git tracking.
- Updated README, dashboard, methodology, outline, tables, release artifact, and
  LaTeX README to point to the revised canonical manuscript.

## Packaging

- Rebuilt `experimentation/paper/paper.pdf`.
- Rebuilt `experimentation/paper/paper-latex.pdf`.
- Rebuilt `experimentation/paper/greenops-paper-overleaf.zip`.
- Rebuilt `experimentation/paper/latex.zip`.
