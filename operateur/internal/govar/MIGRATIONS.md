# GOV-AR ledger compatibility and migration

The integer ledger does not automatically import the earlier exploratory
`DOUBLE PRECISION` tables. Startup fails if any legacy tenant, reservation, or
settlement row exists. Reconcile those rows outside the service, archive their
provenance, and start from a reviewed integer migration or a clean experimental
database.

Existing integer reservations must contain an immutable admission fingerprint,
candidate snapshot version, pricing version, and policy version. Rows missing
the fingerprint are rejected at startup; they must not be backfilled from a
current mutable catalog.

The current budget-window rule is deliberately fail closed. A tenant with any
settled cost, outstanding liability, or carried debt cannot change budget
policy identity, generation, target, period, or amount. Automated window
rollover and carry allocation are not implemented yet.

Production mode requires PostgreSQL. `GOV_AR_DEV_IN_MEMORY=true` is explicitly
development-only and the Helm chart requires one replica in that mode.
