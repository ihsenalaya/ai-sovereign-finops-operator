# ADR 0001

## Title

Use clean `origin/main` commit `07cdd3b` as the scientific baseline for Article 3

## Status

Accepted

## Context

The original local checkout contained many uncommitted and ahead-of-remote changes, including a newer apparent operator chart version around `0.5.18`.

The clean remote worktree:

- is provenance-clean
- is reproducible
- is coherent around `0.5.11`

## Decision

Article 3 development starts from the clean remote worktree on branch `article3-gov-ar`.

## Consequences

- easier auditability
- lower risk of mixing unpublished local artifacts with measured results
- any later reuse from the dirty local checkout must be explicitly documented
