# Review Protocol

## Objective

Identify the closest prior work to GOV-AR across:

- LLM routing
- budget-aware decision making
- delayed feedback online learning
- contextual bandits with resource constraints
- output-length prediction for LLM serving
- multi-tenant admission and budget isolation

## Time Window

- primary focus: 2023-01-01 to 2026-07-10
- foundational exceptions allowed for prerequisite theory

## Preferred Sources

- ACL Anthology
- PMLR
- OpenReview when accepted or highly relevant recent preprint context is needed
- arXiv only when no peer-reviewed version is yet available

## Inclusion Criteria

- directly studies routing among LLMs or model families
- studies budget-constrained online decision making relevant to request routing
- studies delayed feedback for online learning or serving decisions
- studies output length prediction or serving-time estimation for generative models
- offers an algorithmic or systems mechanism that could overlap with GOV-AR

## Exclusion Criteria

- blogs, marketing material, and product docs as evidence of scientific novelty
- non-primary summaries when the primary paper is available
- papers on generic access control or multi-tenancy with no routing or budget-control relevance

## Data Extraction Fields

- bibliographic metadata
- peer-reviewed or preprint status
- problem formulation
- whether routing, admission, budget, unknown output cost, delayed settlement, inflight liability, multitenancy, hard governance, risk bounds, atomic ledger, or Kubernetes enforcement are present
- key datasets and baselines
- main difference from GOV-AR
