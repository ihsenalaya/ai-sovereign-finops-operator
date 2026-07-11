# Article 3 Transfer Package

This package is a point-in-time transfer of the Article 3 worktree. It
contains the Article 3 source, design, operator audit, tests, raw and processed
experiment outputs, logs, figures, tables, provenance, manuscript sources,
compiled PDF, Overleaf archive, replication archive, and the Article 3-related
operator integration source.

The package is intentionally evidence-preserving. It does not claim that the
manuscript is Q1-ready. The authoritative limitation record is:

- `article3/reports/PROMPT_LINE_BY_LINE_AUDIT.md`
- `article3/reports/PROMPT_RELEASE_GATE.md`
- `article3/artifacts/FINAL_REPORT.md`

The strict release gate currently fails for incomplete prompt-scale evidence,
the literature target, and GHCR/OCI publication permissions. No Azure secret,
GitHub token, private key, `.env` file, or `.git` metadata is included.

Useful entry points:

- `article3/README.md`
- `article3/REPRODUCTION.md`
- `article3/run_all.sh`
- `article3/experiments/orchestrator/run_all.sh`
- `article3/artifacts/GOV_AR_article.pdf`
- `article3/artifacts/GOV_AR_overleaf.zip`
- `article3/artifacts/GOV_AR_replication_package.zip`
- `operateur/cmd/gov-ar-admission/main.go`
- `operateur/internal/govar/`

The archive excludes `.git/`, generated operator binaries under
`operateur/bin/`, Python caches, and LaTeX temporary files. Those exclusions
are mechanical packaging exclusions, not experiment-result exclusions.
