# Experiment Orchestrator

This directory contains the reproducible entrypoints used to execute and verify
the current GOV-AR experiment suite.

## Current scope

- `run_all.sh`: runs the implemented local experiment chain end to end.
- `verify_outputs.sh`: validates that expected raw, processed, and report
  artifacts exist and remain parseable.
- `../../infra/azure/run_e6_live.py`: executes the credentialed live Azure
  validation for E6.

## Usage

From `article3/`:

```bash
bash experiments/orchestrator/run_all.sh
bash experiments/orchestrator/verify_outputs.sh
python3 infra/azure/run_e6_live.py
```

The current orchestrator covers the implemented scaffold for:

- `E0`
- `E1`
- `E2`
- `E3`
- `E4`
- `E5`
- `E7`
- `E1-campaign`
- `E2-campaign`
- `E1-matrix`
- `E2-matrix`
- `E1-compare`
- `E2-compare`

`E6` is intentionally executed through the Azure-specific runner because it
requires authenticated discovery and live invocation against existing
deployments.
