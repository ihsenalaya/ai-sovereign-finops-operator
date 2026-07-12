package govar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

type PostgresEngine struct {
	pool               *pgxpool.Pool
	now                func() time.Time
	authorityKeyID     string
	authorityKey       []byte
	cohortSoftwareHash string
}

func NewPostgresEngine(ctx context.Context, databaseURL string) (*PostgresEngine, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	engine := &PostgresEngine{pool: pool, now: time.Now}
	if err := engine.initSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return engine, nil
}

func (e *PostgresEngine) Close() {
	if e != nil && e.pool != nil {
		e.pool.Close()
	}
}

func (e *PostgresEngine) Ready(ctx context.Context) error { return e.pool.Ping(ctx) }

func (e *PostgresEngine) initSchema(ctx context.Context) error {
	// Fail closed rather than silently converting or zeroing legacy float state.
	schema := `
DO $$
DECLARE recorded_version INTEGER;
BEGIN
  -- Runtime bootstrap may create a wholly fresh schema or verify the one exact
  -- layout this binary understands.  It is not a migration engine: even an
  -- empty earlier layout must go through the evidence-producing clean
  -- migration command so startup cannot silently mutate schema semantics.
  IF to_regclass('govar_schema_migrations') IS NOT NULL THEN
    EXECUTE 'SELECT COALESCE(MAX(version),0) FROM govar_schema_migrations' INTO recorded_version;
	    IF recorded_version <> 4 THEN
	      RAISE EXCEPTION 'existing GOV-AR schema version % is incompatible with v4; use reviewed clean migration', recorded_version;
    END IF;
  ELSIF to_regclass('govar_tenants') IS NOT NULL
     OR to_regclass('govar_schema_metadata') IS NOT NULL
     OR to_regclass('govar_reservations') IS NOT NULL
     OR to_regclass('govar_settlements') IS NOT NULL
     OR to_regclass('govar_outbox') IS NOT NULL
     OR to_regclass('govar_inbox') IS NOT NULL
     OR to_regclass('govar_budget_adjustments') IS NOT NULL
     OR to_regclass('govar_reconciliation_tasks') IS NOT NULL
     OR to_regclass('govar_frozen_cohorts') IS NOT NULL
     OR to_regclass('govar_frozen_cohort_slots') IS NOT NULL THEN
    RAISE EXCEPTION 'unversioned existing GOV-AR schema is incompatible; use reviewed clean migration';
  END IF;
END $$;
CREATE TABLE IF NOT EXISTS govar_schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $$
DECLARE legacy_exists BOOLEAN;
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_tenants' AND column_name='budget_eur') THEN
	RAISE EXCEPTION 'legacy floating-point tenant layout is incompatible; use reviewed clean migration';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_reservations' AND column_name='reserved_cost') THEN
	RAISE EXCEPTION 'legacy floating-point reservation layout is incompatible; use reviewed clean migration';
  END IF;
  IF to_regclass('govar_settlements') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS(SELECT 1 FROM govar_settlements)' INTO legacy_exists;
    IF legacy_exists THEN RAISE EXCEPTION 'unreconciled legacy settlement rows exist'; END IF;
  END IF;
END $$;
DO $$ DECLARE ok BOOLEAN; BEGIN
	 IF COALESCE((SELECT MAX(version) FROM govar_schema_migrations),0)=4 THEN
  IF to_regclass('govar_schema_metadata') IS NULL THEN RAISE EXCEPTION 'v4 schema metadata missing'; END IF;
  EXECUTE 'SELECT EXISTS(SELECT 1 FROM govar_schema_metadata WHERE version=4 AND layout_id=''govar-v4-route-snapshot-20260712'')' INTO ok;
  IF NOT ok OR NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='govar_reservations' AND column_name='route_snapshot_hash')
    OR NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='govar_frozen_cohorts' AND column_name='software_hash') THEN
    RAISE EXCEPTION 'v4 schema layout identifier or route columns mismatch';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_reservation_tenant_fk' AND conrelid='govar_reservations'::regclass AND contype='f')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_outbox_request_fk' AND conrelid='govar_outbox'::regclass AND contype='f')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_inbox_request_fk' AND conrelid='govar_inbox'::regclass AND contype='f')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_adjustment_tenant_fk' AND conrelid='govar_budget_adjustments'::regclass AND contype='f')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_reconciliation_request_fk' AND conrelid='govar_reconciliation_tasks'::regclass AND contype='f')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_model_generation_positive')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_provider_generation_positive')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_route_snapshot_hash_shape')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_pricing_compliance_hash_shape')
    OR NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_route_path_mode_closed') THEN
    RAISE EXCEPTION 'v4 schema constraint identity mismatch';
  END IF;
 END IF;
END $$;
CREATE TABLE IF NOT EXISTS govar_tenants (
  tenant_id TEXT PRIMARY KEY,
  budget_micros BIGINT NOT NULL DEFAULT 0 CHECK (budget_micros >= 0),
  settled_micros BIGINT NOT NULL DEFAULT 0 CHECK (settled_micros >= 0),
  reserved_micros BIGINT NOT NULL DEFAULT 0 CHECK (reserved_micros >= 0),
  carried_adjustment_micros BIGINT NOT NULL DEFAULT 0 CHECK (carried_adjustment_micros >= 0),
  active_reservations INTEGER NOT NULL DEFAULT 0 CHECK (active_reservations >= 0),
  budget_identity TEXT NOT NULL,
	current_window_id TEXT NOT NULL DEFAULT '',
	window_period TEXT NOT NULL DEFAULT '',
	historical_credit_micros BIGINT NOT NULL DEFAULT 0 CHECK (historical_credit_micros >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS budget_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS settled_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS reserved_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS carried_adjustment_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS budget_identity TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS current_window_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS window_period TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS historical_credit_micros BIGINT NOT NULL DEFAULT 0;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='govar_carried_adjustment_nonnegative') THEN
    ALTER TABLE govar_tenants ADD CONSTRAINT govar_carried_adjustment_nonnegative CHECK (carried_adjustment_micros >= 0);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS govar_reservations (
  request_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  workload_uid TEXT NOT NULL,
  selected_deployment TEXT NOT NULL,
  provider_attempt_id TEXT NOT NULL UNIQUE,
  outbox_id TEXT NOT NULL UNIQUE,
  outbox_state TEXT NOT NULL,
  state TEXT NOT NULL,
  reserved_cost_micros BIGINT NOT NULL CHECK (reserved_cost_micros >= 0),
  provisional_cost_micros BIGINT NOT NULL DEFAULT 0 CHECK (provisional_cost_micros >= 0),
	base_actual_micros BIGINT NOT NULL DEFAULT 0 CHECK (base_actual_micros >= 0),
	settled_effect_micros BIGINT NOT NULL DEFAULT 0 CHECK(settled_effect_micros>=0),
  residual_hold_micros BIGINT NOT NULL CHECK (residual_hold_micros >= 0),
  usage_version BIGINT NOT NULL DEFAULT 0 CHECK (usage_version >= 0),
  finalized BOOLEAN NOT NULL DEFAULT FALSE,
  policy_version TEXT NOT NULL,
  pricing_version TEXT NOT NULL,
  reservation_mode TEXT NOT NULL,
  risk_level TEXT NOT NULL,
  allocated_risk_ppb BIGINT NOT NULL DEFAULT 0 CHECK (allocated_risk_ppb >= 0),
  input_price_micros_per_million BIGINT NOT NULL DEFAULT 0,
  output_price_micros_per_million BIGINT NOT NULL DEFAULT 0,
  admission_fingerprint TEXT NOT NULL,
  candidate_snapshot_version TEXT NOT NULL,
  cohort_id TEXT NOT NULL DEFAULT '',
  cohort_index BIGINT NOT NULL DEFAULT 0,
	cohort_registry_digest TEXT NOT NULL DEFAULT '',
	origin_window_id TEXT NOT NULL DEFAULT '',
	enforcement_window_id TEXT NOT NULL DEFAULT '',
	rollover_guard_micros BIGINT NOT NULL DEFAULT 0 CHECK (rollover_guard_micros >= 0),
	carry_effect_micros BIGINT NOT NULL DEFAULT 0 CHECK (carry_effect_micros >= 0),
	historical_credit_micros BIGINT NOT NULL DEFAULT 0 CHECK (historical_credit_micros >= 0),
	carried BOOLEAN NOT NULL DEFAULT FALSE,
	last_usage_event_id TEXT NOT NULL DEFAULT '',
	provider_retry_policy TEXT NOT NULL DEFAULT 'NO_PROVIDER_RETRY',
	last_reason_code TEXT NOT NULL DEFAULT '',last_transition_event_id TEXT NOT NULL DEFAULT '',
	route_namespace TEXT NOT NULL,selected_model_uid TEXT NOT NULL,selected_model_generation BIGINT NOT NULL CHECK(selected_model_generation>0),selected_model_resource_version TEXT NOT NULL,
	selected_provider_name TEXT NOT NULL,selected_provider_uid TEXT NOT NULL,selected_provider_generation BIGINT NOT NULL CHECK(selected_provider_generation>0),selected_provider_resource_version TEXT NOT NULL,
	pricing_compliance_hash TEXT NOT NULL CHECK(pricing_compliance_hash ~ '^[0-9a-f]{64}$'),route_binding_name TEXT NOT NULL,route_provider_deployment TEXT NOT NULL,
	route_cluster TEXT NOT NULL,route_authority TEXT NOT NULL,route_path_mode TEXT NOT NULL CHECK(route_path_mode IN('openai-body','azure-deployment-path','anthropic-body','google-generate-path')),
	route_snapshot_hash TEXT NOT NULL CHECK(route_snapshot_hash ~ '^[0-9a-f]{64}$'),
  expiry TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS workload_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS provider_attempt_id TEXT;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS outbox_id TEXT;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS outbox_state TEXT NOT NULL DEFAULT 'PENDING';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT 'RESERVED';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS reserved_cost_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS provisional_cost_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS base_actual_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS settled_effect_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS residual_hold_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS usage_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS finalized BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS allocated_risk_ppb BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS input_price_micros_per_million BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS output_price_micros_per_million BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS admission_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS candidate_snapshot_version TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS cohort_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS cohort_index BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS cohort_registry_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS origin_window_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS enforcement_window_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS rollover_guard_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS carry_effect_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS historical_credit_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS carried BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS last_usage_event_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS provider_retry_policy TEXT NOT NULL DEFAULT 'NO_PROVIDER_RETRY';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS last_reason_code TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS last_transition_event_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_namespace TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_model_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_model_generation BIGINT NOT NULL DEFAULT 1;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_model_resource_version TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_provider_name TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_provider_uid TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_provider_generation BIGINT NOT NULL DEFAULT 1;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS selected_provider_resource_version TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS pricing_compliance_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_binding_name TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_provider_deployment TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_cluster TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_authority TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_path_mode TEXT NOT NULL DEFAULT 'openai-body';
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS route_snapshot_hash TEXT NOT NULL DEFAULT '';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM govar_reservations WHERE route_snapshot_hash !~ '^[0-9a-f]{64}$' OR pricing_compliance_hash !~ '^[0-9a-f]{64}$') THEN RAISE EXCEPTION 'unreconciled reservation route snapshot';END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_model_generation_positive') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_model_generation_positive CHECK(selected_model_generation>0);END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_provider_generation_positive') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_provider_generation_positive CHECK(selected_provider_generation>0);END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_pricing_compliance_hash_shape') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_pricing_compliance_hash_shape CHECK(pricing_compliance_hash ~ '^[0-9a-f]{64}$');END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_route_snapshot_hash_shape') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_route_snapshot_hash_shape CHECK(route_snapshot_hash ~ '^[0-9a-f]{64}$');END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_route_path_mode_closed') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_route_path_mode_closed CHECK(route_path_mode IN('openai-body','azure-deployment-path','anthropic-body','google-generate-path'));END IF;
END $$;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM govar_reservations WHERE admission_fingerprint='') THEN
    RAISE EXCEPTION 'unreconciled reservation rows without immutable admission fingerprint exist';
  END IF;
END $$;
ALTER TABLE govar_reservations ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE UNIQUE INDEX IF NOT EXISTS govar_reservations_provider_attempt_idx
  ON govar_reservations(provider_attempt_id) WHERE provider_attempt_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS govar_reservations_outbox_idx
  ON govar_reservations(outbox_id) WHERE outbox_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS govar_reservations_cohort_opportunity_idx
  ON govar_reservations(tenant_id,cohort_id,cohort_index) WHERE cohort_id <> '';

CREATE TABLE IF NOT EXISTS govar_outbox (
  outbox_id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL UNIQUE,
  tenant_id TEXT NOT NULL,
  workload_uid TEXT NOT NULL,
  provider_attempt_id TEXT NOT NULL UNIQUE,
  state TEXT NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS govar_inbox (
  event_id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL,
  event_kind TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS govar_budget_adjustments(
 adjustment_id TEXT PRIMARY KEY,tenant_id TEXT NOT NULL,window_id TEXT NOT NULL,
 new_budget_micros BIGINT NOT NULL CHECK(new_budget_micros>=0),debt_payment_micros BIGINT NOT NULL CHECK(debt_payment_micros>=0),
 authorized_by TEXT NOT NULL,reason TEXT NOT NULL,payload_hash TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	,authority_key_id TEXT NOT NULL,authority_proof TEXT NOT NULL,approved_at TIMESTAMPTZ NOT NULL,expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS govar_reconciliation_tasks(task_id TEXT PRIMARY KEY,request_id TEXT NOT NULL,reason_code TEXT NOT NULL,state TEXT NOT NULL,payload_hash TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),completed_at TIMESTAMPTZ);
CREATE TABLE IF NOT EXISTS govar_schema_metadata(version INTEGER PRIMARY KEY,layout_id TEXT NOT NULL);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_reservation_tenant_fk') THEN ALTER TABLE govar_reservations ADD CONSTRAINT govar_reservation_tenant_fk FOREIGN KEY(tenant_id) REFERENCES govar_tenants(tenant_id) ON DELETE RESTRICT; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_outbox_request_fk') THEN ALTER TABLE govar_outbox ADD CONSTRAINT govar_outbox_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_inbox_request_fk') THEN ALTER TABLE govar_inbox ADD CONSTRAINT govar_inbox_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_adjustment_tenant_fk') THEN ALTER TABLE govar_budget_adjustments ADD CONSTRAINT govar_adjustment_tenant_fk FOREIGN KEY(tenant_id) REFERENCES govar_tenants(tenant_id) ON DELETE RESTRICT; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='govar_reconciliation_request_fk') THEN ALTER TABLE govar_reconciliation_tasks ADD CONSTRAINT govar_reconciliation_request_fk FOREIGN KEY(request_id) REFERENCES govar_reservations(request_id) ON DELETE RESTRICT; END IF;
END $$;
ALTER TABLE govar_inbox ADD COLUMN IF NOT EXISTS payload_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS govar_frozen_cohorts (
 tenant_id TEXT NOT NULL,
 cohort_id TEXT NOT NULL,
 size BIGINT NOT NULL CHECK(size > 0),
 tenant_risk_ppb BIGINT NOT NULL CHECK(tenant_risk_ppb BETWEEN 0 AND 1000000000),
 data_hash TEXT NOT NULL, config_hash TEXT NOT NULL, protocol_hash TEXT NOT NULL,
 frozen_at TIMESTAMPTZ NOT NULL, registered_at TIMESTAMPTZ NOT NULL, registry_digest TEXT NOT NULL UNIQUE,
	authority_key_id TEXT NOT NULL,authority_proof TEXT NOT NULL,
	ledger_layout_id TEXT NOT NULL,route_snapshot_schema TEXT NOT NULL,software_hash TEXT NOT NULL,
 PRIMARY KEY(tenant_id,cohort_id)
);
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS registered_at TIMESTAMPTZ;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM govar_frozen_cohorts WHERE registered_at IS NULL) THEN
  RAISE EXCEPTION 'unreconciled frozen cohort rows without server registration time exist';
 END IF;
END $$;
ALTER TABLE govar_frozen_cohorts ALTER COLUMN registered_at SET NOT NULL;
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS authority_key_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS authority_proof TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS ledger_layout_id TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS route_snapshot_schema TEXT NOT NULL DEFAULT '';
ALTER TABLE govar_frozen_cohorts ADD COLUMN IF NOT EXISTS software_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS govar_frozen_cohort_slots (
 tenant_id TEXT NOT NULL, cohort_id TEXT NOT NULL, slot_index BIGINT NOT NULL,
 request_id TEXT NOT NULL, opportunity_digest TEXT NOT NULL, weight_ppb BIGINT NOT NULL CHECK(weight_ppb >= 0),
 PRIMARY KEY(tenant_id,cohort_id,slot_index), UNIQUE(tenant_id,cohort_id,request_id),
 FOREIGN KEY(tenant_id,cohort_id) REFERENCES govar_frozen_cohorts(tenant_id,cohort_id) ON DELETE RESTRICT
);
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_reservations' AND column_name='reserved_cost') THEN
    ALTER TABLE govar_reservations ALTER COLUMN reserved_cost SET DEFAULT 0;
  END IF;
END $$;
DO $$
BEGIN
  IF COALESCE((SELECT MAX(version) FROM govar_schema_migrations),0) > 4 THEN
    RAISE EXCEPTION 'database schema is newer than supported v4; binary rollback refused';
  END IF;
  IF COALESCE((SELECT MAX(version) FROM govar_schema_migrations),0) < 3
     AND (EXISTS(SELECT 1 FROM govar_tenants) OR EXISTS(SELECT 1 FROM govar_reservations)
          OR EXISTS(SELECT 1 FROM govar_outbox) OR EXISTS(SELECT 1 FROM govar_inbox)
          OR EXISTS(SELECT 1 FROM govar_frozen_cohorts) OR EXISTS(SELECT 1 FROM govar_budget_adjustments)) THEN
    RAISE EXCEPTION 'unreconciled pre-v3 integer ledger rows exist; window/origin/base/guard state cannot be inferred';
  END IF;
END $$;
DO $$ DECLARE t RECORD; calc_reserved BIGINT;calc_active BIGINT;calc_settled BIGINT;calc_carry BIGINT;calc_credit BIGINT;BEGIN
 FOR t IN SELECT * FROM govar_tenants LOOP
  SELECT COALESCE(SUM(residual_hold_micros+rollover_guard_micros),0),COUNT(*) INTO calc_reserved,calc_active FROM govar_reservations WHERE tenant_id=t.tenant_id AND state NOT IN('CANCELED_UNBILLED','FAILED_UNBILLED','EXPIRED_UNDISPATCHED','FINALIZED','LATE_FINALIZED');
  SELECT COALESCE(SUM(settled_effect_micros),0) INTO calc_settled FROM govar_reservations WHERE tenant_id=t.tenant_id AND carried=FALSE AND enforcement_window_id=t.current_window_id;
  SELECT COALESCE(SUM(carry_effect_micros),0)-COALESCE((SELECT SUM(debt_payment_micros) FROM govar_budget_adjustments WHERE tenant_id=t.tenant_id),0),COALESCE(SUM(historical_credit_micros),0) INTO calc_carry,calc_credit FROM govar_reservations WHERE tenant_id=t.tenant_id;
  IF (t.reserved_micros,t.active_reservations,t.settled_micros,t.carried_adjustment_micros,t.historical_credit_micros) IS DISTINCT FROM (calc_reserved,calc_active,calc_settled,calc_carry,calc_credit) THEN RAISE EXCEPTION 'tenant aggregate invariant mismatch for %',t.tenant_id; END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM govar_reservations r LEFT JOIN govar_outbox o ON o.request_id=r.request_id WHERE o.request_id IS NULL OR (r.outbox_id,r.tenant_id,r.workload_uid,r.provider_attempt_id,r.outbox_state) IS DISTINCT FROM (o.outbox_id,o.tenant_id,o.workload_uid,o.provider_attempt_id,o.state)) THEN RAISE EXCEPTION 'reservation/outbox invariant mismatch';END IF;
 IF EXISTS(SELECT 1 FROM govar_reservations WHERE residual_hold_micros+rollover_guard_micros>reserved_cost_micros AND state NOT IN('FINALIZED','LATE_FINALIZED')) THEN RAISE EXCEPTION 'reservation hold exceeds ceiling';END IF;
END $$;
INSERT INTO govar_schema_metadata(version,layout_id) VALUES(4,'govar-v4-route-snapshot-20260712') ON CONFLICT(version) DO UPDATE SET layout_id=EXCLUDED.layout_id;
INSERT INTO govar_schema_migrations(version) VALUES (4) ON CONFLICT DO NOTHING;`
	if _, err := e.pool.Exec(ctx, schema); err != nil {
		return err
	}
	return e.validateStoredRouteSnapshots(ctx)
}

func (e *PostgresEngine) validateStoredRouteSnapshots(ctx context.Context) error {
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT request_id FROM govar_reservations ORDER BY request_id`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := loadReservationTx(ctx, tx, id, false); err != nil {
			return fmt.Errorf("stored reservation %s route snapshot: %w", id, err)
		}
	}
	return tx.Commit(ctx)
}

func (e *PostgresEngine) Admit(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
	var response AdmitResponse
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		response, err = e.admitOnce(req, budget, routing, candidates)
		if !isRetryablePG(err) && !isUniqueViolationPG(err) {
			return response, err
		}
		time.Sleep(time.Duration(1<<attempt) * 10 * time.Millisecond)
	}
	return response, err
}

func (e *PostgresEngine) admitOnce(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
	if err := validateAdmitRequest(req); err != nil {
		return AdmitResponse{}, err
	}
	candidates = validatedCandidates(candidates)
	snapshot := BuildPolicySnapshot(budget, routing)
	candidates = rankedCandidates(candidates, req.InputTokens, req.MaxOutputTokens, snapshot.Objective)
	fingerprint := admissionFingerprint(req, budget, routing, candidates)
	// Check active IDs before any decision-only early return so a conflicting
	// retry cannot evade the immutable request binding.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return AdmitResponse{}, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		existing, loadErr := loadReservationTx(ctx, tx, req.RequestID, false)
		if loadErr == nil {
			if err := matchPrincipal(existing, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
				return AdmitResponse{}, err
			}
			if existing.AdmissionFingerprint != fingerprint {
				return AdmitResponse{}, errors.New("duplicate request_id has conflicting immutable admission payload")
			}
			if err := tx.Commit(ctx); err != nil {
				return AdmitResponse{}, err
			}
			if !reservationIsActive(existing.State) {
				return AdmitResponse{Decision: DecisionReject, ReasonCode: ReasonInvalidTransition, TraceID: req.RequestID}, nil
			}
			return responseForReservation(existing, ReasonDuplicateRequest), nil
		}
		if !errors.Is(loadErr, pgx.ErrNoRows) {
			return AdmitResponse{}, loadErr
		}
		if err := tx.Rollback(ctx); err != nil {
			return AdmitResponse{}, err
		}
		cancel()
	}
	if reason := validatePolicyAndTarget(req, budget, routing); reason != "" {
		return decisionResponse(req.RequestID, DecisionReject, reason, budget, routing), nil
	}
	if routing.Spec.Canary.Enabled {
		return decisionResponse(req.RequestID, DecisionRequireApproval, ReasonApprovalRequired, budget, routing), nil
	}
	budgetMicros, err := quantityToMicros(budget.Spec.BudgetEUR)
	if err != nil {
		return AdmitResponse{}, fmt.Errorf("budget conversion: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return AdmitResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := loadReservationTx(ctx, tx, req.RequestID, true)
	if err == nil {
		if err := matchPrincipal(existing, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
			return AdmitResponse{}, err
		}
		if existing.AdmissionFingerprint != fingerprint {
			return AdmitResponse{}, errors.New("duplicate request_id has conflicting immutable admission payload")
		}
		if err := tx.Commit(ctx); err != nil {
			return AdmitResponse{}, err
		}
		if !reservationIsActive(existing.State) {
			return AdmitResponse{Decision: DecisionReject, ReasonCode: ReasonInvalidTransition, TraceID: req.RequestID}, nil
		}
		return responseForReservation(existing, ReasonDuplicateRequest), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AdmitResponse{}, err
	}

	windowID, err := budgetWindowID(budget, e.now())
	if err != nil {
		return AdmitResponse{}, err
	}
	policyIdentity := budgetPolicyIdentity(budget)
	if _, err := tx.Exec(ctx, `
INSERT INTO govar_tenants (tenant_id, budget_micros, budget_identity,current_window_id,window_period)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tenant_id) DO NOTHING`, req.AuthenticatedTenantID, budgetMicros, policyIdentity, windowID, strings.ToLower(budget.Spec.Period)); err != nil {
		return AdmitResponse{}, err
	}
	ledger, err := loadTenantTx(ctx, tx, req.AuthenticatedTenantID)
	if err != nil {
		return AdmitResponse{}, err
	}
	if ledger.CurrentWindowID == windowID && (ledger.BudgetIdentity != policyIdentity || ledger.BudgetMicros != budgetMicros) {
		if err := tx.Commit(ctx); err != nil {
			return AdmitResponse{}, err
		}
		return decisionResponse(req.RequestID, DecisionReject, ReasonBudgetWindowConflict, budget, routing), nil
	}
	if ledger.CurrentWindowID != windowID {
		if !windowStartsAfter(windowID, ledger.CurrentWindowID) {
			_ = tx.Commit(ctx)
			return decisionResponse(req.RequestID, DecisionReject, ReasonBudgetWindowConflict, budget, routing), nil
		}
		if err := rolloverTenantTx(ctx, tx, req.AuthenticatedTenantID, ledger, windowID, budgetMicros); err != nil {
			return AdmitResponse{}, err
		}
		ledger.BudgetIdentity = policyIdentity
		ledger.WindowPeriod = strings.ToLower(budget.Spec.Period)
		if _, err := tx.Exec(ctx, `UPDATE govar_tenants SET budget_identity=$2,window_period=$3 WHERE tenant_id=$1`, req.AuthenticatedTenantID, policyIdentity, ledger.WindowPeriod); err != nil {
			return AdmitResponse{}, err
		}
	}
	cohortDigest := ""
	allocatedRiskOverride := int64(-1)
	if strings.TrimSpace(routing.Annotations[AnnotationReservationMethod]) == "govar_fixed_cohort" {
		cohort, loadErr := loadFrozenCohortTx(ctx, tx, req.AuthenticatedTenantID, req.CohortID)
		if loadErr != nil {
			if errors.Is(loadErr, pgx.ErrNoRows) {
				_ = tx.Commit(ctx)
				return decisionResponse(req.RequestID, DecisionAbstain, ReasonInsufficientCalibration, budget, routing), nil
			}
			return AdmitResponse{}, loadErr
		}
		if cohort.LedgerLayoutID != LedgerLayoutID || cohort.RouteSnapshotSchema != RouteSnapshotSchemaID || cohort.SoftwareHash != e.cohortSoftwareHash {
			_ = tx.Commit(ctx)
			return decisionResponse(req.RequestID, DecisionAbstain, ReasonInsufficientCalibration, budget, routing), nil
		}
		allocatedRiskOverride, loadErr = validateCohortAdmission(req, routing, cohort, e.now())
		if loadErr != nil {
			_ = tx.Commit(ctx)
			return decisionResponse(req.RequestID, DecisionAbstain, ReasonInsufficientCalibration, budget, routing), nil
		}
		cohortDigest = cohort.RegistryDigest
	}
	choice, infeasibleReason, err := chooseAdmission(req, routing, candidates, ledger.available())
	if err != nil {
		return AdmitResponse{}, err
	}
	if infeasibleReason != "" {
		if err := tx.Commit(ctx); err != nil {
			return AdmitResponse{}, err
		}
		decision := DecisionAbstain
		if infeasibleReason == ReasonBudgetUnavailable {
			decision = DecisionQueue
		}
		return decisionResponse(req.RequestID, decision, infeasibleReason, budget, routing), nil
	}
	best, reservedCost := choice.Candidate, choice.Reservation
	if allocatedRiskOverride >= 0 && choice.Method == "govar_fixed_cohort" {
		choice.AllocatedRiskPPB = allocatedRiskOverride
	} else if choice.Method != "govar_fixed_cohort" {
		choice.AllocatedRiskPPB = 0
	}
	expiry := e.now().UTC().Add(5 * time.Minute)
	res := Reservation{RequestID: req.RequestID, TenantID: req.AuthenticatedTenantID, WorkloadUID: req.AuthenticatedWorkloadUID,
		SelectedDeployment: best.ModelRef, ProviderAttemptID: req.RequestID + ":attempt:1", OutboxID: req.RequestID + ":dispatch:1",
		OutboxState: OutboxPending, State: StateReserved, ReservedCostMicros: reservedCost, ResidualHoldMicros: reservedCost,
		PolicyVersion: policyVersion(budget, routing), PricingVersion: best.PricingVersion, ReservationMode: choice.Method,
		RiskLevel: riskLevel(snapshot), AllocatedRiskPPB: choice.AllocatedRiskPPB, Expiry: expiry,
		InputPriceMicrosPerMillion: best.InputPriceMicrosPerMillion, OutputPriceMicrosPerMillion: best.OutputPriceMicrosPerMillion,
		AdmissionFingerprint: fingerprint, CandidateSnapshotVersion: best.SnapshotVersion, RouteSnapshot: best.RouteSnapshot, CohortID: req.CohortID, CohortIndex: req.CohortIndex,
		CohortRegistryDigest: cohortDigest, OriginWindowID: ledger.CurrentWindowID, EnforcementWindowID: ledger.CurrentWindowID, ProviderRetryPolicy: "NO_PROVIDER_RETRY"}

	if _, err := tx.Exec(ctx, `
INSERT INTO govar_reservations (
 request_id, tenant_id, workload_uid, selected_deployment, provider_attempt_id,
 outbox_id, outbox_state, state, reserved_cost_micros, provisional_cost_micros,
 residual_hold_micros, usage_version, finalized, policy_version, pricing_version,
 reservation_mode, risk_level, allocated_risk_ppb, expiry,
 input_price_micros_per_million, output_price_micros_per_million, admission_fingerprint, candidate_snapshot_version,cohort_id,cohort_index,
 cohort_registry_digest,origin_window_id,enforcement_window_id,
 route_namespace,selected_model_uid,selected_model_generation,selected_model_resource_version,selected_provider_name,selected_provider_uid,
 selected_provider_generation,selected_provider_resource_version,pricing_compliance_hash,route_binding_name,route_provider_deployment,route_cluster,route_authority,route_path_mode,route_snapshot_hash
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,0,$9,0,FALSE,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,
 $25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39)`,
		res.RequestID, res.TenantID, res.WorkloadUID, res.SelectedDeployment, res.ProviderAttemptID,
		res.OutboxID, res.OutboxState, res.State, res.ReservedCostMicros, res.PolicyVersion,
		res.PricingVersion, res.ReservationMode, res.RiskLevel, res.AllocatedRiskPPB, res.Expiry,
		res.InputPriceMicrosPerMillion, res.OutputPriceMicrosPerMillion, res.AdmissionFingerprint, res.CandidateSnapshotVersion, res.CohortID, res.CohortIndex,
		res.CohortRegistryDigest, res.OriginWindowID, res.EnforcementWindowID,
		res.RouteSnapshot.Namespace, res.RouteSnapshot.ModelUID, res.RouteSnapshot.ModelGeneration, res.RouteSnapshot.ModelResourceVersion,
		res.RouteSnapshot.ProviderName, res.RouteSnapshot.ProviderUID, res.RouteSnapshot.ProviderGeneration, res.RouteSnapshot.ProviderResourceVersion,
		res.RouteSnapshot.PricingComplianceHash, res.RouteSnapshot.RouteBindingName, res.RouteSnapshot.ProviderDeployment, res.RouteSnapshot.Cluster,
		res.RouteSnapshot.Authority, res.RouteSnapshot.PathMode, res.RouteSnapshot.SnapshotHash); err != nil {
		return AdmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO govar_outbox(outbox_id, request_id, tenant_id, workload_uid, provider_attempt_id, state)
VALUES ($1,$2,$3,$4,$5,$6)`, res.OutboxID, res.RequestID, res.TenantID, res.WorkloadUID, res.ProviderAttemptID, res.OutboxState); err != nil {
		return AdmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE govar_tenants SET reserved_micros=reserved_micros+$2,
 active_reservations=active_reservations+1, updated_at=NOW() WHERE tenant_id=$1`, res.TenantID, reservedCost); err != nil {
		return AdmitResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdmitResponse{}, err
	}
	return responseForReservation(res, ReasonHighestUtility), nil
}

func (e *PostgresEngine) Dispatch(req DispatchRequest) (Reservation, ReasonCode, error) {
	if err := validateEventPrincipal(req.RequestID, req.EventID, req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	payloadHash := eventPayloadHash("dispatch", req.RequestID, req.TenantID, req.WorkloadUID, req.ProviderAttemptID, req.RouteSnapshotHash, string(req.Status))
	return e.mutateEvent(req.EventID, req.RequestID, "dispatch", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, _ *tenantLedger) (ReasonCode, error) {
		if req.ProviderAttemptID != res.ProviderAttemptID {
			return ReasonInvalidTransition, errors.New("provider_attempt_id does not match the reserved attempt")
		}
		if req.RouteSnapshotHash != res.RouteSnapshot.SnapshotHash {
			return ReasonInvalidTransition, errors.New("dispatch route snapshot does not match reservation")
		}
		previousOutbox := res.OutboxState
		code, err := applyDispatch(res, req.Status)
		if err != nil {
			return code, err
		}
		return code, updateOutboxState(ctx, tx, res.OutboxID, previousOutbox, res.OutboxState)
	}, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID)
}

func (e *PostgresEngine) Settle(req SettleRequest) (Reservation, ReasonCode, error) {
	if err := validateSettleRequest(req); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	payloadHash := eventPayloadHash("settlement", req.RequestID, req.TenantID, req.WorkloadUID, req.ProviderAttemptID, fmt.Sprint(req.ActualCostMicros), fmt.Sprint(req.ActualInput), fmt.Sprint(req.ActualOutput), fmt.Sprint(req.UsageVersion), req.PredecessorEventID, fmt.Sprint(req.Final), req.ErrorStatus)
	return e.mutateEvent(req.SettlementID, req.RequestID, "settlement", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, tenant *tenantLedger) (ReasonCode, error) {
		if req.ProviderAttemptID != res.ProviderAttemptID {
			return ReasonInvalidTransition, errors.New("provider_attempt_id does not match the reserved attempt")
		}
		previousOutbox := res.OutboxState
		code, err := applySettlement(res, tenant, req)
		if err != nil {
			return code, err
		}
		return code, updateOutboxState(ctx, tx, res.OutboxID, previousOutbox, res.OutboxState)
	}, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID)
}

func (e *PostgresEngine) Cancel(req CancelRequest) (Reservation, ReasonCode, error) {
	if err := validateEventPrincipal(req.RequestID, req.EventID, req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	payloadHash := eventPayloadHash("cancel", req.RequestID, req.TenantID, req.WorkloadUID, req.ProviderAttemptID, req.Reason, fmt.Sprint(req.AuthoritativeUnbilled))
	return e.mutateEvent(req.EventID, req.RequestID, "cancel", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, tenant *tenantLedger) (ReasonCode, error) {
		if req.ProviderAttemptID != res.ProviderAttemptID {
			return ReasonInvalidTransition, errors.New("provider_attempt_id does not match the reserved attempt")
		}
		previousOutbox := res.OutboxState
		code, err := applyCancel(res, tenant, req.AuthoritativeUnbilled)
		if err != nil {
			return code, err
		}
		return code, updateOutboxState(ctx, tx, res.OutboxID, previousOutbox, res.OutboxState)
	}, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID)
}

type eventMutation func(context.Context, pgx.Tx, *Reservation, *tenantLedger) (ReasonCode, error)

func (e *PostgresEngine) mutateEvent(eventID, requestID, kind, payloadHash string, mutation eventMutation, tenantID, workloadUID string) (Reservation, ReasonCode, error) {
	var reservation Reservation
	var code ReasonCode
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		reservation, code, err = e.mutateEventOnce(eventID, requestID, kind, payloadHash, mutation, tenantID, workloadUID)
		if !isRetryablePG(err) {
			return reservation, code, err
		}
		time.Sleep(time.Duration(1<<attempt) * 10 * time.Millisecond)
	}
	return reservation, code, err
}

func (e *PostgresEngine) mutateEventOnce(eventID, requestID, kind, payloadHash string, mutation eventMutation, tenantID, workloadUID string) (Reservation, ReasonCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Reservation{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var priorRequest, priorHash string
	err = tx.QueryRow(ctx, `SELECT request_id,payload_hash FROM govar_inbox WHERE event_id=$1`, eventID).Scan(&priorRequest, &priorHash)
	if err == nil {
		if priorRequest != requestID {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event id is already bound to another request")
		}
		if priorHash != payloadHash {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event id replay has conflicting immutable payload")
		}
		res, loadErr := loadReservationTx(ctx, tx, requestID, true)
		if loadErr != nil {
			return Reservation{}, ReasonDuplicateEvent, loadErr
		}
		if err := matchPrincipal(res, tenantID, workloadUID); err != nil {
			return Reservation{}, ReasonPrincipalMismatch, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Reservation{}, "", err
		}
		if kind == "settlement" {
			return res, ReasonSettlementDuplicate, nil
		}
		return res, ReasonDuplicateEvent, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, "", err
	}
	res, err := loadReservationTx(ctx, tx, requestID, true)
	if err != nil {
		return Reservation{}, ReasonReservationNotFound, errors.New("reservation not found")
	}
	if err := matchPrincipal(res, tenantID, workloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	tenant, err := loadTenantTx(ctx, tx, res.TenantID)
	if err != nil {
		return Reservation{}, "", err
	}
	if err := advanceTenantWindowTx(ctx, tx, res.TenantID, tenant, e.now()); err != nil {
		return Reservation{}, ReasonBudgetWindowConflict, err
	}
	wasActive := reservationIsActive(res.State)
	code, err := mutation(ctx, tx, &res, tenant)
	if err != nil {
		return res, code, err
	}
	res.LastReasonCode, res.LastTransitionEventID = code, eventID
	if _, err := tx.Exec(ctx, `INSERT INTO govar_inbox(event_id,request_id,event_kind,payload_hash) VALUES ($1,$2,$3,$4)`, eventID, requestID, kind, payloadHash); err != nil {
		return Reservation{}, "", err
	}
	if err := storeReservationTx(ctx, tx, res); err != nil {
		return Reservation{}, "", err
	}
	isActive := reservationIsActive(res.State)
	if wasActive && !isActive {
		tenant.ActiveReservations--
	} else if !wasActive && isActive {
		tenant.ActiveReservations++
	}
	if _, err := tx.Exec(ctx, `UPDATE govar_tenants SET settled_micros=$2,reserved_micros=$3,
	 carried_adjustment_micros=$4,active_reservations=$5,historical_credit_micros=$6,updated_at=NOW() WHERE tenant_id=$1`,
		res.TenantID, tenant.SettledMicros, tenant.ReservedMicros, tenant.CarriedAdjustmentMicros, tenant.ActiveReservations, tenant.HistoricalCreditMicros); err != nil {
		return Reservation{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, "", err
	}
	return res, code, nil
}

func (e *PostgresEngine) Liability(tenantID string) LiabilityResponse {
	response, _ := e.LiabilityWithError(tenantID)
	return response
}

func (e *PostgresEngine) LiabilityWithError(tenantID string) (LiabilityResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp := LiabilityResponse{TenantID: tenantID}
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return resp, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	t, err := loadTenantTx(ctx, tx, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return resp, nil
	}
	if err != nil {
		return resp, err
	}
	if err = advanceTenantWindowTx(ctx, tx, tenantID, t, e.now()); err != nil {
		return resp, err
	}
	if err = tx.Commit(ctx); err != nil {
		return resp, err
	}
	resp.SettledSpendMicros = t.SettledMicros
	resp.OutstandingLiabilityMicros = t.ReservedMicros
	resp.CarriedAdjustmentMicros = t.CarriedAdjustmentMicros
	resp.ActiveReservations = t.ActiveReservations
	resp.CurrentWindowID = t.CurrentWindowID
	resp.HistoricalAuditCreditMicros = t.HistoricalCreditMicros
	resp.AvailableBudgetMicros = t.available()
	return resp, nil
}

func loadTenantTx(ctx context.Context, tx pgx.Tx, tenantID string) (*tenantLedger, error) {
	var t tenantLedger
	err := tx.QueryRow(ctx, `SELECT budget_micros,settled_micros,reserved_micros,
	 carried_adjustment_micros,active_reservations,budget_identity,current_window_id,historical_credit_micros,window_period FROM govar_tenants WHERE tenant_id=$1 FOR UPDATE`, tenantID).Scan(
		&t.BudgetMicros, &t.SettledMicros, &t.ReservedMicros, &t.CarriedAdjustmentMicros, &t.ActiveReservations, &t.BudgetIdentity, &t.CurrentWindowID, &t.HistoricalCreditMicros, &t.WindowPeriod)
	return &t, err
}

func rolloverTenantTx(ctx context.Context, tx pgx.Tx, tenantID string, t *tenantLedger, nextWindow string, nextBudget MoneyMicros) error {
	rows, err := tx.Query(ctx, `SELECT request_id FROM govar_reservations WHERE tenant_id=$1 AND state IN ('SETTLED_PROVISIONAL','CORRECTED_PROVISIONAL') AND carried=FALSE AND enforcement_window_id=$2 FOR UPDATE`, tenantID, t.CurrentWindowID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		r, err := loadReservationTx(ctx, tx, id, true)
		if err != nil {
			return err
		}
		r.RolloverGuardMicros += r.ProvisionalCostMicros
		t.ReservedMicros += r.ProvisionalCostMicros
		if err := storeReservationTx(ctx, tx, r); err != nil {
			return err
		}
	}
	t.SettledMicros = 0
	t.CurrentWindowID = nextWindow
	t.BudgetMicros = nextBudget
	_, err = tx.Exec(ctx, `UPDATE govar_tenants SET budget_micros=$2,settled_micros=0,reserved_micros=$3,current_window_id=$4,updated_at=NOW() WHERE tenant_id=$1`, tenantID, nextBudget, t.ReservedMicros, nextWindow)
	return err
}

func advanceTenantWindowTx(ctx context.Context, tx pgx.Tx, tenantID string, t *tenantLedger, at time.Time) error {
	if t.CurrentWindowID == "" || t.WindowPeriod == "" {
		return nil
	}
	next, err := budgetWindowForPeriod(t.WindowPeriod, at)
	if err != nil {
		return err
	}
	if next == t.CurrentWindowID {
		return nil
	}
	if !windowStartsAfter(next, t.CurrentWindowID) {
		return errors.New("trusted clock moved before active budget window")
	}
	return rolloverTenantTx(ctx, tx, tenantID, t, next, t.BudgetMicros)
}

func loadReservationTx(ctx context.Context, tx pgx.Tx, requestID string, lock bool) (Reservation, error) {
	query := `SELECT request_id,tenant_id,workload_uid,selected_deployment,provider_attempt_id,
 outbox_id,outbox_state,state,reserved_cost_micros,provisional_cost_micros,base_actual_micros,settled_effect_micros,residual_hold_micros,
 usage_version,finalized,policy_version,pricing_version,reservation_mode,risk_level,
 allocated_risk_ppb,expiry,input_price_micros_per_million,output_price_micros_per_million,admission_fingerprint,candidate_snapshot_version,cohort_id,cohort_index,
 cohort_registry_digest,origin_window_id,enforcement_window_id,rollover_guard_micros,carry_effect_micros,historical_credit_micros,carried,last_usage_event_id,provider_retry_policy,last_reason_code,last_transition_event_id
 ,route_namespace,selected_model_uid,selected_model_generation,selected_model_resource_version,selected_provider_name,selected_provider_uid,
 selected_provider_generation,selected_provider_resource_version,pricing_compliance_hash,route_binding_name,route_provider_deployment,route_cluster,route_authority,route_path_mode,route_snapshot_hash
 FROM govar_reservations WHERE request_id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var r Reservation
	err := tx.QueryRow(ctx, query, requestID).Scan(
		&r.RequestID, &r.TenantID, &r.WorkloadUID, &r.SelectedDeployment, &r.ProviderAttemptID,
		&r.OutboxID, &r.OutboxState, &r.State, &r.ReservedCostMicros, &r.ProvisionalCostMicros,
		&r.BaseActualMicros, &r.SettledEffectMicros, &r.ResidualHoldMicros, &r.UsageVersion, &r.Finalized, &r.PolicyVersion, &r.PricingVersion,
		&r.ReservationMode, &r.RiskLevel, &r.AllocatedRiskPPB, &r.Expiry,
		&r.InputPriceMicrosPerMillion, &r.OutputPriceMicrosPerMillion, &r.AdmissionFingerprint, &r.CandidateSnapshotVersion, &r.CohortID, &r.CohortIndex,
		&r.CohortRegistryDigest, &r.OriginWindowID, &r.EnforcementWindowID, &r.RolloverGuardMicros, &r.CarryEffectMicros, &r.HistoricalCreditMicros, &r.Carried, &r.LastUsageEventID, &r.ProviderRetryPolicy, &r.LastReasonCode, &r.LastTransitionEventID,
		&r.RouteSnapshot.Namespace, &r.RouteSnapshot.ModelUID, &r.RouteSnapshot.ModelGeneration, &r.RouteSnapshot.ModelResourceVersion,
		&r.RouteSnapshot.ProviderName, &r.RouteSnapshot.ProviderUID, &r.RouteSnapshot.ProviderGeneration, &r.RouteSnapshot.ProviderResourceVersion,
		&r.RouteSnapshot.PricingComplianceHash, &r.RouteSnapshot.RouteBindingName, &r.RouteSnapshot.ProviderDeployment, &r.RouteSnapshot.Cluster,
		&r.RouteSnapshot.Authority, &r.RouteSnapshot.PathMode, &r.RouteSnapshot.SnapshotHash)
	r.RouteSnapshot.ModelName = r.SelectedDeployment
	r.RouteSnapshot.PricingVersion = r.PricingVersion
	if err == nil {
		err = validateReservationRoute(r)
	}
	return r, err
}

func storeReservationTx(ctx context.Context, tx pgx.Tx, r Reservation) error {
	_, err := tx.Exec(ctx, `UPDATE govar_reservations SET outbox_state=$2,state=$3,
 provisional_cost_micros=$4,residual_hold_micros=$5,usage_version=$6,finalized=$7,
	 rollover_guard_micros=$8,carry_effect_micros=$9,historical_credit_micros=$10,carried=$11,enforcement_window_id=$12,
	 base_actual_micros=$13,last_usage_event_id=$14,last_reason_code=$15,last_transition_event_id=$16,settled_effect_micros=$17,updated_at=NOW() WHERE request_id=$1`, r.RequestID, r.OutboxState, r.State,
		r.ProvisionalCostMicros, r.ResidualHoldMicros, r.UsageVersion, r.Finalized, r.RolloverGuardMicros, r.CarryEffectMicros, r.HistoricalCreditMicros, r.Carried, r.EnforcementWindowID, r.BaseActualMicros, r.LastUsageEventID, r.LastReasonCode, r.LastTransitionEventID, r.SettledEffectMicros)
	return err
}

func reservationIsActive(state ReservationState) bool {
	return state != StateCanceledUnbilled && state != StateFailedUnbilled && state != StateExpiredUndispatched && state != StateFinalized && state != StateLateFinalized
}

func (e *PostgresEngine) ReserveRetry(context.Context, string) (ReasonCode, error) {
	return ReasonRetriesDisabled, errors.New("provider retry/fallback/hedge is disabled unless a separately reserved attempt API is implemented")
}

func updateOutboxState(ctx context.Context, tx pgx.Tx, outboxID string, previous, next OutboxState) error {
	result, err := tx.Exec(ctx, `UPDATE govar_outbox SET state=$3,version=version+1,updated_at=NOW() WHERE outbox_id=$1 AND state=$2`, outboxID, previous, next)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("outbox state compare-and-swap affected no row")
	}
	return nil
}

func isRetryablePG(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01")
}
func isUniqueViolationPG(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
