# GOV-AR ledger compatibility and migration

The integer ledger does not automatically import the earlier exploratory
`DOUBLE PRECISION` tables. Startup fails if any legacy floating-point layout
exists, even when empty. Reconcile those schemas outside the service, archive their
provenance, and start from a reviewed integer migration or a clean experimental
database.

Existing integer reservations must contain an immutable admission fingerprint,
candidate snapshot version, pricing version, and policy version. Rows missing
the fingerprint are rejected at startup; they must not be backfilled from a
current mutable catalog.

Schema v6 is the route-, cap-, complete-price-snapshot-, and tamper-evident
transition-audit-bound integer ledger in `article3/infra/postgres/init.sql`. The
companion `clean_migrate_v6.sh` records before/after schema dumps, exact row
counts, reconciliation output, and SHA-256 checksums. It refuses every
nonempty ledger: there is no implicit float-to-integer monetary conversion.
Runtime bootstrap also refuses an empty pre-v3 or unversioned layout. Empty
exact v3 can migrate transactionally to v4, v5, and then v6, but nonempty reservations or v1
cohorts are refused and preserved as read-only evidence. Runtime startup accepts
only the exact v6 layout identifier and required
safety columns, then recomputes tenant aggregates, active holds, carried debt,
audit credit, and reservation/outbox identity. Any discrepancy fails readiness
instead of being repaired from mutable state.

The v5-to-v6 migration also refuses nonempty reservations. Earlier versions did
not atomically commit audit-chain entries, so retroactive rows would be
fabricated provenance. Each v6 tenant chain covers effective reserve, dispatch,
settlement/correction, cancel, expiry, window rollover, budget adjustment, and
cohort-registration mutations in their serializable transaction. A dedicated
tenant sequence row is locked to allocate a contiguous monotone sequence.
PostgreSQL rejects update, delete, and truncate on the event table. The
`govar_runtime` role has only SELECT/INSERT on events; the migration owner owns
the protection trigger. Exported rows are independently verified by
`govaraudit.Verify` and `article3/tools/verify_govar_audit.py`. Production opens
the migrated layout without DDL, requires a distinct nonsuperuser login that
is a member of `govar_runtime`, executes `SET ROLE govar_runtime` on every
pooled connection, and rejects migration-owner credentials and direct audit
mutation privileges. `GOV_AR_SOFTWARE_SHA256` is mandatory in that production
path and is bound into each transition. The migration credential must never be
mounted into the admission service.

Trusted checkpoints are required to detect deletion of an otherwise valid
chain suffix; the independent verifier supports them and regression fixtures
cover truncation, reordering, empty input, and wrong heads. Periodic checkpoint
publication is not yet implemented. Likewise, calibration/drift state is still
published by the Kubernetes controller rather than atomically through this
ledger, so v6 reserves those typed events but does not claim that coverage.

Daily, weekly, and monthly UTC windows are derived from the server clock.
Policy identity and budget are immutable within a window. At renewal,
unresolved holds persist; on-time provisional records gain a guard equal to
their provisional actual so guard plus residual remains the original
reservation until finality. Late actuals and positive corrections become
nonnegative carried debt. Historical downward corrections are audit-only and
never expand admission availability. Debt can change only through a separately
authorized, window-bound adjustment carrying a time-bounded HMAC proof from a
configured ledger authority. Frozen-cohort artifact hashes and the pre-outcome
registry digest require the same authority boundary. Provider retries, hedges,
and fallbacks are disabled: the attempt records `NO_PROVIDER_RETRY`, and a
second attempt fails closed because no separately reserving retry API exists.

Production mode requires PostgreSQL. `GOV_AR_DEV_IN_MEMORY=true` is explicitly
development-only and the Helm chart requires one replica in that mode.
