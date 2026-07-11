# ADR 0002

## Title

Scope GOV-AR as an online reserve-route-settle method, not a generic feature paper

## Status

Accepted

## Context

The existing operator already supports many governance and confidential-computing features.

The strongest scientific gap identified by the audit is the absence of:

- in-flight liability accounting
- delayed-settlement-aware budget control
- admit / queue / reject / abstain decision logic

## Decision

Center Article 3 on GOV-AR as:

- a hard-governance filter
- a risk-bounded reservation method
- an online admission and routing controller
- an idempotent settlement mechanism

## Consequences

- better novelty separation
- cleaner experimental design
- more defensible contribution claims
