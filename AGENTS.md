# Article 3 recovery instructions

This repository contains a Kubernetes operator and the Article 3 GOV-AR recovery project. Work only on a dedicated `article3-q1-recovery-*` branch. Preserve unrelated user changes and never expose credentials, private endpoints, or tokens.

## Evidence rules

- Treat `article3/Q1_GATE_CONTRACT.md` as authoritative and immutable except for additive tightening reviewed by an independent verifier.
- Treat everything under `article3/archive/codex_20260711/` as exploratory, non-citable evidence. Do not read archived measurements into final analysis.
- Every numerical manuscript claim must be generated from immutable raw data by committed code and mapped in `article3/provenance/claims_to_evidence.csv`.
- Freeze `article3/experiments/registry/frozen_protocol.yaml` before final-test runs. Record every post-freeze change in the append-only protocol-deviation registry and rerun affected cells.
- Never infer success from filenames or status prose. Recompute counts, checksums, tests, remote digests, and document properties.
- Use public or synthetic prompts for live validation. Respect `ARTICLE3_AZURE_MAX_USD` (USD 100 if unset). Never create provisioned throughput.

## Primary commands

```bash
python3 article3/tools/q1_gate.py
python3 article3/tools/q1_gate.py --strict --write-status
bash article3/run_all_q1.sh --resume
(cd operateur && go test ./...)
(cd operateur && go test -race ./internal/govar/...)
```

Use `article3/EXECUTION_PLAN.md` for phase acceptance and recovery commands. Meaningful checkpoints must be committed with precise messages.

## Done definition

The work is done only when the strict gate exits 0, all final files are committed and pushed with immutable image/chart digests, and an independent release verifier rebuilds from a clean checkout with no unresolved critical or major finding.
