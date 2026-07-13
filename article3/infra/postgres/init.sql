\set ON_ERROR_STOP on
BEGIN;
CREATE TABLE govar_schema_migrations(version INTEGER PRIMARY KEY,applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE govar_schema_metadata(version INTEGER PRIMARY KEY,layout_id TEXT NOT NULL);
CREATE TABLE govar_tenants(
 tenant_id TEXT PRIMARY KEY,budget_micros BIGINT NOT NULL DEFAULT 0 CHECK(budget_micros>=0),
 settled_micros BIGINT NOT NULL DEFAULT 0 CHECK(settled_micros>=0),reserved_micros BIGINT NOT NULL DEFAULT 0 CHECK(reserved_micros>=0),
 carried_adjustment_micros BIGINT NOT NULL DEFAULT 0 CHECK(carried_adjustment_micros>=0),active_reservations INTEGER NOT NULL DEFAULT 0 CHECK(active_reservations>=0),
 budget_identity TEXT NOT NULL,current_window_id TEXT NOT NULL DEFAULT '',window_period TEXT NOT NULL DEFAULT '',historical_credit_micros BIGINT NOT NULL DEFAULT 0 CHECK(historical_credit_micros>=0),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE govar_reservations(
 request_id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,workload_uid TEXT NOT NULL,selected_deployment TEXT NOT NULL,
 provider_attempt_id TEXT NOT NULL UNIQUE,outbox_id TEXT NOT NULL UNIQUE,outbox_state TEXT NOT NULL,state TEXT NOT NULL,
 reserved_cost_micros BIGINT NOT NULL CHECK(reserved_cost_micros>=0),provisional_cost_micros BIGINT NOT NULL DEFAULT 0 CHECK(provisional_cost_micros>=0),
 base_actual_micros BIGINT NOT NULL DEFAULT 0 CHECK(base_actual_micros>=0),settled_effect_micros BIGINT NOT NULL DEFAULT 0 CHECK(settled_effect_micros>=0),residual_hold_micros BIGINT NOT NULL CHECK(residual_hold_micros>=0),usage_version BIGINT NOT NULL DEFAULT 0 CHECK(usage_version>=0),finalized BOOLEAN NOT NULL DEFAULT FALSE,
 policy_version TEXT NOT NULL,pricing_version TEXT NOT NULL,reservation_mode TEXT NOT NULL,risk_level TEXT NOT NULL,allocated_risk_ppb BIGINT NOT NULL DEFAULT 0 CHECK(allocated_risk_ppb>=0),
 input_price_micros_per_million BIGINT NOT NULL DEFAULT 0,output_price_micros_per_million BIGINT NOT NULL DEFAULT 0,admission_fingerprint TEXT NOT NULL,candidate_snapshot_version TEXT NOT NULL,
 cohort_id TEXT NOT NULL DEFAULT '',cohort_index BIGINT NOT NULL DEFAULT 0,cohort_registry_digest TEXT NOT NULL DEFAULT '',origin_window_id TEXT NOT NULL DEFAULT '',enforcement_window_id TEXT NOT NULL DEFAULT '',
 rollover_guard_micros BIGINT NOT NULL DEFAULT 0 CHECK(rollover_guard_micros>=0),carry_effect_micros BIGINT NOT NULL DEFAULT 0 CHECK(carry_effect_micros>=0),historical_credit_micros BIGINT NOT NULL DEFAULT 0 CHECK(historical_credit_micros>=0),carried BOOLEAN NOT NULL DEFAULT FALSE,last_usage_event_id TEXT NOT NULL DEFAULT '',provider_retry_policy TEXT NOT NULL DEFAULT 'NO_PROVIDER_RETRY',last_reason_code TEXT NOT NULL DEFAULT '',last_transition_event_id TEXT NOT NULL DEFAULT '',
 route_namespace TEXT NOT NULL,selected_model_uid TEXT NOT NULL,selected_model_generation BIGINT NOT NULL CONSTRAINT govar_model_generation_positive CHECK(selected_model_generation>0),selected_model_resource_version TEXT NOT NULL,
 selected_provider_name TEXT NOT NULL,selected_provider_uid TEXT NOT NULL,selected_provider_generation BIGINT NOT NULL CONSTRAINT govar_provider_generation_positive CHECK(selected_provider_generation>0),selected_provider_resource_version TEXT NOT NULL,
 pricing_compliance_hash TEXT NOT NULL CONSTRAINT govar_pricing_compliance_hash_shape CHECK(pricing_compliance_hash ~ '^[0-9a-f]{64}$'),route_binding_name TEXT NOT NULL,route_provider_deployment TEXT NOT NULL,route_cluster TEXT NOT NULL,route_authority TEXT NOT NULL,
 route_path_mode TEXT NOT NULL CONSTRAINT govar_route_path_mode_closed CHECK(route_path_mode IN('openai-body','azure-deployment-path','anthropic-body','google-generate-path')),route_snapshot_hash TEXT NOT NULL CONSTRAINT govar_route_snapshot_hash_shape CHECK(route_snapshot_hash ~ '^[0-9a-f]{64}$'),
 pricing_snapshot_json JSONB NOT NULL,reserved_components_json JSONB NOT NULL,actual_components_json JSONB NOT NULL DEFAULT '[]'::jsonb,missing_usage_bases_json JSONB NOT NULL DEFAULT '[]'::jsonb,
 pricing_snapshot_sha256 TEXT NOT NULL CHECK(pricing_snapshot_sha256 ~ '^[0-9a-f]{64}$'),cap_evidence_sha256 TEXT NOT NULL CHECK(cap_evidence_sha256 ~ '^[0-9a-f]{64}$'),component_bound_exceeded BOOLEAN NOT NULL DEFAULT FALSE,calibration_artifact_sha256 TEXT NOT NULL DEFAULT '' CONSTRAINT govar_calibration_artifact_hash_shape CHECK(calibration_artifact_sha256='' OR calibration_artifact_sha256 ~ '^[0-9a-f]{64}$'),
 expiry TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),CONSTRAINT govar_reservation_tenant_fk FOREIGN KEY(tenant_id) REFERENCES govar_tenants(tenant_id) ON DELETE RESTRICT);
CREATE UNIQUE INDEX govar_reservations_cohort_opportunity_idx ON govar_reservations(tenant_id,cohort_id,cohort_index) WHERE cohort_id<>'';
CREATE TABLE govar_outbox(outbox_id TEXT PRIMARY KEY,request_id TEXT NOT NULL UNIQUE,tenant_id TEXT NOT NULL,workload_uid TEXT NOT NULL,provider_attempt_id TEXT NOT NULL UNIQUE,state TEXT NOT NULL,version BIGINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),CONSTRAINT govar_outbox_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT);
CREATE TABLE govar_inbox(event_id TEXT PRIMARY KEY,request_id TEXT NOT NULL,event_kind TEXT NOT NULL,payload_hash TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),CONSTRAINT govar_inbox_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT);
CREATE TABLE govar_budget_adjustments(adjustment_id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,window_id TEXT NOT NULL,new_budget_micros BIGINT NOT NULL CHECK(new_budget_micros>=0),debt_payment_micros BIGINT NOT NULL CHECK(debt_payment_micros>=0),authorized_by TEXT NOT NULL,reason TEXT NOT NULL,payload_hash TEXT NOT NULL,authority_key_id TEXT NOT NULL,authority_proof TEXT NOT NULL,approved_at TIMESTAMPTZ NOT NULL,expires_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),CONSTRAINT govar_adjustment_tenant_fk FOREIGN KEY(tenant_id) REFERENCES govar_tenants(tenant_id) ON DELETE RESTRICT);
CREATE TABLE govar_reconciliation_tasks(task_id TEXT PRIMARY KEY,request_id TEXT NOT NULL,reason_code TEXT NOT NULL,state TEXT NOT NULL,payload_hash TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),completed_at TIMESTAMPTZ,CONSTRAINT govar_reconciliation_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT);
CREATE TABLE govar_frozen_cohorts(tenant_id TEXT NOT NULL,cohort_id TEXT NOT NULL,size BIGINT NOT NULL CHECK(size>0),tenant_risk_ppb BIGINT NOT NULL CHECK(tenant_risk_ppb BETWEEN 0 AND 1000000000),data_hash TEXT NOT NULL,config_hash TEXT NOT NULL,protocol_hash TEXT NOT NULL,frozen_at TIMESTAMPTZ NOT NULL,registered_at TIMESTAMPTZ NOT NULL,registry_digest TEXT NOT NULL UNIQUE,authority_key_id TEXT NOT NULL,authority_proof TEXT NOT NULL,ledger_layout_id TEXT NOT NULL,route_snapshot_schema TEXT NOT NULL,software_hash TEXT NOT NULL,PRIMARY KEY(tenant_id,cohort_id));
CREATE TABLE govar_frozen_cohort_slots(tenant_id TEXT NOT NULL,cohort_id TEXT NOT NULL,slot_index BIGINT NOT NULL,request_id TEXT NOT NULL,opportunity_digest TEXT NOT NULL,weight_ppb BIGINT NOT NULL CHECK(weight_ppb>=0),PRIMARY KEY(tenant_id,cohort_id,slot_index),UNIQUE(tenant_id,cohort_id,request_id),FOREIGN KEY(tenant_id,cohort_id) REFERENCES govar_frozen_cohorts(tenant_id,cohort_id) ON DELETE RESTRICT);
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
INSERT INTO govar_schema_migrations(version) VALUES(6);
INSERT INTO govar_schema_metadata(version,layout_id) VALUES(6,'govar-v6-transition-audit-20260713');
COMMIT;
