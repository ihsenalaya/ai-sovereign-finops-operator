# GOV-AR PostgreSQL schema v4

`init.sql` creates the integer-micro ledger, immutable cohort registry, inbox,
and outbox used by the service. It contains no floating monetary columns.
The exact layout is bound by `govar_schema_metadata`; runtime restarts verify
cross-table foreign keys and recompute monetary aggregates before readiness.
Empty legacy floating layouts are incompatible and are not mutated in place.
Empty pre-v3 integer and unversioned layouts are equally incompatible at
runtime; use the evidence-producing clean migration instead of relying on
`CREATE TABLE IF NOT EXISTS` to change their meaning.

`clean_migrate_v4.sh` is deliberately fail closed. It records a schema dump,
row count, and SHA-256 checksums; refuses every nonempty ledger; then performs
a clean v4 install and records the resulting schema and zero-row
reconciliation. Monetary rows are never rounded or silently backfilled.
Nonempty legacy state must be externally reconciled and archived before this
clean experimental migration. A database rollback does not down-convert v4
rows: retain the pre-migration dump and restore it only as a separately named
database after stopping all writers.

For local development:

```bash
docker compose -f article3/infra/postgres/docker-compose.yaml up -d
export DATABASE_URL=postgres://govar:govar@127.0.0.1:5432/govar?sslmode=disable
(cd operateur && go run ./cmd/gov-ar-admission/main.go)
```

Production requires PostgreSQL. In-memory operation is an explicitly enabled,
single-replica development mode; absence of `DATABASE_URL` does not silently
select it.
