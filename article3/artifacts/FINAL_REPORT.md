# Final Report

## Scope

Article 3 work was executed in the dedicated worktree
`/mnt/c/Users/Ihsen/Documents/kubebuilder/ai-sovereign-finops-operator-article3`
on branch `article3-gov-ar`, starting from base commit
`07cdd3baad26abfa7248dd69cdd507aab4be8177`.

## What was completed

- operator audit deliverables under `article3/operator_audit/`
- GOV-AR research scaffold under `article3/src/`
- frozen experimental protocol and reproducible orchestrator
- experiment families E0, E1, E2, E3, E4, E5, E6, and E7 with raw and
  processed outputs
- operator-side GOV-AR integration slice with:
  - shared CRD snapshot translation
  - stable reason codes
  - `gov-ar-admission` service
  - optional PostgreSQL-backed ledger
  - `header-proxy` admit, settle, and cancel path
- local Docker build, Helm lint, Helm package, Kind execution, manuscript PDF,
  Overleaf ZIP, and replication bundle generation

## Verified environment facts

- base commit: `07cdd3baad26abfa7248dd69cdd507aab4be8177`
- local original checkout at prompt start: `1e700a5326c8e385b587eac49430cb830e51f7aa`
- chart baseline: `0.5.11`
- Go: `go1.26.4`
- Helm: `v3.18.6`
- Kind: `v0.31.0`
- Docker: `28.3.2`
- Azure CLI: `2.75.0`
- GH CLI: `2.4.0+dfsg1`

## Azure models validated

- `gpt_france_mini` on `a2fr60f9b020260708` in `francecentral`
- `gpt_us_mini` on `a2us60f9b020260708` in `eastus`
- `mistral_large_latest` on `greenops-fdry-60f9b0` in `westus2`

The live run remains a bounded validation pass with estimated token cost, not a
full-scale paid production benchmark.

## Main artifacts

- PDF: `article3/artifacts/GOV_AR_article.pdf`
- Overleaf ZIP: `article3/artifacts/GOV_AR_overleaf.zip`
- replication bundle: `article3/artifacts/GOV_AR_replication_package.zip`
- experiment summary: `article3/artifacts/EXPERIMENT_SUMMARY.csv`
- claims mapping: `article3/artifacts/CLAIMS_TO_EVIDENCE.csv`
- image digests: `article3/artifacts/IMAGE_DIGESTS.csv`

## Remaining gaps

- GHCR publication is still blocked by missing effective `packages:write`
  permission in the active GitHub authentication context
- OCI chart publication to GHCR remains blocked for the same reason
- the current operator integration is a real proxy-triggered path, but not yet
  a final Envoy-native or `ext_proc` native production path
- the literature review is substantially expanded but not yet fully saturated to
  the prompt target of a final 35 to 50 reference submission set
- the manuscript is compiled and strengthened, but not yet fully submission
  hardened for a Q1 venue
