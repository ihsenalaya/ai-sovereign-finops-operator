package govar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
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

func (e *PostgresEngine) initSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS govar_tenants (
  tenant_id TEXT PRIMARY KEY,
  budget_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  settled_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  reserved_eur DOUBLE PRECISION NOT NULL DEFAULT 0,
  active_reservations INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS govar_reservations (
  request_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  selected_deployment TEXT NOT NULL,
  reserved_cost DOUBLE PRECISION NOT NULL,
  actual_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
  policy_version TEXT NOT NULL,
  pricing_version TEXT NOT NULL,
  reservation_mode TEXT NOT NULL,
  risk_level TEXT NOT NULL,
  expiry TIMESTAMPTZ NOT NULL,
  settled BOOLEAN NOT NULL DEFAULT FALSE,
  canceled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS govar_settlements (
  settlement_id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := e.pool.Exec(ctx, schema)
	return err
}

func (e *PostgresEngine) Admit(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
	if req.RequestID == "" {
		return AdmitResponse{}, errors.New("request_id is required")
	}

	snapshot := BuildPolicySnapshot(budget, routing)
	sort.SliceStable(candidates, func(i, j int) bool {
		left := expectedCost(candidates[i], req.InputTokens, req.MaxOutputTokens)
		right := expectedCost(candidates[j], req.InputTokens, req.MaxOutputTokens)
		if left == right {
			return candidates[i].ModelRef < candidates[j].ModelRef
		}
		return left < right
	})

	if req.RequireApproval {
		return AdmitResponse{
			Decision:       DecisionRequireApproval,
			ReasonCode:     ReasonApprovalRequired,
			PolicyVersion:  policyVersion(budget, routing),
			PricingVersion: "provider-pricing-live",
			TraceID:        req.RequestID,
		}, nil
	}
	if len(candidates) == 0 {
		return AdmitResponse{
			Decision:       DecisionAbstain,
			ReasonCode:     ReasonNoCandidate,
			PolicyVersion:  policyVersion(budget, routing),
			PricingVersion: "provider-pricing-live",
			TraceID:        req.RequestID,
		}, nil
	}

	best := candidates[0]
	reservedCost := expectedCost(best, req.InputTokens, req.MaxOutputTokens)
	expiry := time.Now().UTC().Add(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdmitResponse{}, err
	}
	defer tx.Rollback(ctx)

	var existing Reservation
	err = tx.QueryRow(ctx, `
SELECT request_id, tenant_id, selected_deployment, reserved_cost, actual_cost,
       policy_version, pricing_version, reservation_mode, risk_level, expiry,
       settled, canceled
FROM govar_reservations WHERE request_id=$1`, req.RequestID).Scan(
		&existing.RequestID,
		&existing.TenantID,
		&existing.SelectedDeployment,
		&existing.ReservedCost,
		&existing.ActualCost,
		&existing.PolicyVersion,
		&existing.PricingVersion,
		&existing.ReservationMode,
		&existing.RiskLevel,
		&existing.Expiry,
		&existing.Settled,
		&existing.Canceled,
	)
	if err == nil {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return AdmitResponse{}, commitErr
		}
		return AdmitResponse{
			Decision:           DecisionAdmit,
			ReasonCode:         ReasonDuplicateRequest,
			SelectedDeployment: existing.SelectedDeployment,
			ReservationID:      existing.RequestID,
			ReservedCost:       existing.ReservedCost,
			ReservationMode:    existing.ReservationMode,
			RiskLevel:          existing.RiskLevel,
			PolicyVersion:      existing.PolicyVersion,
			PricingVersion:     existing.PricingVersion,
			Expiry:             existing.Expiry.UTC().Format(time.RFC3339),
			TraceID:            existing.RequestID,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AdmitResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO govar_tenants (tenant_id, budget_eur)
VALUES ($1, $2)
ON CONFLICT (tenant_id) DO UPDATE
SET budget_eur = EXCLUDED.budget_eur, updated_at = NOW()`,
		req.TenantID, snapshot.BudgetEUR); err != nil {
		return AdmitResponse{}, err
	}

	var budgetEUR, settledEUR, reservedEUR float64
	if err := tx.QueryRow(ctx, `
SELECT budget_eur, settled_eur, reserved_eur
FROM govar_tenants
WHERE tenant_id=$1
FOR UPDATE`, req.TenantID).Scan(&budgetEUR, &settledEUR, &reservedEUR); err != nil {
		return AdmitResponse{}, err
	}

	if budgetEUR-settledEUR-reservedEUR < reservedCost {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return AdmitResponse{}, commitErr
		}
		return AdmitResponse{
			Decision:       DecisionQueue,
			ReasonCode:     ReasonBudgetUnavailable,
			PolicyVersion:  policyVersion(budget, routing),
			PricingVersion: "provider-pricing-live",
			TraceID:        req.RequestID,
		}, nil
	}

	res := Reservation{
		RequestID:          req.RequestID,
		TenantID:           req.TenantID,
		SelectedDeployment: best.ModelRef,
		ReservedCost:       reservedCost,
		PolicyVersion:      policyVersion(budget, routing),
		PricingVersion:     "provider-pricing-live",
		ReservationMode:    "fixed_reserve_settle",
		RiskLevel:          riskLevel(snapshot),
		Expiry:             expiry,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO govar_reservations (
  request_id, tenant_id, selected_deployment, reserved_cost, actual_cost,
  policy_version, pricing_version, reservation_mode, risk_level, expiry, settled, canceled
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,FALSE,FALSE)`,
		res.RequestID, res.TenantID, res.SelectedDeployment, res.ReservedCost, 0.0,
		res.PolicyVersion, res.PricingVersion, res.ReservationMode, res.RiskLevel, res.Expiry); err != nil {
		return AdmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE govar_tenants
SET reserved_eur = reserved_eur + $2,
    active_reservations = active_reservations + 1,
    updated_at = NOW()
WHERE tenant_id = $1`, req.TenantID, reservedCost); err != nil {
		return AdmitResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdmitResponse{}, err
	}
	return AdmitResponse{
		Decision:           DecisionAdmit,
		ReasonCode:         ReasonHighestUtility,
		SelectedDeployment: best.ModelRef,
		ReservationID:      req.RequestID,
		ReservedCost:       reservedCost,
		ReservationMode:    res.ReservationMode,
		RiskLevel:          res.RiskLevel,
		PolicyVersion:      res.PolicyVersion,
		PricingVersion:     res.PricingVersion,
		Expiry:             expiry.Format(time.RFC3339),
		TraceID:            req.RequestID,
	}, nil
}

func (e *PostgresEngine) Settle(req SettleRequest) (Reservation, ReasonCode, error) {
	if req.RequestID == "" || req.SettlementID == "" {
		return Reservation{}, "", errors.New("request_id and settlement_id are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Reservation{}, "", err
	}
	defer tx.Rollback(ctx)

	var exists string
	if err := tx.QueryRow(ctx, `SELECT settlement_id FROM govar_settlements WHERE settlement_id=$1`, req.SettlementID).Scan(&exists); err == nil {
		res, loadErr := loadReservationTx(ctx, tx, req.RequestID)
		if loadErr != nil {
			return Reservation{}, ReasonSettlementDuplicate, fmt.Errorf("duplicate settlement for unknown request %s", req.RequestID)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return Reservation{}, "", commitErr
		}
		return res, ReasonSettlementDuplicate, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, "", err
	}

	res, err := loadReservationTx(ctx, tx, req.RequestID)
	if err != nil {
		return Reservation{}, ReasonReservationNotFound, fmt.Errorf("reservation not found")
	}
	if res.Canceled {
		if err := tx.Commit(ctx); err != nil {
			return Reservation{}, "", err
		}
		return res, ReasonCanceled, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO govar_settlements (settlement_id, request_id) VALUES ($1, $2)`, req.SettlementID, req.RequestID); err != nil {
		return Reservation{}, "", err
	}
	if _, err := tx.Exec(ctx, `
UPDATE govar_reservations
SET settled = TRUE, actual_cost = $2
WHERE request_id = $1`, req.RequestID, req.ActualCost); err != nil {
		return Reservation{}, "", err
	}
	if _, err := tx.Exec(ctx, `
UPDATE govar_tenants
SET reserved_eur = GREATEST(reserved_eur - $2, 0),
    settled_eur = settled_eur + $3,
    active_reservations = GREATEST(active_reservations - 1, 0),
    updated_at = NOW()
WHERE tenant_id = $1`, res.TenantID, res.ReservedCost, req.ActualCost); err != nil {
		return Reservation{}, "", err
	}
	res.Settled = true
	res.ActualCost = req.ActualCost
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, "", err
	}
	return res, ReasonHighestUtility, nil
}

func (e *PostgresEngine) Cancel(req CancelRequest) (Reservation, error) {
	if req.RequestID == "" {
		return Reservation{}, errors.New("request_id is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)

	res, err := loadReservationTx(ctx, tx, req.RequestID)
	if err != nil {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	if res.Settled || res.Canceled {
		if err := tx.Commit(ctx); err != nil {
			return Reservation{}, err
		}
		return res, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE govar_reservations SET canceled = TRUE WHERE request_id = $1`, req.RequestID); err != nil {
		return Reservation{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE govar_tenants
SET reserved_eur = GREATEST(reserved_eur - $2, 0),
    active_reservations = GREATEST(active_reservations - 1, 0),
    updated_at = NOW()
WHERE tenant_id = $1`, res.TenantID, res.ReservedCost); err != nil {
		return Reservation{}, err
	}
	res.Canceled = true
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return res, nil
}

func (e *PostgresEngine) Liability(tenantID string) LiabilityResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var resp LiabilityResponse
	resp.TenantID = tenantID
	_ = e.pool.QueryRow(ctx, `
SELECT settled_eur, reserved_eur, budget_eur - settled_eur - reserved_eur, active_reservations
FROM govar_tenants
WHERE tenant_id = $1`, tenantID).Scan(
		&resp.SettledSpend,
		&resp.ReservedLiability,
		&resp.AvailableBudget,
		&resp.ActiveReservations,
	)
	return resp
}

func loadReservationTx(ctx context.Context, tx pgx.Tx, requestID string) (Reservation, error) {
	var res Reservation
	err := tx.QueryRow(ctx, `
SELECT request_id, tenant_id, selected_deployment, reserved_cost, actual_cost,
       policy_version, pricing_version, reservation_mode, risk_level, expiry,
       settled, canceled
FROM govar_reservations
WHERE request_id=$1
FOR UPDATE`, requestID).Scan(
		&res.RequestID,
		&res.TenantID,
		&res.SelectedDeployment,
		&res.ReservedCost,
		&res.ActualCost,
		&res.PolicyVersion,
		&res.PricingVersion,
		&res.ReservationMode,
		&res.RiskLevel,
		&res.Expiry,
		&res.Settled,
		&res.Canceled,
	)
	return res, err
}
