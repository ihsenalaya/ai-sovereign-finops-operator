# Formal ledger model

`ledger_model.py` performs depth-six bounded exhaustive exploration of a symmetry-reduced two-tenant action alphabet plus explicit longer traces. It covers reserve, transactional outbox claim/cancel/delivery, unambiguous unbilled failure, timeout/expiry, unresolved rollover, provisional/late settlement, semantic duplicates, monotone corrections, authoritative finality, wrong tenant/workload UID, and same-tenant concurrent records.

It recomputes these conditional ledger properties:

- non-negative integer monetary views;
- conditional strict-ledger feasibility when actual estimated token cost is no greater than its reservation;
- record/aggregate equality;
- unresolved dispatched liability and non-negative carried debt survive window rollover;
- dispatch claim and pending-outbox cancellation have mutually exclusive ledger effects;
- provisional settlement retains `R-C` correction exposure; after rollover a temporary guard keeps total exposure at `R` until finality, while historical credits remain audit-only;
- one effective base settlement, monotone correction delta, and finality effect despite duplicates;
- wrong-tenant/wrong-workload/rejected transitions are atomic no-ops;
- record/aggregate equality and tenant/workload isolation.

Run:

```bash
bash article3/formal/check.sh
```

The checker reruns the model, verifies `results.json` against the model SHA-256, and requires the named semantic invariants. It does not prove provider billing caps, calibration, selected-set coverage, availability, regret, or universal correctness. Those remain explicit assumptions and empirical obligations.
