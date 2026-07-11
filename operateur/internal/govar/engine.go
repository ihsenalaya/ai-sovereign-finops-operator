package govar

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

type Decision string

const (
	DecisionAdmit           Decision = "ADMIT"
	DecisionQueue           Decision = "QUEUE"
	DecisionReject          Decision = "REJECT"
	DecisionAbstain         Decision = "ABSTAIN"
	DecisionRequireApproval Decision = "REQUIRE_APPROVAL"
)

type ReasonCode string

const (
	ReasonHighestUtility      ReasonCode = "highest_utility_feasible"
	ReasonNoCandidate         ReasonCode = "no_candidate_after_governance"
	ReasonBudgetUnavailable   ReasonCode = "budget_unavailable"
	ReasonDuplicateRequest    ReasonCode = "duplicate_request"
	ReasonReservationNotFound ReasonCode = "reservation_not_found"
	ReasonSettlementDuplicate ReasonCode = "duplicate_settlement"
	ReasonApprovalRequired    ReasonCode = "approval_required"
	ReasonCanceled            ReasonCode = "reservation_canceled"
)

type AdmitRequest struct {
	RequestID         string   `json:"request_id"`
	Namespace         string   `json:"namespace"`
	TenantID          string   `json:"tenant_id"`
	Team              string   `json:"team,omitempty"`
	Application       string   `json:"application,omitempty"`
	SensitiveData     bool     `json:"sensitive_data,omitempty"`
	AllowedZones      []string `json:"allowed_zones,omitempty"`
	BudgetPolicyName  string   `json:"budget_policy_name"`
	RoutingPolicyName string   `json:"routing_policy_name"`
	InputTokens       int64    `json:"input_tokens,omitempty"`
	MaxOutputTokens   int64    `json:"max_output_tokens,omitempty"`
	RequireApproval   bool     `json:"require_approval,omitempty"`
}

type AdmitResponse struct {
	Decision           Decision   `json:"decision"`
	ReasonCode         ReasonCode `json:"reason_code"`
	SelectedDeployment string     `json:"selected_deployment,omitempty"`
	ReservationID      string     `json:"reservation_id,omitempty"`
	ReservedCost       float64    `json:"reserved_cost,omitempty"`
	ReservationMode    string     `json:"reservation_mode,omitempty"`
	RiskLevel          string     `json:"risk_level,omitempty"`
	PolicyVersion      string     `json:"policy_version,omitempty"`
	PricingVersion     string     `json:"pricing_version,omitempty"`
	Expiry             string     `json:"expiry,omitempty"`
	TraceID            string     `json:"trace_id,omitempty"`
}

type SettleRequest struct {
	RequestID    string  `json:"request_id"`
	SettlementID string  `json:"settlement_id"`
	ActualCost   float64 `json:"actual_cost"`
	ActualInput  int64   `json:"actual_input_tokens,omitempty"`
	ActualOutput int64   `json:"actual_output_tokens,omitempty"`
	ErrorStatus  string  `json:"error_status,omitempty"`
}

type CancelRequest struct {
	RequestID string `json:"request_id"`
	Reason    string `json:"reason,omitempty"`
}

type LiabilityResponse struct {
	TenantID           string  `json:"tenant_id"`
	SettledSpend       float64 `json:"settled_spend"`
	ReservedLiability  float64 `json:"outstanding_reserved_liability"`
	AvailableBudget    float64 `json:"available_budget"`
	ActiveReservations int     `json:"active_reservations"`
}

type Reservation struct {
	RequestID          string
	TenantID           string
	SelectedDeployment string
	ReservedCost       float64
	ActualCost         float64
	PolicyVersion      string
	PricingVersion     string
	ReservationMode    string
	RiskLevel          string
	Expiry             time.Time
	Settled            bool
	Canceled           bool
}

type tenantLedger struct {
	BudgetEUR   float64
	SettledEUR  float64
	ReservedEUR float64
	Requests    map[string]struct{}
}

type Engine struct {
	mu           sync.Mutex
	reservations map[string]Reservation
	settlements  map[string]struct{}
	tenants      map[string]*tenantLedger
}

func NewEngine() *Engine {
	return &Engine{
		reservations: map[string]Reservation{},
		settlements:  map[string]struct{}{},
		tenants:      map[string]*tenantLedger{},
	}
}

func (e *Engine) Admit(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
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

	e.mu.Lock()
	defer e.mu.Unlock()

	tenant := e.ensureTenant(req.TenantID, snapshot.BudgetEUR)
	if _, exists := e.reservations[req.RequestID]; exists {
		existing := e.reservations[req.RequestID]
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
	if tenant.available() < reservedCost {
		return AdmitResponse{
			Decision:       DecisionQueue,
			ReasonCode:     ReasonBudgetUnavailable,
			PolicyVersion:  policyVersion(budget, routing),
			PricingVersion: "provider-pricing-live",
			TraceID:        req.RequestID,
		}, nil
	}

	tenant.ReservedEUR += reservedCost
	tenant.Requests[req.RequestID] = struct{}{}
	expiry := time.Now().UTC().Add(5 * time.Minute)
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
	e.reservations[req.RequestID] = res

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

func (e *Engine) Settle(req SettleRequest) (Reservation, ReasonCode, error) {
	if req.RequestID == "" || req.SettlementID == "" {
		return Reservation{}, "", errors.New("request_id and settlement_id are required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.settlements[req.SettlementID]; exists {
		res, ok := e.reservations[req.RequestID]
		if !ok {
			return Reservation{}, ReasonSettlementDuplicate, fmt.Errorf("duplicate settlement for unknown request %s", req.RequestID)
		}
		return res, ReasonSettlementDuplicate, nil
	}
	res, ok := e.reservations[req.RequestID]
	if !ok {
		return Reservation{}, ReasonReservationNotFound, fmt.Errorf("reservation not found")
	}
	if res.Canceled {
		return res, ReasonCanceled, nil
	}
	tenant := e.ensureTenant(res.TenantID, 0)
	tenant.ReservedEUR -= res.ReservedCost
	if tenant.ReservedEUR < 0 {
		tenant.ReservedEUR = 0
	}
	tenant.SettledEUR += req.ActualCost
	delete(tenant.Requests, req.RequestID)
	res.Settled = true
	res.ActualCost = req.ActualCost
	e.reservations[req.RequestID] = res
	e.settlements[req.SettlementID] = struct{}{}
	return res, ReasonHighestUtility, nil
}

func (e *Engine) Cancel(req CancelRequest) (Reservation, error) {
	if req.RequestID == "" {
		return Reservation{}, errors.New("request_id is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	res, ok := e.reservations[req.RequestID]
	if !ok {
		return Reservation{}, fmt.Errorf("reservation not found")
	}
	if res.Settled || res.Canceled {
		return res, nil
	}
	tenant := e.ensureTenant(res.TenantID, 0)
	tenant.ReservedEUR -= res.ReservedCost
	if tenant.ReservedEUR < 0 {
		tenant.ReservedEUR = 0
	}
	delete(tenant.Requests, req.RequestID)
	res.Canceled = true
	e.reservations[req.RequestID] = res
	return res, nil
}

func (e *Engine) Liability(tenantID string) LiabilityResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	tenant := e.ensureTenant(tenantID, 0)
	return LiabilityResponse{
		TenantID:           tenantID,
		SettledSpend:       tenant.SettledEUR,
		ReservedLiability:  tenant.ReservedEUR,
		AvailableBudget:    tenant.available(),
		ActiveReservations: len(tenant.Requests),
	}
}

func (e *Engine) ensureTenant(tenantID string, budgetEUR float64) *tenantLedger {
	tenant, ok := e.tenants[tenantID]
	if !ok {
		tenant = &tenantLedger{BudgetEUR: budgetEUR, Requests: map[string]struct{}{}}
		e.tenants[tenantID] = tenant
	}
	if budgetEUR > 0 {
		tenant.BudgetEUR = budgetEUR
	}
	return tenant
}

func (t *tenantLedger) available() float64 {
	return t.BudgetEUR - t.SettledEUR - t.ReservedEUR
}

func expectedCost(c Candidate, inputTokens, maxOutputTokens int64) float64 {
	in := (float64(inputTokens) / 1_000_000.0) * c.InputPricePerMillion
	out := (float64(maxOutputTokens) / 1_000_000.0) * c.OutputPricePerMillion
	return in + out
}

func policyVersion(budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) string {
	return fmt.Sprintf("%s:%d|%s:%d", budget.Name, budget.Generation, routing.Name, routing.Generation)
}

func riskLevel(snapshot PolicySnapshot) string {
	if snapshot.RequireSovereigntyCompliance {
		return "strict"
	}
	return "bounded"
}
