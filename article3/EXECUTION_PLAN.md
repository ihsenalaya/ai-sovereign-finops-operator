# GOV-AR Q1 recovery ExecPlan

## Objective and authority

Repair, validate, and release Article 3 under `Q1_GATE_CONTRACT.md`. The target is rigorous, submission-grade evidence; negative or mixed results are retained. Archived Codex-era measurements are excluded.

## Verified takeover state

- Recovery branch: `article3-q1-recovery-20260711`.
- Operator base: `origin/main` at `07cdd3baad26abfa7248dd69cdd507aab4be8177`.
- Coherent remote app/chart version: `0.5.11`; no fetched `v0.5.11` tag exists. The latest fetched tag is `v0.5.4` and is not the release version declared by the remote charts.
- Remote controller `0.5.11` digest and OCI chart checksum are recorded in `provenance/base_version.json`.
- Prior Article 3 evidence is quarantined under `archive/codex_20260711/` with SHA-256 provenance.

## Phase graph and acceptance

`A -> B -> C -> D -> E -> F -> G -> H -> I(E0..E7) -> J -> K -> clean release verification`

| Phase | Depends on | Acceptance evidence | Status |
|---|---|---|---|
| A forensic/operator audit | branch + archive | recomputed prior-output audit; source-derived architecture/inventories reproducible by auditor | completed (`ed8105d`) |
| B literature/novelty | A comprehension | two saturation passes; >=40 verified references; independent novelty and red-team resolutions | completed |
| C formulation/theory | B novelty gate | frozen 2-3 contributions; reviewed assumptions/proofs; formal invariant model passes | in progress |
| D implementation | C | measured-path tests, race, transaction, integration, gateway, envtest all pass | pending |
| E images/Helm/Kind | D | immutable remote digests; install/upgrade/rollback/uninstall; three idempotent cluster profiles | pending |
| F datasets/splits | B,D | verified licenses/checksums; contamination checks; hidden frozen test split | pending |
| G baselines | C,F | faithful implementations validated on identical streams | pending |
| H pilot/freeze | D-G | precision/power report and immutable non-draft protocol hash | pending |
| I experiments | H | E0 gate and valid E1-E7 manifests/count floors/checksums | pending |
| J statistics | I | independent raw-only recomputation; all discrepancies resolved | pending |
| K manuscript/release | B,J | current Q1 verification; substantive clean build; packages secret-scanned | pending |
| Release verification | K | strict gate 0 and clean-checkout independent verifier approval | pending |

## Current checkpoint

Phase C. Replace the exploratory formulation/state machine with the red-team-approved three-contribution boundary; define every liability/risk event and transition; add outbox/exactly-once-ledger semantics, unresolved-liability carryover, explicit assumptions, and an executable formal model. Obtain independent theory review before implementation.

## Recovery commands

```bash
git status --short --branch
python3 article3/tools/q1_gate.py --write-status
python3 article3/tools/q1_gate.py --strict
bash article3/run_all_q1.sh --resume
sha256sum -c article3/archive/codex_20260711/PRE_ARCHIVE_SHA256SUMS.txt  # paths require archive remapping; gate performs it
```

For interrupted experiments, resume only runs whose manifest code/config/data hashes match. Invalidate incomplete or hash-mismatched runs. For Azure/GHCR transient errors use bounded exponential backoff; for authentication failure continue all offline phases and write the minimal action to `HUMAN_ACTION_REQUIRED.md`.

## Independent roles

- Operator architecture auditor: source/inventory review, no implementation self-review.
- Literature/novelty auditor: current primary-source review.
- Experiment/power reviewer: pilot and frozen-design challenge.
- Systems implementer: code path implementation.
- Theory reviewer and statistical auditor: independent derivation/raw-only recomputation.
- Scientific red team: fresh-context claim and method attack.
- Release verifier: clean-checkout rebuild and remote digest verification.

Findings and resolutions live under `article3/reviews/`; decisions are appended to `DECISIONS.md`.
