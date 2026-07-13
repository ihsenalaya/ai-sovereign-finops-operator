# GOV-AR PostgreSQL schema v6

`init.sql` creates the integer-micro ledger, immutable cohort registry, inbox,
outbox, and append-only per-request transition chain used by the service. It
contains no floating monetary columns.
The exact complete-liability layout is bound by `govar_schema_metadata`; runtime restarts verify
cross-table foreign keys and recompute monetary aggregates before readiness.
Empty legacy floating layouts are incompatible and are not mutated in place.
Empty pre-v3 integer and unversioned layouts are equally incompatible at
runtime; use the evidence-producing clean migration instead of relying on
`CREATE TABLE IF NOT EXISTS` to change their meaning.

`clean_migrate_v6.sh` is deliberately fail closed. It records a schema dump,
row count, and SHA-256 checksums; refuses every nonempty ledger; then performs
a clean v6 install and records the resulting schema and zero-row
reconciliation. Monetary rows are never rounded or silently backfilled.
Nonempty legacy state must be externally reconciled and archived before this
clean experimental migration. The exact v5-to-v6 migration refuses nonempty
reservations because no trustworthy historical transition chain exists to
backfill. A database rollback does not down-convert v6
rows: retain the pre-migration dump and restore it only as a separately named
database after stopping all writers.

The immutable empty v3 fixture and every reviewed incremental migration can be
exercised as one exact chain with:

```bash
DATABASE_URL=postgres://... bash article3/infra/postgres/test_migration_chain.sh
```

The command checks the final layout identifier, zero-row precondition, audit
table creation, and the route/pricing/calibration columns before printing the
input SHA-256 checksums. It is a schema compatibility test, not permission to
backfill historical monetary or audit evidence.

Every effective state transition is committed atomically with a tenant-chain
entry. Covered mutations are reserve, dispatch, settlement/correction, cancel,
expiry, rollover, budget adjustment, and cohort registration. The row contains
a contiguous per-tenant sequence allocated under a locked tenant-sequence row,
closed actor class and bounded reason, tenant/request/workload/attempt identity,
before/after state digests, frozen policy/pricing/route/cap/cohort/software
commitments, predecessor hash, and entry hash. Production startup requires a
lowercase `GOV_AR_SOFTWARE_SHA256` and binds it into every event; adaptive or
fixed-cohort reservations additionally bind the frozen calibration artifact.
Replayed or
semantically duplicate events add no entry. Database triggers reject update,
delete, and truncate. Deployments connect with a distinct nonsuperuser login
that is a member of, and immediately `SET ROLE`s to, `govar_runtime`; production
startup rejects owner/superuser credentials or a login with audit mutation
privileges. The role has SELECT/INSERT but no UPDATE/DELETE on
`govar_audit_events`; migrations
run under the separate trigger-owning role. PostgreSQL superuser or table-owner
actions are explicitly outside this role-separation claim.

Applications call `PostgresEngine.ExportTenantAudit`. Verify exported JSON
without trusting service code with:

```bash
python3 article3/tools/verify_govar_audit.py tenant-audit.json \
  --checkpoint trusted-checkpoint.json --output verification.json
python3 article3/tools/test_verify_govar_audit.py
```

The verifier rejects empty, reordered, deleted, tampered, and duplicate-event
exports. A chain is not deletion-detecting without a trusted prior checkpoint;
the checkpoint records the expected event count and per-tenant head. The
repository fixture exercises both valid and wrong-checkpoint paths. Automated
periodic checkpoint publication is not yet implemented and remains an M8
release item.

Run `init.sql` and reviewed migrations with a migration-owner credential that
is never mounted into the admission Pod. Separately create a nonsuperuser login
and grant only membership in `govar_runtime`, for example (substitute a
deployment-managed login and secret rather than committing credentials):

```sql
CREATE ROLE govar_admission LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT govar_runtime TO govar_admission;
```

The Pod's `DATABASE_URL` must name that runtime login. The service calls the
read-only `OpenPostgresEngine`; only explicit migration/test tooling calls the
schema-owning bootstrap constructor.

Calibration publication and drift-state changes currently live in the
Kubernetes controller path rather than this PostgreSQL transaction boundary.
Their event kinds are reserved in v6, but no article or release claim may say
they are atomically audited until a database-backed publication API and its
tests exist.

For local development:

```bash
docker compose -f article3/infra/postgres/docker-compose.yaml up -d
export DATABASE_URL=postgres://govar:govar@127.0.0.1:5432/govar?sslmode=disable
# Development bootstrap/tests may initialize schema with this owner URL.
# The service itself requires the separate runtime login described above.
```

Production requires PostgreSQL. In-memory operation is an explicitly enabled,
single-replica development mode; absence of `DATABASE_URL` does not silently
select it.
