#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="${root}/../../tests/fixtures/govar_schema_v3.sql"

psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 <<'SQL'
DROP TABLE IF EXISTS govar_audit_events,govar_audit_tenant_sequences,govar_frozen_cohort_slots,govar_frozen_cohorts,govar_reconciliation_tasks,govar_budget_adjustments,govar_inbox,govar_outbox,govar_settlements,govar_reservations,govar_tenants,govar_schema_metadata,govar_schema_migrations CASCADE;
DROP FUNCTION IF EXISTS govar_reject_audit_mutation() CASCADE;
DROP FUNCTION IF EXISTS govar_validate_audit_append() CASCADE;
DROP ROLE IF EXISTS govar_runtime;
SQL
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${fixture}"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${root}/migrate_v3_to_v4.sql"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${root}/migrate_v4_to_v5.sql"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${root}/migrate_v5_to_v6.sql"

actual="$(psql "${DATABASE_URL}" -X -qAt -v ON_ERROR_STOP=1 <<'SQL'
SELECT max(m.version),md.layout_id FROM govar_schema_migrations m CROSS JOIN govar_schema_metadata md GROUP BY md.layout_id;
SELECT count(*) FROM govar_reservations;
SELECT count(*) FROM govar_audit_events;
SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='govar_reservations' AND column_name IN ('route_snapshot_hash','pricing_snapshot_json','calibration_artifact_sha256');
SQL
)"
expected=$'6|govar-v6-transition-audit-20260713\n0\n0\n3'
if [[ "${actual}" != "${expected}" ]]; then
  printf 'migration chain mismatch\nexpected:\n%s\nactual:\n%s\n' "${expected}" "${actual}" >&2
  exit 1
fi
sha256sum "${fixture}" "${root}/migrate_v3_to_v4.sql" "${root}/migrate_v4_to_v5.sql" "${root}/migrate_v5_to_v6.sql"
