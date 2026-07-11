package faultinjector

import (
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
)

type DuplicateSettlementResult struct {
	FirstAccepted          bool   `json:"first_accepted"`
	SameEventAccepted      bool   `json:"same_event_accepted"`
	SecondDistinctRejected bool   `json:"second_distinct_rejected"`
	Error                  string `json:"error,omitempty"`
}

type ReservationExpiryResult struct {
	Expired            bool    `json:"expired"`
	ReservedAfter      float64 `json:"reserved_after"`
	AvailableAfter     float64 `json:"available_after"`
	SecondExpireFailed bool    `json:"second_expire_failed"`
	Error              string  `json:"error,omitempty"`
}

type TelemetryFaultResult struct {
	Action        admission.Action `json:"action"`
	Reason        string           `json:"reason"`
	Abstained     bool             `json:"abstained"`
	TelemetrySeen bool             `json:"telemetry_seen"`
}

func SimulateDuplicateSettlement() DuplicateSettlementResult {
	state := ledger.TenantState{TenantID: "tenant-a", Budget: 20}
	l := ledger.NewRequestLedger()
	if err := l.Reserve(&state, "dup-1", "m1", 8); err != nil {
		return DuplicateSettlementResult{Error: err.Error()}
	}
	if err := l.SettleOnce(&state, "dup-1", "evt-1", 6); err != nil {
		return DuplicateSettlementResult{Error: err.Error()}
	}
	result := DuplicateSettlementResult{FirstAccepted: true}
	if err := l.SettleOnce(&state, "dup-1", "evt-1", 6); err == nil {
		result.SameEventAccepted = true
	}
	if err := l.SettleOnce(&state, "dup-1", "evt-2", 6); err != nil {
		result.SecondDistinctRejected = true
		result.Error = err.Error()
	}
	return result
}

func SimulateReservationExpiry() ReservationExpiryResult {
	state := ledger.TenantState{TenantID: "tenant-a", Budget: 15}
	l := ledger.NewRequestLedger()
	if err := l.Reserve(&state, "exp-1", "m1", 5); err != nil {
		return ReservationExpiryResult{Error: err.Error()}
	}
	if err := l.Expire(&state, "exp-1"); err != nil {
		return ReservationExpiryResult{Error: err.Error()}
	}
	result := ReservationExpiryResult{
		Expired:        true,
		ReservedAfter:  state.Reserved,
		AvailableAfter: state.Available(),
	}
	if err := l.Expire(&state, "exp-1"); err != nil {
		result.SecondExpireFailed = true
	}
	return result
}

func SimulateTelemetryFault() TelemetryFaultResult {
	policy := admission.Policy{
		StrictMode:            true,
		AllowQueue:            true,
		RequireFreshSignals:   true,
		RequireStrongEvidence: true,
	}
	tenant := ledger.TenantState{TenantID: "tenant-a", Budget: 10}
	decision := admission.Decide(policy, tenant, []admission.Candidate{
		{
			Model:              "m-telemetry-stale",
			UtilityScore:       0.9,
			ReservationCost:    3,
			GovernanceAllowed:  true,
			TelemetryFresh:     false,
			CalibrationTrusted: true,
			EvidenceStrong:     true,
		},
		{
			Model:              "m-evidence-weak",
			UtilityScore:       0.85,
			ReservationCost:    3,
			GovernanceAllowed:  true,
			TelemetryFresh:     true,
			CalibrationTrusted: true,
			EvidenceStrong:     false,
		},
	})
	return TelemetryFaultResult{
		Action:        decision.Action,
		Reason:        decision.Reason,
		Abstained:     decision.Action == admission.ActionAbstain,
		TelemetrySeen: false,
	}
}
