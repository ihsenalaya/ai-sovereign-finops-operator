# Formal Model

This directory contains the Phase C deterministic ledger model for GOV-AR.

`ledger_model.py` performs bounded exhaustive state exploration over reserve,
settle, duplicate-settle, wrong-tenant, and expiry transitions for two tenants
and multiple requests. It checks deterministic strict-mode safety invariants:

- non-negative tenant ledger views;
- `settled + reserved <= budget` under the explicit strict-mode assumption that
  authoritative actual cost is no greater than the reservation;
- per-request records match tenant aggregate views;
- terminal events have a single effect;
- wrong-tenant transitions are rejected as no-ops;
- duplicate settlement with the same event id is idempotent.

It does not prove empirical calibration, regret, or live provider correctness.
Those remain experiment and statistical-analysis obligations.
