package govar

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

// MoneyMicros is one millionth of the configured ledger currency. All ledger
// arithmetic uses this integer type; provider prices are converted before a
// reservation transaction begins.
type MoneyMicros int64

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
	ReasonHighestUtility           ReasonCode = "highest_utility_feasible"
	ReasonNoCandidate              ReasonCode = "no_candidate_after_governance"
	ReasonInsufficientEvidence     ReasonCode = "insufficient_reservation_evidence"
	ReasonBudgetUnavailable        ReasonCode = "budget_unavailable"
	ReasonDuplicateRequest         ReasonCode = "duplicate_request"
	ReasonDuplicateConflict        ReasonCode = "duplicate_request_conflict"
	ReasonDuplicateEvent           ReasonCode = "duplicate_event"
	ReasonReservationNotFound      ReasonCode = "reservation_not_found"
	ReasonSettlementDuplicate      ReasonCode = "duplicate_settlement"
	ReasonApprovalRequired         ReasonCode = "approval_required"
	ReasonCanceled                 ReasonCode = "authoritative_unbilled_cancellation"
	ReasonCancellationUnclear      ReasonCode = "cancellation_delivery_ambiguous"
	ReasonDispatchClaimed          ReasonCode = "dispatch_claimed"
	ReasonDispatchDelivered        ReasonCode = "dispatch_delivered"
	ReasonDispatchUnresolved       ReasonCode = "dispatch_delivery_ambiguous"
	ReasonProvisionalSettlement    ReasonCode = "provisional_settlement"
	ReasonLateSettlement           ReasonCode = "late_settlement"
	ReasonCorrection               ReasonCode = "settlement_correction"
	ReasonFinalized                ReasonCode = "settlement_finalized"
	ReasonInvalidTransition        ReasonCode = "invalid_transition"
	ReasonPrincipalMismatch        ReasonCode = "authenticated_principal_mismatch"
	ReasonReservationExceeded      ReasonCode = "reservation_exceeded"
	ReasonPolicyNotReady           ReasonCode = "policy_not_ready"
	ReasonBudgetTargetMismatch     ReasonCode = "budget_target_mismatch"
	ReasonWorkloadTargetMismatch   ReasonCode = "workload_target_mismatch"
	ReasonModelNotReady            ReasonCode = "model_not_ready"
	ReasonProviderUnavailable      ReasonCode = "provider_unavailable"
	ReasonNotRoutable              ReasonCode = "model_not_routable"
	ReasonGovernanceInfeasible     ReasonCode = "governance_infeasible"
	ReasonQualityStale             ReasonCode = "quality_observation_stale"
	ReasonQualityBelowMinimum      ReasonCode = "quality_below_minimum"
	ReasonPricingIncomplete        ReasonCode = "pricing_incomplete"
	ReasonPricingStale             ReasonCode = "pricing_stale"
	ReasonContextLimit             ReasonCode = "context_limit_exceeded"
	ReasonLatencyUnavailable       ReasonCode = "latency_observation_unavailable"
	ReasonLatencyExceeded          ReasonCode = "latency_guardrail_exceeded"
	ReasonStrictCapUnverified      ReasonCode = "strict_cap_unverified"
	ReasonInsufficientCalibration  ReasonCode = "insufficient_calibration"
	ReasonReservationMethodUnknown ReasonCode = "reservation_method_unknown"
	ReasonBudgetWindowConflict     ReasonCode = "budget_window_conflict"
)

type ReservationState string

const (
	StateReserved             ReservationState = "RESERVED"
	StateDispatchPending      ReservationState = "DISPATCH_PENDING"
	StateDispatched           ReservationState = "DISPATCHED"
	StateUnresolved           ReservationState = "UNRESOLVED"
	StateSettledProvisional   ReservationState = "SETTLED_PROVISIONAL"
	StateCorrectedProvisional ReservationState = "CORRECTED_PROVISIONAL"
	StateFinalized            ReservationState = "FINALIZED"
	StateCanceledUnbilled     ReservationState = "CANCELED_UNBILLED"
	StateFailedUnbilled       ReservationState = "FAILED_UNBILLED"
)

type OutboxState string

const (
	OutboxPending   OutboxState = "PENDING"
	OutboxClaimed   OutboxState = "CLAIMED"
	OutboxDelivered OutboxState = "DELIVERED"
	OutboxCanceled  OutboxState = "CANCELED"
)

type DispatchStatus string

const (
	DispatchClaimed        DispatchStatus = "CLAIMED"
	DispatchDelivered      DispatchStatus = "DELIVERED"
	DispatchAmbiguous      DispatchStatus = "AMBIGUOUS"
	DispatchFailedUnbilled DispatchStatus = "FAILED_UNBILLED"
)

type AdmitRequest struct {
	RequestID         string   `json:"request_id"`
	Namespace         string   `json:"namespace"`
	TenantID          string   `json:"tenant_id"`
	WorkloadUID       string   `json:"workload_uid"`
	Team              string   `json:"team,omitempty"`
	Application       string   `json:"application,omitempty"`
	SensitiveData     bool     `json:"sensitive_data,omitempty"`
	AllowedZones      []string `json:"allowed_zones,omitempty"`
	BudgetPolicyName  string   `json:"budget_policy_name"`
	RoutingPolicyName string   `json:"routing_policy_name"`
	InputTokens       int64    `json:"input_tokens,omitempty"`
	InputTokensExact  bool     `json:"input_tokens_exact,omitempty"`
	MaxOutputTokens   int64    `json:"max_output_tokens,omitempty"`
	RequireApproval   bool     `json:"require_approval,omitempty"`
	CohortID          string   `json:"cohort_id,omitempty"`
	CohortIndex       int64    `json:"cohort_index,omitempty"`

	// Authenticated fields are populated by the trusted HTTP identity boundary,
	// never decoded from the request body.
	AuthenticatedTenantID    string `json:"-"`
	AuthenticatedWorkloadUID string `json:"-"`
	AuthenticatedNamespace   string `json:"-"`
}

type AdmitResponse struct {
	Decision           Decision    `json:"decision"`
	ReasonCode         ReasonCode  `json:"reason_code"`
	SelectedDeployment string      `json:"selected_deployment,omitempty"`
	ReservationID      string      `json:"reservation_id,omitempty"`
	ProviderAttemptID  string      `json:"provider_attempt_id,omitempty"`
	ReservedCostMicros MoneyMicros `json:"reserved_cost_micros,omitempty"`
	ReservationMode    string      `json:"reservation_method,omitempty"`
	AllocatedRiskPPB   int64       `json:"allocated_risk_ppb,omitempty"`
	RiskLevel          string      `json:"risk_level,omitempty"`
	PolicyVersion      string      `json:"policy_version,omitempty"`
	PricingVersion     string      `json:"pricing_version,omitempty"`
	Expiry             string      `json:"expiry,omitempty"`
	TraceID            string      `json:"trace_id,omitempty"`
}

type DispatchRequest struct {
	RequestID                string         `json:"request_id"`
	EventID                  string         `json:"event_id"`
	TenantID                 string         `json:"tenant_id"`
	WorkloadUID              string         `json:"workload_uid"`
	ProviderAttemptID        string         `json:"provider_attempt_id"`
	Status                   DispatchStatus `json:"status"`
	AuthenticatedTenantID    string         `json:"-"`
	AuthenticatedWorkloadUID string         `json:"-"`
}

type SettleRequest struct {
	RequestID                string          `json:"request_id"`
	SettlementID             string          `json:"settlement_id"`
	TenantID                 string          `json:"tenant_id"`
	WorkloadUID              string          `json:"workload_uid"`
	ActualCostMicros         MoneyMicros     `json:"actual_cost_micros"`
	LegacyActualCost         json.RawMessage `json:"actual_cost,omitempty"`
	ActualInput              int64           `json:"actual_input_tokens,omitempty"`
	ActualOutput             int64           `json:"actual_output_tokens,omitempty"`
	UsageVersion             int64           `json:"usage_version"`
	Final                    bool            `json:"final"`
	ErrorStatus              string          `json:"error_status,omitempty"`
	AuthenticatedTenantID    string          `json:"-"`
	AuthenticatedWorkloadUID string          `json:"-"`
}

type CancelRequest struct {
	RequestID                string `json:"request_id"`
	EventID                  string `json:"event_id"`
	TenantID                 string `json:"tenant_id"`
	WorkloadUID              string `json:"workload_uid"`
	Reason                   string `json:"reason,omitempty"`
	AuthoritativeUnbilled    bool   `json:"authoritative_unbilled,omitempty"`
	AuthenticatedTenantID    string `json:"-"`
	AuthenticatedWorkloadUID string `json:"-"`
}

type LiabilityResponse struct {
	TenantID                   string      `json:"tenant_id"`
	SettledSpendMicros         MoneyMicros `json:"settled_spend_micros"`
	OutstandingLiabilityMicros MoneyMicros `json:"outstanding_liability_micros"`
	CarriedAdjustmentMicros    MoneyMicros `json:"carried_adjustment_micros"`
	AvailableBudgetMicros      MoneyMicros `json:"available_budget_micros"`
	ActiveReservations         int         `json:"active_reservations"`
}

type Reservation struct {
	RequestID                   string
	TenantID                    string
	WorkloadUID                 string
	SelectedDeployment          string
	ProviderAttemptID           string
	OutboxID                    string
	OutboxState                 OutboxState
	State                       ReservationState
	ReservedCostMicros          MoneyMicros
	ProvisionalCostMicros       MoneyMicros
	ResidualHoldMicros          MoneyMicros
	UsageVersion                int64
	Finalized                   bool
	PolicyVersion               string
	PricingVersion              string
	ReservationMode             string
	RiskLevel                   string
	AllocatedRiskPPB            int64
	InputPriceMicrosPerMillion  int64
	OutputPriceMicrosPerMillion int64
	AdmissionFingerprint        string
	CandidateSnapshotVersion    string
	CohortID                    string
	CohortIndex                 int64
	Expiry                      time.Time
}

type inboxEvent struct {
	RequestID   string
	Kind        string
	PayloadHash string
}

type tenantLedger struct {
	BudgetMicros            MoneyMicros
	SettledMicros           MoneyMicros
	ReservedMicros          MoneyMicros
	CarriedAdjustmentMicros MoneyMicros
	ActiveReservations      int
	Requests                map[string]struct{}
	BudgetIdentity          string
}

type Engine struct {
	mu           sync.Mutex
	reservations map[string]Reservation
	inbox        map[string]inboxEvent
	tenants      map[string]*tenantLedger
	now          func() time.Time
}

func NewEngine() *Engine {
	return &Engine{
		reservations: map[string]Reservation{},
		inbox:        map[string]inboxEvent{},
		tenants:      map[string]*tenantLedger{},
		now:          time.Now,
	}
}

func (e *Engine) Ready(context.Context) error { return nil }

func (e *Engine) Admit(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) (AdmitResponse, error) {
	if err := validateAdmitRequest(req); err != nil {
		return AdmitResponse{}, err
	}
	snapshot := BuildPolicySnapshot(budget, routing)
	fingerprint := admissionFingerprint(req, budget, routing, candidates)

	e.mu.Lock()
	defer e.mu.Unlock()

	if existing, exists := e.reservations[req.RequestID]; exists {
		if err := matchPrincipal(existing, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
			return AdmitResponse{}, err
		}
		if existing.AdmissionFingerprint != fingerprint {
			return AdmitResponse{}, errors.New("duplicate request_id has conflicting immutable admission payload")
		}
		if existing.State == StateCanceledUnbilled || existing.State == StateFailedUnbilled || existing.State == StateFinalized {
			return AdmitResponse{Decision: DecisionReject, ReasonCode: ReasonInvalidTransition, TraceID: req.RequestID}, nil
		}
		return responseForReservation(existing, ReasonDuplicateRequest), nil
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
	tenant, err := e.bindTenantBudget(req.AuthenticatedTenantID, budgetMicros, budgetIdentity(budget))
	if err != nil {
		return decisionResponse(req.RequestID, DecisionReject, ReasonBudgetWindowConflict, budget, routing), nil
	}
	choice, infeasibleReason, err := chooseAdmission(req, routing, candidates, tenant.available())
	if err != nil {
		return AdmitResponse{}, err
	}
	if infeasibleReason != "" {
		decision := DecisionAbstain
		if infeasibleReason == ReasonBudgetUnavailable {
			decision = DecisionQueue
		}
		return decisionResponse(req.RequestID, decision, infeasibleReason, budget, routing), nil
	}
	best, reservedCost := choice.Candidate, choice.Reservation
	if choice.Method == "govar_fixed_cohort" {
		for _, existing := range e.reservations {
			if existing.TenantID == req.AuthenticatedTenantID && existing.CohortID == req.CohortID && existing.CohortIndex == req.CohortIndex {
				return decisionResponse(req.RequestID, DecisionReject, ReasonDuplicateConflict, budget, routing), nil
			}
		}
	}

	expiry := e.now().UTC().Add(5 * time.Minute)
	res := Reservation{
		RequestID: req.RequestID, TenantID: req.AuthenticatedTenantID, WorkloadUID: req.AuthenticatedWorkloadUID,
		SelectedDeployment: best.ModelRef, ProviderAttemptID: req.RequestID + ":attempt:1",
		OutboxID: req.RequestID + ":dispatch:1", OutboxState: OutboxPending, State: StateReserved,
		ReservedCostMicros: reservedCost, ResidualHoldMicros: reservedCost,
		PolicyVersion: policyVersion(budget, routing), PricingVersion: best.PricingVersion,
		ReservationMode: choice.Method, RiskLevel: riskLevel(snapshot), AllocatedRiskPPB: choice.AllocatedRiskPPB, Expiry: expiry,
		InputPriceMicrosPerMillion:  best.InputPriceMicrosPerMillion,
		OutputPriceMicrosPerMillion: best.OutputPriceMicrosPerMillion,
		AdmissionFingerprint:        fingerprint,
		CandidateSnapshotVersion:    best.SnapshotVersion,
		CohortID:                    req.CohortID,
		CohortIndex:                 req.CohortIndex,
	}
	tenant.ReservedMicros += reservedCost
	tenant.Requests[req.RequestID] = struct{}{}
	e.reservations[req.RequestID] = res
	return responseForReservation(res, ReasonHighestUtility), nil
}

func (e *Engine) Dispatch(req DispatchRequest) (Reservation, ReasonCode, error) {
	if err := validateEventPrincipal(req.RequestID, req.EventID, req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	payloadHash := eventPayloadHash("dispatch", req.RequestID, req.TenantID, req.WorkloadUID, req.ProviderAttemptID, string(req.Status))
	if prior, ok := e.inbox[req.EventID]; ok {
		if prior.RequestID != req.RequestID {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event_id is already bound to another request")
		}
		if prior.PayloadHash != payloadHash {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event_id replay has conflicting immutable payload")
		}
		res, exists := e.reservations[req.RequestID]
		if !exists {
			return Reservation{}, ReasonDuplicateEvent, errors.New("duplicate event references unknown request")
		}
		if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
			return Reservation{}, ReasonPrincipalMismatch, err
		}
		return res, ReasonDuplicateEvent, nil
	}
	res, ok := e.reservations[req.RequestID]
	if !ok {
		return Reservation{}, ReasonReservationNotFound, errors.New("reservation not found")
	}
	if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	if req.ProviderAttemptID != res.ProviderAttemptID {
		return res, ReasonInvalidTransition, errors.New("provider_attempt_id does not match the reserved attempt")
	}
	code, err := applyDispatch(&res, req.Status)
	if err != nil {
		return res, code, err
	}
	e.reservations[req.RequestID] = res
	e.inbox[req.EventID] = inboxEvent{RequestID: req.RequestID, Kind: "dispatch", PayloadHash: payloadHash}
	return res, code, nil
}

func (e *Engine) Settle(req SettleRequest) (Reservation, ReasonCode, error) {
	if err := validateSettleRequest(req); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	payloadHash := eventPayloadHash("settlement", req.RequestID, req.TenantID, req.WorkloadUID, fmt.Sprint(req.ActualCostMicros), fmt.Sprint(req.ActualInput), fmt.Sprint(req.ActualOutput), fmt.Sprint(req.UsageVersion), fmt.Sprint(req.Final))
	if prior, ok := e.inbox[req.SettlementID]; ok {
		if prior.RequestID != req.RequestID {
			return Reservation{}, ReasonDuplicateEvent, errors.New("settlement_id is already bound to another request")
		}
		if prior.PayloadHash != payloadHash {
			return Reservation{}, ReasonDuplicateEvent, errors.New("settlement_id replay has conflicting immutable payload")
		}
		res, exists := e.reservations[req.RequestID]
		if !exists {
			return Reservation{}, ReasonSettlementDuplicate, errors.New("duplicate settlement references unknown request")
		}
		if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
			return Reservation{}, ReasonPrincipalMismatch, err
		}
		return res, ReasonSettlementDuplicate, nil
	}
	res, ok := e.reservations[req.RequestID]
	if !ok {
		return Reservation{}, ReasonReservationNotFound, errors.New("reservation not found")
	}
	if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	tenant := e.ensureTenant(res.TenantID, 0)
	code, err := applySettlement(&res, tenant, req)
	if err != nil {
		return res, code, err
	}
	e.reservations[req.RequestID] = res
	e.inbox[req.SettlementID] = inboxEvent{RequestID: req.RequestID, Kind: "settlement", PayloadHash: payloadHash}
	if res.State == StateFinalized {
		delete(tenant.Requests, req.RequestID)
	}
	return res, code, nil
}

func (e *Engine) Cancel(req CancelRequest) (Reservation, ReasonCode, error) {
	if err := validateEventPrincipal(req.RequestID, req.EventID, req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	payloadHash := eventPayloadHash("cancel", req.RequestID, req.TenantID, req.WorkloadUID, req.Reason, fmt.Sprint(req.AuthoritativeUnbilled))
	if prior, ok := e.inbox[req.EventID]; ok {
		if prior.RequestID != req.RequestID {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event_id is already bound to another request")
		}
		if prior.PayloadHash != payloadHash {
			return Reservation{}, ReasonDuplicateEvent, errors.New("event_id replay has conflicting immutable payload")
		}
		res, exists := e.reservations[req.RequestID]
		if !exists {
			return Reservation{}, ReasonDuplicateEvent, errors.New("duplicate event references unknown request")
		}
		if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
			return Reservation{}, ReasonPrincipalMismatch, err
		}
		return res, ReasonDuplicateEvent, nil
	}
	res, ok := e.reservations[req.RequestID]
	if !ok {
		return Reservation{}, ReasonReservationNotFound, errors.New("reservation not found")
	}
	if err := matchPrincipal(res, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID); err != nil {
		return Reservation{}, ReasonPrincipalMismatch, err
	}
	tenant := e.ensureTenant(res.TenantID, 0)
	code, err := applyCancel(&res, tenant, req.AuthoritativeUnbilled)
	if err != nil {
		return res, code, err
	}
	e.reservations[req.RequestID] = res
	e.inbox[req.EventID] = inboxEvent{RequestID: req.RequestID, Kind: "cancel", PayloadHash: payloadHash}
	if res.State == StateCanceledUnbilled || res.State == StateFailedUnbilled {
		delete(tenant.Requests, req.RequestID)
	}
	return res, code, nil
}

func (e *Engine) Liability(tenantID string) LiabilityResponse {
	e.mu.Lock()
	defer e.mu.Unlock()
	tenant := e.ensureTenant(tenantID, 0)
	return LiabilityResponse{
		TenantID: tenantID, SettledSpendMicros: tenant.SettledMicros,
		OutstandingLiabilityMicros: tenant.ReservedMicros,
		CarriedAdjustmentMicros:    tenant.CarriedAdjustmentMicros,
		AvailableBudgetMicros:      tenant.available(), ActiveReservations: len(tenant.Requests),
	}
}

func (e *Engine) LiabilityWithError(tenantID string) (LiabilityResponse, error) {
	return e.Liability(tenantID), nil
}

func applyDispatch(res *Reservation, status DispatchStatus) (ReasonCode, error) {
	switch status {
	case DispatchClaimed:
		if res.State != StateReserved || res.OutboxState != OutboxPending {
			return ReasonInvalidTransition, errors.New("dispatch claim requires a pending reserved outbox")
		}
		res.State, res.OutboxState = StateDispatchPending, OutboxClaimed
		return ReasonDispatchClaimed, nil
	case DispatchDelivered:
		if res.State != StateDispatchPending || res.OutboxState != OutboxClaimed {
			return ReasonInvalidTransition, errors.New("delivery requires a claimed dispatch")
		}
		res.State, res.OutboxState = StateDispatched, OutboxDelivered
		return ReasonDispatchDelivered, nil
	case DispatchAmbiguous:
		if res.State != StateDispatchPending || res.OutboxState != OutboxClaimed {
			return ReasonInvalidTransition, errors.New("ambiguous result requires a claimed dispatch")
		}
		res.State = StateUnresolved
		return ReasonDispatchUnresolved, nil
	case DispatchFailedUnbilled:
		return ReasonInvalidTransition, errors.New("authoritative unbilled failure must use cancel so release is atomic")
	default:
		return ReasonInvalidTransition, fmt.Errorf("unsupported dispatch status %q", status)
	}
}

func applySettlement(res *Reservation, tenant *tenantLedger, req SettleRequest) (ReasonCode, error) {
	if req.ActualCostMicros == 0 && (req.ActualInput > 0 || req.ActualOutput > 0) {
		cost, err := costFromPriceMicros(res.InputPriceMicrosPerMillion, res.OutputPriceMicrosPerMillion, req.ActualInput, req.ActualOutput)
		if err != nil {
			return ReasonInvalidTransition, err
		}
		req.ActualCostMicros = cost
	}
	if req.ActualCostMicros < 0 || req.UsageVersion <= 0 {
		return ReasonInvalidTransition, errors.New("actual_cost_micros must be non-negative and usage_version positive")
	}
	if res.State == StateCanceledUnbilled || res.State == StateFailedUnbilled {
		return ReasonInvalidTransition, errors.New("cannot settle an authoritative unbilled terminal request")
	}
	if res.State == StateReserved || res.OutboxState == OutboxPending {
		return ReasonInvalidTransition, errors.New("settlement requires a previously claimed provider attempt")
	}
	if res.Finalized {
		if req.UsageVersion <= res.UsageVersion && req.ActualCostMicros == res.ProvisionalCostMicros {
			return ReasonSettlementDuplicate, nil
		}
		if req.UsageVersion <= res.UsageVersion {
			return ReasonInvalidTransition, errors.New("post-finality correction version is not monotone")
		}
		delta := req.ActualCostMicros - res.ProvisionalCostMicros
		res.UsageVersion = req.UsageVersion
		if delta > 0 {
			tenant.CarriedAdjustmentMicros += delta
			res.ProvisionalCostMicros = req.ActualCostMicros
			return ReasonReservationExceeded, nil
		}
		// Downward post-final corrections are audit-only. They cannot mint an
		// availability credit after the authoritative hold has been released.
		return ReasonCorrection, nil
	}
	// Authoritative usage is evidence that the provider attempt was delivered,
	// even when its acknowledgement raced or was lost.
	res.OutboxState = OutboxDelivered
	if req.UsageVersion < res.UsageVersion {
		return ReasonInvalidTransition, errors.New("usage_version is stale")
	}
	if req.UsageVersion == res.UsageVersion && res.UsageVersion != 0 {
		if req.ActualCostMicros != res.ProvisionalCostMicros {
			return ReasonInvalidTransition, errors.New("same usage_version has conflicting cost")
		}
		if !req.Final {
			return ReasonSettlementDuplicate, nil
		}
	} else {
		previous := res.ProvisionalCostMicros
		delta := req.ActualCostMicros - previous
		newResidual := res.ReservedCostMicros - req.ActualCostMicros
		if newResidual < 0 {
			newResidual = 0
		}
		oldHold := res.ResidualHoldMicros
		tenant.SettledMicros += delta
		tenant.ReservedMicros += newResidual - oldHold
		res.ProvisionalCostMicros = req.ActualCostMicros
		res.ResidualHoldMicros = newResidual
		res.UsageVersion = req.UsageVersion
	}

	late := res.State == StateUnresolved || time.Now().UTC().After(res.Expiry)
	if req.Final {
		tenant.ReservedMicros -= res.ResidualHoldMicros
		res.ResidualHoldMicros = 0
		res.Finalized = true
		res.State = StateFinalized
		if req.ActualCostMicros > res.ReservedCostMicros {
			return ReasonReservationExceeded, nil
		}
		return ReasonFinalized, nil
	}
	if req.ActualCostMicros > res.ReservedCostMicros {
		res.State = StateCorrectedProvisional
		return ReasonReservationExceeded, nil
	}
	if res.UsageVersion > 1 {
		res.State = StateCorrectedProvisional
		return ReasonCorrection, nil
	}
	res.State = StateSettledProvisional
	if late {
		return ReasonLateSettlement, nil
	}
	return ReasonProvisionalSettlement, nil
}

func applyCancel(res *Reservation, tenant *tenantLedger, authoritative bool) (ReasonCode, error) {
	switch res.State {
	case StateReserved:
		if res.OutboxState != OutboxPending {
			return ReasonInvalidTransition, errors.New("reserved request has non-pending outbox")
		}
		res.OutboxState = OutboxCanceled
		res.State = StateCanceledUnbilled
		tenant.ReservedMicros -= res.ResidualHoldMicros
		res.ResidualHoldMicros = 0
		return ReasonCanceled, nil
	case StateDispatchPending, StateDispatched, StateUnresolved:
		if !authoritative {
			res.State = StateUnresolved
			return ReasonCancellationUnclear, nil
		}
		res.OutboxState = OutboxCanceled
		res.State = StateFailedUnbilled
		tenant.ReservedMicros -= res.ResidualHoldMicros
		res.ResidualHoldMicros = 0
		return ReasonCanceled, nil
	case StateSettledProvisional, StateCorrectedProvisional, StateFinalized:
		return ReasonInvalidTransition, errors.New("settled request cannot be canceled")
	case StateCanceledUnbilled, StateFailedUnbilled:
		return ReasonDuplicateEvent, nil
	default:
		return ReasonInvalidTransition, errors.New("unsupported cancellation state")
	}
}

func validateAdmitRequest(req AdmitRequest) error {
	if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.WorkloadUID) == "" {
		return errors.New("request_id, tenant_id, and workload_uid are required")
	}
	if req.InputTokens < 0 || req.MaxOutputTokens < 0 {
		return errors.New("token counts cannot be negative")
	}
	if req.Namespace == "" || req.Namespace != req.AuthenticatedNamespace {
		return errors.New("request namespace does not match authenticated namespace")
	}
	return validatePrincipal(req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID)
}

func validateSettleRequest(req SettleRequest) error {
	if strings.TrimSpace(req.SettlementID) == "" {
		return errors.New("settlement_id is required")
	}
	if legacy := strings.TrimSpace(string(req.LegacyActualCost)); legacy != "" && legacy != "0" && legacy != "0.0" {
		return errors.New("actual_cost is deprecated; send integer actual_cost_micros")
	}
	return validateEventPrincipal(req.RequestID, req.SettlementID, req.TenantID, req.WorkloadUID, req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID)
}

func validateEventPrincipal(requestID, eventID, tenantID, workloadUID, authenticatedTenantID, authenticatedWorkloadUID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(eventID) == "" {
		return errors.New("request_id and event_id are required")
	}
	return validatePrincipal(tenantID, workloadUID, authenticatedTenantID, authenticatedWorkloadUID)
}

func validatePrincipal(tenantID, workloadUID, authenticatedTenantID, authenticatedWorkloadUID string) error {
	if tenantID == "" || workloadUID == "" || authenticatedTenantID == "" || authenticatedWorkloadUID == "" {
		return errors.New("tenant_id, workload_uid, and authenticated principal are required")
	}
	if tenantID != authenticatedTenantID || workloadUID != authenticatedWorkloadUID {
		return errors.New("request principal does not match authenticated principal")
	}
	return nil
}

func matchPrincipal(res Reservation, tenantID, workloadUID string) error {
	if res.TenantID != tenantID || res.WorkloadUID != workloadUID {
		return errors.New("authenticated principal does not own request")
	}
	return nil
}

func responseForReservation(res Reservation, reason ReasonCode) AdmitResponse {
	return AdmitResponse{
		Decision: DecisionAdmit, ReasonCode: reason, SelectedDeployment: res.SelectedDeployment,
		ReservationID: res.RequestID, ProviderAttemptID: res.ProviderAttemptID,
		ReservedCostMicros: res.ReservedCostMicros, ReservationMode: res.ReservationMode,
		AllocatedRiskPPB: res.AllocatedRiskPPB, RiskLevel: res.RiskLevel,
		PolicyVersion: res.PolicyVersion, PricingVersion: res.PricingVersion,
		Expiry: res.Expiry.UTC().Format(time.RFC3339), TraceID: res.RequestID,
	}
}

func decisionResponse(trace string, decision Decision, reason ReasonCode, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) AdmitResponse {
	return AdmitResponse{Decision: decision, ReasonCode: reason, PolicyVersion: policyVersion(budget, routing), PricingVersion: "provider-pricing-live", TraceID: trace}
}

func (e *Engine) ensureTenant(tenantID string, budget MoneyMicros) *tenantLedger {
	tenant, ok := e.tenants[tenantID]
	if !ok {
		tenant = &tenantLedger{BudgetMicros: budget, Requests: map[string]struct{}{}}
		e.tenants[tenantID] = tenant
	}
	return tenant
}

func (e *Engine) bindTenantBudget(tenantID string, budget MoneyMicros, identity string) (*tenantLedger, error) {
	tenant := e.ensureTenant(tenantID, budget)
	if tenant.BudgetIdentity != "" && tenant.BudgetIdentity != identity && (tenant.ReservedMicros != 0 || tenant.SettledMicros != 0 || tenant.CarriedAdjustmentMicros != 0) {
		return nil, errBudgetWindowChanged
	}
	tenant.BudgetIdentity = identity
	tenant.BudgetMicros = budget
	return tenant, nil
}

func (t *tenantLedger) available() MoneyMicros {
	return t.BudgetMicros - t.SettledMicros - t.ReservedMicros - t.CarriedAdjustmentMicros
}

func rankedCandidates(candidates []Candidate, inputTokens, maxOutputTokens int64, objective string) []Candidate {
	out := append([]Candidate(nil), candidates...)
	sort.Slice(out, func(i, j int) bool {
		left, leftErr := expectedCostMicros(out[i], inputTokens, maxOutputTokens)
		right, rightErr := expectedCostMicros(out[j], inputTokens, maxOutputTokens)
		if leftErr != nil || rightErr != nil {
			return out[i].ModelRef < out[j].ModelRef
		}
		if strings.EqualFold(objective, "quality") && out[i].QualityTier != out[j].QualityTier {
			return qualityRank(out[i].QualityTier) > qualityRank(out[j].QualityTier)
		}
		if left != right {
			return left < right
		}
		return out[i].ModelRef < out[j].ModelRef
	})
	return out
}

func qualityRank(t aiopsv1alpha1.Tier) int {
	switch t {
	case aiopsv1alpha1.TierHigh:
		return 3
	case aiopsv1alpha1.TierMedium:
		return 2
	case aiopsv1alpha1.TierLow:
		return 1
	default:
		return 0
	}
}

func eventPayloadHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func admissionFingerprint(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate) string {
	parts := []string{"admission-v1", req.Namespace, req.TenantID, req.WorkloadUID, req.Team, req.Application,
		fmt.Sprint(req.SensitiveData), strings.Join(req.AllowedZones, "\x1f"), req.BudgetPolicyName,
		req.RoutingPolicyName, fmt.Sprint(req.InputTokens), fmt.Sprint(req.MaxOutputTokens),
		budget.Spec.BudgetEUR.String(), policyVersion(budget, routing), budgetIdentity(budget),
		routing.Annotations[AnnotationReservationMethod], routing.Annotations[AnnotationMeanOutputTokens],
		routing.Annotations[AnnotationMarginTokens], routing.Annotations[AnnotationQuantileTokens],
		routing.Annotations[AnnotationAdaptiveTokens], routing.Annotations[AnnotationCalibrationSupport],
		routing.Annotations[AnnotationCalibrationDrift], routing.Annotations[AnnotationCohortSize],
		routing.Annotations[AnnotationTenantRiskPPB]}
	parts = append(parts, req.CohortID, fmt.Sprint(req.CohortIndex))
	for _, c := range candidates {
		parts = append(parts, c.ModelRef, c.ProviderRef, c.Region,
			fmt.Sprint(c.InputPriceMicrosPerMillion), fmt.Sprint(c.OutputPriceMicrosPerMillion),
			c.PricingVersion, c.SnapshotVersion, fmt.Sprint(c.ContextWindow), fmt.Sprint(c.QualityScore), fmt.Sprint(c.VerifiedOutputCap))
	}
	return eventPayloadHash(parts...)
}

func budgetIdentity(budget aiopsv1alpha1.AIBudgetPolicy) string {
	return eventPayloadHash("budget-window-v1", string(budget.UID), budget.Namespace, budget.Name,
		fmt.Sprint(budget.Generation), budget.Spec.Period, budget.Spec.BudgetEUR.String(),
		budget.Spec.Target.Namespace, budget.Spec.Target.Team, budget.Spec.Target.Application)
}

func policyVersion(budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) string {
	return fmt.Sprintf("%s:%d@%s|%s:%d@%s", budget.Name, budget.Generation, budget.ResourceVersion, routing.Name, routing.Generation, routing.ResourceVersion)
}

func riskLevel(snapshot PolicySnapshot) string {
	if snapshot.RequireSovereigntyCompliance {
		return "strict"
	}
	return "bounded"
}
