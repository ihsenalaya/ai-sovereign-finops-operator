# Reproducible R55 source audit

R55 is closest practical prior art, so implementation-level differences are based on its pinned public source, not the Solo.io prose alone.

- Repository: `https://github.com/day0ops/quota-management.git`
- Audited commit: `c72d26a7f74f761a9f871b91b8c320db0825db25`
- Reproduction: `python3 article3/tools/audit_r55_prior_art.py`

The committed verifier checks exact SHA-256 values for the budget service, repository, money models, and architecture/design documents before evaluating any pattern.

Verified source facts at that commit:

- the implementation has an atomic check/reserve path with locked budgets and pending spend;
- expired reservations are deleted and their pending spend is decremented;
- authoritative usage arriving after the reservation is absent returns without charging a budget;
- monetary model fields are Go `float64` values;
- usage history is pruned to 30 records per budget;
- the architecture document explicitly describes demo fail-open behavior.

Concurrency inference requiring experimental falsification: `DecrementBudgets` loads a reservation before opening the charge transaction; usage insertion has no request-level conflict clause; deleting an already-deleted reservation does not require one affected row. Therefore two concurrent, differently keyed deliveries can both reach charge effects. This is our source-level inference, not a result claimed by R55. The faithful practical-envelope baseline must reproduce or refute it under the pinned code semantics before the manuscript states it as an observed defect.

These differences do not restore integration novelty. They motivate the faulty-event comparison and conservative unresolved-liability semantics.
