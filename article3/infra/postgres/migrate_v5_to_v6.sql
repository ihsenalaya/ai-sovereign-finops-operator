\set ON_ERROR_STOP on
BEGIN;
DO $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM govar_schema_migrations WHERE version=5)
    OR NOT EXISTS(SELECT 1 FROM govar_schema_metadata WHERE version=5 AND layout_id='govar-v5-complete-liability-20260713') THEN
    RAISE EXCEPTION 'source is not the exact GOV-AR v5 complete-liability layout';
  END IF;
  IF EXISTS(SELECT 1 FROM govar_reservations) THEN
    RAISE EXCEPTION 'v5 reservations cannot receive a retrospectively fabricated transition chain';
  END IF;
END $$;
ALTER TABLE govar_reservations ADD COLUMN calibration_artifact_sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD CONSTRAINT govar_calibration_artifact_hash_shape CHECK(calibration_artifact_sha256='' OR calibration_artifact_sha256 ~ '^[0-9a-f]{64}$');
CREATE TABLE govar_audit_tenant_sequences(tenant_id TEXT PRIMARY KEY,next_sequence BIGINT NOT NULL CHECK(next_sequence>0));
CREATE TABLE govar_audit_events(
 tenant_id TEXT NOT NULL,sequence BIGINT NOT NULL CHECK(sequence>0),event_id TEXT NOT NULL UNIQUE,event_kind TEXT NOT NULL CHECK(event_kind IN('RESERVE','DISPATCH','SETTLE','CANCEL','EXPIRY','ROLLOVER','BUDGET_ADJUSTMENT','COHORT_REGISTRATION','CALIBRATION_PUBLICATION','DRIFT_CHANGE','RECONCILIATION')),
 payload_sha256 TEXT NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),request_id TEXT NOT NULL DEFAULT '',workload_uid TEXT NOT NULL DEFAULT '',provider_attempt_id TEXT NOT NULL DEFAULT '',actor_class TEXT NOT NULL CHECK(actor_class IN('ADMISSION','GATEWAY','RECONCILER','CORRECTION','AUTHORITY','REGISTRY','CALIBRATION')),reason TEXT NOT NULL CHECK(length(reason) BETWEEN 1 AND 64 AND reason ~ '^[a-z0-9_:-]+$'),
 before_state_sha256 TEXT NOT NULL CHECK(before_state_sha256='' OR before_state_sha256 ~ '^[0-9a-f]{64}$'),after_state_sha256 TEXT NOT NULL CHECK(after_state_sha256 ~ '^[0-9a-f]{64}$'),policy_version TEXT NOT NULL DEFAULT '',pricing_snapshot_sha256 TEXT NOT NULL DEFAULT '' CHECK(pricing_snapshot_sha256='' OR pricing_snapshot_sha256 ~ '^[0-9a-f]{64}$'),route_snapshot_sha256 TEXT NOT NULL DEFAULT '' CHECK(route_snapshot_sha256='' OR route_snapshot_sha256 ~ '^[0-9a-f]{64}$'),cap_evidence_sha256 TEXT NOT NULL DEFAULT '' CHECK(cap_evidence_sha256='' OR cap_evidence_sha256 ~ '^[0-9a-f]{64}$'),cohort_sha256 TEXT NOT NULL DEFAULT '' CHECK(cohort_sha256='' OR cohort_sha256 ~ '^[0-9a-f]{64}$'),software_sha256 TEXT NOT NULL DEFAULT '' CHECK(software_sha256='' OR software_sha256 ~ '^[0-9a-f]{64}$'),calibration_sha256 TEXT NOT NULL DEFAULT '' CHECK(calibration_sha256='' OR calibration_sha256 ~ '^[0-9a-f]{64}$'),previous_entry_sha256 TEXT NOT NULL CHECK(previous_entry_sha256='' OR previous_entry_sha256 ~ '^[0-9a-f]{64}$'),committed_at TIMESTAMPTZ NOT NULL,entry_sha256 TEXT NOT NULL UNIQUE CHECK(entry_sha256 ~ '^[0-9a-f]{64}$'),PRIMARY KEY(tenant_id,sequence));
CREATE FUNCTION govar_reject_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'govar_audit_events is append-only'; END $$;
CREATE FUNCTION govar_validate_audit_append() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE prior govar_audit_events%ROWTYPE;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id,0));
 SELECT * INTO prior FROM govar_audit_events WHERE tenant_id=NEW.tenant_id ORDER BY sequence DESC LIMIT 1;
 IF NOT FOUND THEN
  IF NEW.sequence<>1 OR NEW.previous_entry_sha256<>'' THEN RAISE EXCEPTION 'first audit entry must start a contiguous tenant chain'; END IF;
 ELSE
  IF NEW.sequence<>prior.sequence+1 OR NEW.previous_entry_sha256<>prior.entry_sha256 THEN RAISE EXCEPTION 'audit entry does not continue the contiguous tenant chain'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER govar_audit_validate_append BEFORE INSERT ON govar_audit_events FOR EACH ROW EXECUTE FUNCTION govar_validate_audit_append();
CREATE TRIGGER govar_audit_no_mutation BEFORE UPDATE OR DELETE ON govar_audit_events FOR EACH ROW EXECUTE FUNCTION govar_reject_audit_mutation();
CREATE TRIGGER govar_audit_no_truncate BEFORE TRUNCATE ON govar_audit_events FOR EACH STATEMENT EXECUTE FUNCTION govar_reject_audit_mutation();
REVOKE UPDATE,DELETE,TRUNCATE ON govar_audit_events FROM PUBLIC;
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='govar_runtime') THEN CREATE ROLE govar_runtime NOLOGIN; END IF; END $$;
GRANT govar_runtime TO CURRENT_USER;
GRANT SELECT,INSERT ON govar_audit_events TO govar_runtime;
GRANT SELECT,INSERT,UPDATE ON govar_audit_tenant_sequences TO govar_runtime;
GRANT SELECT ON govar_schema_migrations,govar_schema_metadata TO govar_runtime;
GRANT SELECT,INSERT,UPDATE ON govar_tenants,govar_reservations,govar_outbox,govar_inbox,govar_budget_adjustments,govar_reconciliation_tasks,govar_frozen_cohorts,govar_frozen_cohort_slots TO govar_runtime;
DELETE FROM govar_schema_metadata WHERE version=5;
INSERT INTO govar_schema_metadata(version,layout_id) VALUES(6,'govar-v6-transition-audit-20260713');
INSERT INTO govar_schema_migrations(version) VALUES(6);
COMMIT;
