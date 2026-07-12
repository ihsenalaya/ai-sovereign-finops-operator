#!/usr/bin/env bash
set -euo pipefail
: "${DATABASE_URL:?DATABASE_URL is required}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
evidence_dir="${1:-${root}/migration-v4-evidence}"
mkdir -p "${evidence_dir}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
prefix="${evidence_dir}/${stamp}"
pg_dump "${DATABASE_URL}" --schema-only --no-owner --no-privileges > "${prefix}.before.sql"
psql "${DATABASE_URL}" -X -At -v ON_ERROR_STOP=1 -c \
  "SELECT COALESCE((SELECT count(*) FROM govar_reservations),0),COALESCE((SELECT count(*) FROM govar_frozen_cohorts),0),COALESCE((SELECT count(*) FROM govar_frozen_cohort_slots),0)" > "${prefix}.preflight-counts"
sha256sum "${prefix}.before.sql" "${prefix}.preflight-counts" "${root}/migrate_v3_to_v4.sql" > "${prefix}.before.sha256"
psql "${DATABASE_URL}" -X -v ON_ERROR_STOP=1 -f "${root}/migrate_v3_to_v4.sql"
pg_dump "${DATABASE_URL}" --schema-only --no-owner --no-privileges > "${prefix}.after.sql"
psql "${DATABASE_URL}" -X -At -v ON_ERROR_STOP=1 -c \
  "SELECT max(version),(SELECT layout_id FROM govar_schema_metadata WHERE version=4) FROM govar_schema_migrations" > "${prefix}.result"
sha256sum "${prefix}.after.sql" "${prefix}.result" "${root}/migrate_v3_to_v4.sql" > "${prefix}.after.sha256"
echo "GOV-AR v3-to-v4 migration evidence: ${prefix}.*"
