# Article3 Changelog

## Major additions

- created the `article3/` research workspace with audit, design, experiment,
  provenance, manuscript, and artifact structure
- implemented the GOV-AR research scaffold with reserve-settle ledger,
  admission decisions, predictors, replay harness, and experiment runners
- executed and recorded experiment families E0 through E7
- added operator-side GOV-AR integration through `operateur/internal/govar`
- added `gov-ar-admission` HTTP service and optional PostgreSQL-backed ledger
- extended `header-proxy` with live `admit`, `settle`, and `cancel` hooks
- added Helm support for optional `gov-ar-admission` deployment
- compiled the manuscript and generated the Overleaf and replication archives

## Validation update on 2026-07-11

- revalidated targeted operator GOV-AR tests
- revalidated Helm lint for the GOV-AR chart path
- revalidated manuscript compilation
- fixed a `go vet` incompatibility in `postgres_engine_test.go` for `go 1.21`

## Outstanding items

- GHCR image and OCI chart publication
- final Envoy-native integration path
- full literature saturation and final paper polish
