\set ON_ERROR_STOP on
BEGIN;
DO $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM govar_schema_migrations WHERE version=4)
    OR NOT EXISTS(SELECT 1 FROM govar_schema_metadata WHERE version=4 AND layout_id='govar-v4-route-snapshot-20260712') THEN
    RAISE EXCEPTION 'source is not the exact GOV-AR v4 route-snapshot layout';
  END IF;
  IF EXISTS(SELECT 1 FROM govar_reservations) THEN
    RAISE EXCEPTION 'v4 reservations cannot be assigned immutable component snapshots retroactively';
  END IF;
END $$;
ALTER TABLE govar_reservations ADD COLUMN pricing_snapshot_json JSONB;
ALTER TABLE govar_reservations ADD COLUMN reserved_components_json JSONB;
ALTER TABLE govar_reservations ADD COLUMN actual_components_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE govar_reservations ADD COLUMN missing_usage_bases_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE govar_reservations ADD COLUMN pricing_snapshot_sha256 TEXT;
ALTER TABLE govar_reservations ADD COLUMN cap_evidence_sha256 TEXT;
ALTER TABLE govar_reservations ADD COLUMN component_bound_exceeded BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE govar_reservations ALTER COLUMN pricing_snapshot_json SET NOT NULL;
ALTER TABLE govar_reservations ALTER COLUMN reserved_components_json SET NOT NULL;
ALTER TABLE govar_reservations ALTER COLUMN pricing_snapshot_sha256 SET NOT NULL;
ALTER TABLE govar_reservations ALTER COLUMN cap_evidence_sha256 SET NOT NULL;
ALTER TABLE govar_reservations ADD CONSTRAINT govar_pricing_snapshot_hash_shape CHECK(pricing_snapshot_sha256 ~ '^[0-9a-f]{64}$');
ALTER TABLE govar_reservations ADD CONSTRAINT govar_cap_evidence_hash_shape CHECK(cap_evidence_sha256 ~ '^[0-9a-f]{64}$');
DELETE FROM govar_schema_metadata WHERE version=4;
INSERT INTO govar_schema_metadata(version,layout_id) VALUES(5,'govar-v5-complete-liability-20260713');
INSERT INTO govar_schema_migrations(version) VALUES(5);
COMMIT;
