# Test Report

## Validated on 2026-07-11

Successful checks:

- `cd article3 && go test ./...`
- `cd operateur && go test ./internal/govar ./internal/sidecarproxy ./cmd/gov-ar-admission/... ./cmd/header-proxy/...`
- `cd operateur && go vet ./internal/govar ./internal/sidecarproxy ./cmd/gov-ar-admission/... ./cmd/header-proxy/...`
- `cd operateur && helm lint charts/ai-sovereign-finops-operator --set govArAdmission.enabled=true --set govArAdmission.postgres.enabled=true`
- `bash article3/overleaf/build.sh`
- `python3 article3/analysis/scripts/generate_figures_tables.py`
- `python3 article3/analysis/scripts/generate_statistical_summary.py`

## Fix applied during this validation pass

`operateur/internal/govar/postgres_engine_test.go` used `t.Context()`, which is
not compatible with the module target declared in `operateur/go.mod` as
`go 1.21`. The test now uses `context.Background()`, after which `go vet`
passes.

## Remaining validation not completed in this pass

- full `go test ./...` for the entire operator repo including environment-bound
  end-to-end paths
- `go test -race` on the larger operator surface
- OCI publication validation against GHCR, blocked by package-write permission
