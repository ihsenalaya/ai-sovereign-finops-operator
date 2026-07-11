# Version Audit

## Scope

This audit is anchored on the clean remote worktree created for Article 3:

- remote: `origin https://github.com/ihsenalaya/ai-sovereign-finops-operator`
- remote HEAD branch: `main`
- working branch: `article3-gov-ar`
- remote base commit: `07cdd3baad26abfa7248dd69cdd507aab4be8177`
- remote base summary: `paper: finalize governance routing resubmission artifact`
- local original checkout head at audit start: `1e700a5326c8e385b587eac49430cb830e51f7aa`
- local original branch at audit start: `main`
- local original branch state at audit start: `ahead 20` with many uncommitted changes

## Verified Git Signals

- `git fetch --all --tags --prune` completed successfully on 2026-07-10.
- remote branches seen:
  - `origin/main`
  - `origin/feat/multimodal-quality-gate`
- visible version tags sorted by semantic version:
  - `v0.5.4`
  - `v0.5.3`
  - `v0.4.0`
  - `v0.3.9`
  - `v0.1.0`

## Version Mismatch Findings

There is a significant version discrepancy between the clean remote state and the local dirty checkout that contained the Article 3 prompt:

1. Clean remote worktree on `origin/main` reports:
   - `operateur/charts/ai-sovereign-finops-operator/Chart.yaml`: `version: 0.5.11`, `appVersion: "0.5.11"`
   - `charts/ai-confidential-governance-platform/Chart.yaml`: `version: 0.5.11`, `appVersion: "0.5.11"`
   - `charts/ai-sovereign-platform/Chart.yaml`: umbrella `version: 0.1.0`, dependency on operator chart `0.5.11`

2. The user's dirty local checkout previously showed `operateur/charts/ai-sovereign-finops-operator/Chart.yaml` at `0.5.18`.

3. The release workflow publishes on Git tag pushes matching `v*`, but the fetched tag set stops at `v0.5.4`.

4. Therefore, at audit start:
   - charts and code in clean remote appear coherent around `0.5.11`
   - the local dirty checkout contains newer unpublished or not-yet-tagged changes around `0.5.18`
   - repository tags do not prove a published `v0.5.11` or `v0.5.18`

## Release and Publication Signals

- CI workflow:
  - uses Go `1.21` in `.github/workflows/ci.yaml`
  - runs manifest generation checks, `go vet`, `make test`, `make build`
- Release workflow:
  - triggers only on pushed tags `v*`
  - publishes GHCR images:
    - `controller`
    - `attestation-scheduler`
    - `key-release-gateway`
    - `platform-api`
    - `thesis-bench`
    - `platform-ui`
    - Grafana radar image
  - packages and publishes only the chart `charts/ai-confidential-governance-platform`

## Consistency Assessment

- `origin/main` is internally coherent around `0.5.11`.
- the clean remote state is the safest baseline for reproducible Article 3 work.
- the local dirty checkout contains valuable newer work, but it is not yet provenance-clean enough to treat as the base scientific artifact.
- Article 3 should continue from `07cdd3b` unless a later state is explicitly stabilized, tagged, and re-audited.

## Immediate Consequence for Article 3

- all Article 3 audit files are anchored to `07cdd3b`
- any later cherry-pick from the dirty local checkout must be logged in:
  - `article3/provenance/protocol_deviations.csv`
  - `article3/operator_audit/reuse_plan.md`
