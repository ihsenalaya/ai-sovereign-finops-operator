package govar

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

type PostgresEngine struct {
	pool *pgxpool.Pool
}

func NewPostgresEngine(ctx context.Context, databaseURL string) (*PostgresEngine, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	engine := &PostgresEngine{pool: pool}
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
CREATE TABLE IF NOT EXISTS govar_schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $$
DECLARE legacy_exists BOOLEAN;
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_tenants' AND column_name='budget_eur') THEN
    EXECUTE 'SELECT EXISTS(SELECT 1 FROM govar_tenants)' INTO legacy_exists;
    IF legacy_exists THEN RAISE EXCEPTION 'unreconciled legacy floating-point tenant ledger rows exist'; END IF;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_reservations' AND column_name='reserved_cost') THEN
    EXECUTE 'SELECT EXISTS(SELECT 1 FROM govar_reservations)' INTO legacy_exists;
    IF legacy_exists THEN RAISE EXCEPTION 'unreconciled legacy floating-point reservation rows exist'; END IF;
  END IF;
  IF to_regclass('govar_settlements') IS NOT NULL THEN
    EXECUTE 'SELECT EXISTS(SELECT 1 FROM govar_settlements)' INTO legacy_exists;
    IF legacy_exists THEN RAISE EXCEPTION 'unreconciled legacy settlement rows exist'; END IF;
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS budget_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS settled_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS reserved_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS carried_adjustment_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE govar_tenants ADD COLUMN IF NOT EXISTS budget_identity TEXT NOT NULL DEFAULT '';
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
ALTER TABLE govar_inbox ADD COLUMN IF NOT EXISTS payload_hash TEXT NOT NULL DEFAULT '';
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='govar_reservations' AND column_name='reserved_cost') THEN
    ALTER TABLE govar_reservations ALTER COLUMN reserved_cost SET DEFAULT 0;
  END IF;
END $$;
INSERT INTO govar_schema_migrations(version) VALUES (2) ON CONFLICT DO NOTHING;`
	_, err := e.pool.Exec(ctx, schema)
	return err
}

func (e *PostgresEngine) Admit(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
	var response AdmitResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		response, err = e.admitOnce(req, budget, routing, candidates)
		if !isRetryablePG(err) {
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
		if existing.State == StateCanceledUnbilled || existing.State == StateFailedUnbilled || existing.State == StateFinalized {
			return AdmitResponse{Decision: DecisionReject, ReasonCode: ReasonInvalidTransition, TraceID: req.RequestID}, nil
		}
		return responseForReservation(existing, ReasonDuplicateRequest), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AdmitResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO govar_tenants (tenant_id, budget_micros, budget_identity)
VALUES ($1,$2,$3)
ON CONFLICT (tenant_id) DO NOTHING`, req.AuthenticatedTenantID, budgetMicros, budgetIdentity(budget)); err != nil {
		return AdmitResponse{}, err
	}
	ledger, err := loadTenantTx(ctx, tx, req.AuthenticatedTenantID)
	if err != nil {
		return AdmitResponse{}, err
	}
	if ledger.BudgetIdentity != budgetIdentity(budget) {
		if ledger.ReservedMicros != 0 || ledger.SettledMicros != 0 || ledger.CarriedAdjustmentMicros != 0 {
			if err := tx.Commit(ctx); err != nil {
				return AdmitResponse{}, err
			}
			return decisionResponse(req.RequestID, DecisionReject, ReasonBudgetWindowConflict, budget, routing), nil
		}
		ledger.BudgetIdentity = budgetIdentity(budget)
		ledger.BudgetMicros = budgetMicros
		if _, err := tx.Exec(ctx, `UPDATE govar_tenants SET budget_micros=$2,budget_identity=$3,updated_at=NOW() WHERE tenant_id=$1`, req.AuthenticatedTenantID, budgetMicros, ledger.BudgetIdentity); err != nil {
			return AdmitResponse{}, err
		}
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
	expiry := time.Now().UTC().Add(5 * time.Minute)
	res := Reservation{RequestID: req.RequestID, TenantID: req.AuthenticatedTenantID, WorkloadUID: req.AuthenticatedWorkloadUID,
		SelectedDeployment: best.ModelRef, ProviderAttemptID: req.RequestID + ":attempt:1", OutboxID: req.RequestID + ":dispatch:1",
		OutboxState: OutboxPending, State: StateReserved, ReservedCostMicros: reservedCost, ResidualHoldMicros: reservedCost,
		PolicyVersion: policyVersion(budget, routing), PricingVersion: best.PricingVersion, ReservationMode: choice.Method,
		RiskLevel: riskLevel(snapshot), AllocatedRiskPPB: choice.AllocatedRiskPPB, Expiry: expiry,
		InputPriceMicrosPerMillion: best.InputPriceMicrosPerMillion, OutputPriceMicrosPerMillion: best.OutputPriceMicrosPerMillion,
		AdmissionFingerprint: fingerprint, CandidateSnapshotVersion: best.SnapshotVersion, CohortID: req.CohortID, CohortIndex: req.CohortIndex}

	if _, err := tx.Exec(ctx, `
INSERT INTO govar_reservations (
 request_id, tenant_id, workload_uid, selected_deployment, provider_attempt_id,
 outbox_id, outbox_state, state, reserved_cost_micros, provisional_cost_micros,
 residual_hold_micros, usage_version, finalized, policy_version, pricing_version,
 reservation_mode, risk_level, allocated_risk_ppb, expiry,
 input_price_micros_per_million, output_price_micros_per_million, admission_fingerprint, candidate_snapshot_version,cohort_id,cohort_index
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,0,$9,0,FALSE,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		res.RequestID, res.TenantID, res.WorkloadUID, res.SelectedDeployment, res.ProviderAttemptID,
		res.OutboxID, res.OutboxState, res.State, res.ReservedCostMicros, res.PolicyVersion,
		res.PricingVersion, res.ReservationMode, res.RiskLevel, res.AllocatedRiskPPB, res.Expiry,
		res.InputPriceMicrosPerMillion, res.OutputPriceMicrosPerMillion, res.AdmissionFingerprint, res.CandidateSnapshotVersion, res.CohortID, res.CohortIndex); err != nil {
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
	payloadHash := eventPayloadHash("dispatch", req.RequestID, req.TenantID, req.WorkloadUID, req.ProviderAttemptID, string(req.Status))
	return e.mutateEvent(req.EventID, req.RequestID, "dispatch", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, _ *tenantLedger) (ReasonCode, error) {
		if req.ProviderAttemptID != res.ProviderAttemptID {
			return ReasonInvalidTransition, errors.New("provider_attempt_id does not match the reserved attempt")
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
	payloadHash := eventPayloadHash("settlement", req.RequestID, req.TenantID, req.WorkloadUID, fmt.Sprint(req.ActualCostMicros), fmt.Sprint(req.ActualInput), fmt.Sprint(req.ActualOutput), fmt.Sprint(req.UsageVersion), fmt.Sprint(req.Final))
	return e.mutateEvent(req.SettlementID, req.RequestID, "settlement", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, tenant *tenantLedger) (ReasonCode, error) {
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
	payloadHash := eventPayloadHash("cancel", req.RequestID, req.TenantID, req.WorkloadUID, req.Reason, fmt.Sprint(req.AuthoritativeUnbilled))
	return e.mutateEvent(req.EventID, req.RequestID, "cancel", payloadHash, func(ctx context.Context, tx pgx.Tx, res *Reservation, tenant *tenantLedger) (ReasonCode, error) {
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
	wasActive := reservationIsActive(res.State)
	code, err := mutation(ctx, tx, &res, tenant)
	if err != nil {
		return res, code, err
	}
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
 carried_adjustment_micros=$4,active_reservations=$5,updated_at=NOW() WHERE tenant_id=$1`,
		res.TenantID, tenant.SettledMicros, tenant.ReservedMicros, tenant.CarriedAdjustmentMicros, tenant.ActiveReservations); err != nil {
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
	var budget MoneyMicros
	resp := LiabilityResponse{TenantID: tenantID}
	err := e.pool.QueryRow(ctx, `SELECT budget_micros,settled_micros,reserved_micros,
 carried_adjustment_micros,active_reservations FROM govar_tenants WHERE tenant_id=$1`, tenantID).Scan(
		&budget, &resp.SettledSpendMicros, &resp.OutstandingLiabilityMicros,
		&resp.CarriedAdjustmentMicros, &resp.ActiveReservations)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resp, nil
		}
		return LiabilityResponse{}, err
	}
	resp.AvailableBudgetMicros = budget - resp.SettledSpendMicros - resp.OutstandingLiabilityMicros - resp.CarriedAdjustmentMicros
	return resp, nil
}

func loadTenantTx(ctx context.Context, tx pgx.Tx, tenantID string) (*tenantLedger, error) {
	var t tenantLedger
	err := tx.QueryRow(ctx, `SELECT budget_micros,settled_micros,reserved_micros,
	 carried_adjustment_micros,active_reservations,budget_identity FROM govar_tenants WHERE tenant_id=$1 FOR UPDATE`, tenantID).Scan(
		&t.BudgetMicros, &t.SettledMicros, &t.ReservedMicros, &t.CarriedAdjustmentMicros, &t.ActiveReservations, &t.BudgetIdentity)
	return &t, err
}

func loadReservationTx(ctx context.Context, tx pgx.Tx, requestID string, lock bool) (Reservation, error) {
	query := `SELECT request_id,tenant_id,workload_uid,selected_deployment,provider_attempt_id,
 outbox_id,outbox_state,state,reserved_cost_micros,provisional_cost_micros,residual_hold_micros,
 usage_version,finalized,policy_version,pricing_version,reservation_mode,risk_level,
 allocated_risk_ppb,expiry,input_price_micros_per_million,output_price_micros_per_million,admission_fingerprint,candidate_snapshot_version,cohort_id,cohort_index
 FROM govar_reservations WHERE request_id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var r Reservation
	err := tx.QueryRow(ctx, query, requestID).Scan(
		&r.RequestID, &r.TenantID, &r.WorkloadUID, &r.SelectedDeployment, &r.ProviderAttemptID,
		&r.OutboxID, &r.OutboxState, &r.State, &r.ReservedCostMicros, &r.ProvisionalCostMicros,
		&r.ResidualHoldMicros, &r.UsageVersion, &r.Finalized, &r.PolicyVersion, &r.PricingVersion,
		&r.ReservationMode, &r.RiskLevel, &r.AllocatedRiskPPB, &r.Expiry,
		&r.InputPriceMicrosPerMillion, &r.OutputPriceMicrosPerMillion, &r.AdmissionFingerprint, &r.CandidateSnapshotVersion, &r.CohortID, &r.CohortIndex)
	return r, err
}

func storeReservationTx(ctx context.Context, tx pgx.Tx, r Reservation) error {
	_, err := tx.Exec(ctx, `UPDATE govar_reservations SET outbox_state=$2,state=$3,
 provisional_cost_micros=$4,residual_hold_micros=$5,usage_version=$6,finalized=$7,
 updated_at=NOW() WHERE request_id=$1`, r.RequestID, r.OutboxState, r.State,
		r.ProvisionalCostMicros, r.ResidualHoldMicros, r.UsageVersion, r.Finalized)
	return err
}

func reservationIsActive(state ReservationState) bool {
	return state != StateCanceledUnbilled && state != StateFailedUnbilled && state != StateFinalized
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
