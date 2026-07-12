\set ON_ERROR_STOP on
BEGIN;
DO $$
BEGIN
 IF COALESCE((SELECT MAX(version) FROM govar_schema_migrations),0) <> 3
    OR NOT EXISTS(SELECT 1 FROM govar_schema_metadata WHERE version=3 AND layout_id='govar-v3-20260712-final') THEN
   RAISE EXCEPTION 'exact GOV-AR v3 source layout is required';
 END IF;
 IF EXISTS(SELECT 1 FROM govar_reservations) OR EXISTS(SELECT 1 FROM govar_frozen_cohorts) OR EXISTS(SELECT 1 FROM govar_frozen_cohort_slots) THEN
   RAISE EXCEPTION 'nonempty v3 reservations/cohorts cannot be backfilled; preserve as read-only audit evidence';
 END IF;
END $$;
ALTER TABLE govar_reservations ADD COLUMN route_namespace TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN selected_model_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN selected_model_generation BIGINT NOT NULL DEFAULT 1 CONSTRAINT govar_model_generation_positive CHECK(selected_model_generation>0);
ALTER TABLE govar_reservations ADD COLUMN selected_model_resource_version TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN selected_provider_name TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN selected_provider_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN selected_provider_generation BIGINT NOT NULL DEFAULT 1 CONSTRAINT govar_provider_generation_positive CHECK(selected_provider_generation>0);
ALTER TABLE govar_reservations ADD COLUMN selected_provider_resource_version TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN pricing_compliance_hash TEXT NOT NULL DEFAULT '' CONSTRAINT govar_pricing_compliance_hash_shape CHECK(pricing_compliance_hash ~ '^[0-9a-f]{64}$');
ALTER TABLE govar_reservations ADD COLUMN route_binding_name TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN route_provider_deployment TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN route_cluster TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN route_authority TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN route_path_mode TEXT NOT NULL DEFAULT 'openai-body' CONSTRAINT govar_route_path_mode_closed CHECK(route_path_mode IN('openai-body','azure-deployment-path','anthropic-body','google-generate-path'));
ALTER TABLE govar_reservations ADD COLUMN route_snapshot_hash TEXT NOT NULL DEFAULT '' CONSTRAINT govar_route_snapshot_hash_shape CHECK(route_snapshot_hash ~ '^[0-9a-f]{64}$');
ALTER TABLE govar_frozen_cohorts ADD COLUMN ledger_layout_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN route_snapshot_schema TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN software_hash TEXT NOT NULL DEFAULT '';
-- The preflight proved all affected tables empty. Remove transitional defaults
-- so a migrated v4 has the same fail-closed insert contract as fresh init.sql.
ALTER TABLE govar_reservations
  ALTER COLUMN route_namespace DROP DEFAULT,
  ALTER COLUMN selected_model_uid DROP DEFAULT,
  ALTER COLUMN selected_model_generation DROP DEFAULT,
  ALTER COLUMN selected_model_resource_version DROP DEFAULT,
  ALTER COLUMN selected_provider_name DROP DEFAULT,
  ALTER COLUMN selected_provider_uid DROP DEFAULT,
  ALTER COLUMN selected_provider_generation DROP DEFAULT,
  ALTER COLUMN selected_provider_resource_version DROP DEFAULT,
  ALTER COLUMN pricing_compliance_hash DROP DEFAULT,
  ALTER COLUMN route_binding_name DROP DEFAULT,
  ALTER COLUMN route_provider_deployment DROP DEFAULT,
  ALTER COLUMN route_cluster DROP DEFAULT,
  ALTER COLUMN route_authority DROP DEFAULT,
  ALTER COLUMN route_path_mode DROP DEFAULT,
  ALTER COLUMN route_snapshot_hash DROP DEFAULT;
ALTER TABLE govar_frozen_cohorts
  ALTER COLUMN ledger_layout_id DROP DEFAULT,
  ALTER COLUMN route_snapshot_schema DROP DEFAULT,
  ALTER COLUMN software_hash DROP DEFAULT;
INSERT INTO govar_schema_migrations(version) VALUES(4);
DELETE FROM govar_schema_metadata WHERE version=3;
INSERT INTO govar_schema_metadata(version,layout_id) VALUES(4,'govar-v4-route-snapshot-20260712');
COMMIT;
