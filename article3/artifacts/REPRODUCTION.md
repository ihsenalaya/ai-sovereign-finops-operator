# Reproduction

## Core paths

- paper source: `article3/overleaf/`
- experiments: `article3/experiments/`
- provenance: `article3/provenance/`
- operator integration: `operateur/internal/govar/`,
  `operateur/cmd/gov-ar-admission/`, and
  `operateur/internal/sidecarproxy/`

## Rebuild the paper

Run:

```bash
bash article3/overleaf/build.sh
```

This refreshes `article3/overleaf/main.pdf` and the packaged Overleaf archive.

## Re-run local article3 tests

Run:

```bash
cd article3
go test ./...
```

## Re-run operator GOV-AR validation

Run:

```bash
cd operateur
go test ./internal/govar ./internal/sidecarproxy ./cmd/gov-ar-admission/... ./cmd/header-proxy/...
go vet ./internal/govar ./internal/sidecarproxy ./cmd/gov-ar-admission/... ./cmd/header-proxy/...
helm lint charts/ai-sovereign-finops-operator --set govArAdmission.enabled=true --set govArAdmission.postgres.enabled=true
```

## Rebuild local images

Run:

```bash
docker build -f article3/Dockerfile.experiment -t gov-ar-experiment:local .
docker build -f operateur/Dockerfile.gov-ar-admission -t gov-ar-admission:local operateur
```

## Re-run Kind automation

Use the idempotent scripts in `article3/infra/kind/`:

- `create.sh`
- `install.sh`
- `healthcheck.sh`
- `collect-diagnostics.sh`
- `reset.sh`
- `destroy.sh`

## Re-run the live Azure validation

Use the bounded live validation workflow in `article3/infra/azure/` and
`article3/experiments/configs/e6_azure_live.yaml`. Keep it scoped because the
recorded E6 results are meant as validation, not as an unbounded large-scale
campaign.
