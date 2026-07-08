# Handover Diff Report — Q1 Revised Package

Generated: 2026-07-05. Final technical update: 2026-07-06.

Source package: `article1/article1_Q1_revised_package.zip`.

Extraction path: `article1/_handover_inputs/revised_package/`.

This file is provenance only. The imported revised package was reviewed without
overwriting the live repository. Its pre-fix empirical conclusions are
superseded by the final GHCR `0.5.11` AKS evidence.

## Superseding Evidence

- Final security matrix:
  `article1/results/raw/aks/security_attacks_A1_A11.csv`.
- Main security scope:
  A1-A10 are 300/300 blocked on real AKS.
- A11:
  1/1 fail-closed GPU-scope negative control.
- Final status:
  `GO_TECHNICAL_FOR_CLAIMED_SCOPE`.

## Merge Decision

- Keep the revised package only as historical input.
- Do not import stale PDFs or stale generated figures from the package.
- Use live regenerated figures under `article1/paper/figures/` and
  `article1/overleaf/figures/`.
- Use live regenerated tables under `article1/paper/tables/`.
- Use current AKS raw evidence under `article1/results/raw/aks/`.

## Current Rule

The paper must cite real AKS artifacts only for its main empirical claims.
`kind` and KWOK remain CI/debug/regression artifacts.
