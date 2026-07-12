#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
evidence_dir="${1:-${root}/migration-evidence}"
mkdir -p "${evidence_dir}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
prefix="${evidence_dir}/${stamp}"

# This command is intentionally a clean migration, not an implicit monetary
# conversion. Any nonempty legacy/current ledger requires reviewed external
# reconciliation and is refused before mutation.
psql "${DATABASE_URL}" -X -qAt -v ON_ERROR_STOP=1 > "${prefix}.row_count" <<'SQL'
CREATE TEMP TABLE govar_migration_count(total BIGINT NOT NULL);
INSERT INTO govar_migration_count VALUES(0);
DO $$
DECLARE name TEXT; n BIGINT;
BEGIN
  FOREACH name IN ARRAY ARRAY['govar_tenants','govar_reservations','govar_settlements','govar_outbox','govar_inbox','govar_budget_adjustments','govar_reconciliation_tasks','govar_frozen_cohorts','govar_frozen_cohort_slots'] LOOP
    IF to_regclass(name) IS NOT NULL THEN
      EXECUTE format('SELECT count(*) FROM %I',name) INTO n;
      UPDATE govar_migration_count SET total=total+n;
    END IF;
  END LOOP;
END $$;
SELECT total FROM govar_migration_count;
SQL
if [[ "$(tr -d '[:space:]' < "${prefix}.row_count")" != "0" ]]; then
  echo "refusing nonempty ledger; reconcile and archive immutable monetary evidence first" >&2
  exit 2
fi

pg_dump "${DATABASE_URL}" --schema-only --no-owner --no-privileges > "${prefix}.before.sql"
sha256sum "${prefix}.before.sql" "${root}/init.sql" > "${prefix}.before.sha256"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
DROP TABLE IF EXISTS govar_frozen_cohort_slots,govar_frozen_cohorts,govar_reconciliation_tasks,govar_budget_adjustments,govar_inbox,govar_outbox,govar_settlements,govar_reservations,govar_tenants,govar_schema_metadata,govar_schema_migrations CASCADE;
COMMIT;
SQL
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${root}/init.sql"
pg_dump "${DATABASE_URL}" --schema-only --no-owner --no-privileges > "${prefix}.after.sql"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -Atc \
  "SELECT version FROM govar_schema_migrations ORDER BY version; SELECT count(*) FROM govar_tenants; SELECT count(*) FROM govar_reservations;" \
  > "${prefix}.reconciliation"
sha256sum "${prefix}.after.sql" "${prefix}.reconciliation" "${root}/init.sql" > "${prefix}.after.sha256"
echo "clean GOV-AR schema v4 migration evidence: ${prefix}.* (compatibility script name clean_migrate_v3.sh)"
