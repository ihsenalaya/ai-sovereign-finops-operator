package admission

import (
	"testing"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
)

func TestDecideAdmitsBestFeasibleCandidate(t *testing.T) {
	tenant := ledger.TenantState{TenantID: "t1", Budget: 10, Settled: 2, Reserved: 1}
	decision := Decide(Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true}, tenant, []Candidate{
		{Model: "unsafe", UtilityScore: 0.99, ReservationCost: 3, GovernanceAllowed: false, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false},
		{Model: "expensive", UtilityScore: 0.95, ReservationCost: 8, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false},
		{Model: "best", UtilityScore: 0.90, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false},
	})
	if decision.Action != ActionAdmit {
		t.Fatalf("expected admit, got %s", decision.Action)
	}
	if decision.ReasonCode != ReasonCodeHighestUtility {
		t.Fatalf("expected highest utility reason code, got %s", decision.ReasonCode)
	}
	if decision.Model != "best" {
		t.Fatalf("expected best, got %s", decision.Model)
	}
}

func TestDecideQueuesWhenNoFeasibleCandidate(t *testing.T) {
	tenant := ledger.TenantState{TenantID: "t1", Budget: 5, Settled: 5}
	decision := Decide(Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true}, tenant, []Candidate{
		{Model: "m1", UtilityScore: 1, ReservationCost: 1, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false},
	})
	if decision.Action != ActionQueue {
		t.Fatalf("expected queue, got %s", decision.Action)
	}
	if decision.ReasonCode != ReasonCodeBudgetUnavailable {
		t.Fatalf("expected budget unavailable, got %s", decision.ReasonCode)
	}
}

func TestDecideAbstainsWhenEvidenceIsRequiredButWeak(t *testing.T) {
	tenant := ledger.TenantState{TenantID: "t1", Budget: 10}
	decision := Decide(Policy{StrictMode: false, AllowQueue: true, RequireFreshSignals: false, RequireStrongEvidence: true}, tenant, []Candidate{
		{Model: "m1", UtilityScore: 1, ReservationCost: 1, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: false, Drifted: false},
	})
	if decision.Action != ActionAbstain {
		t.Fatalf("expected abstain, got %s", decision.Action)
	}
	if decision.ReasonCode != ReasonCodeInsufficientEvidence {
		t.Fatalf("expected insufficient evidence, got %s", decision.ReasonCode)
	}
}

func TestDecideAbstainsWhenDriftIsBlocked(t *testing.T) {
	tenant := ledger.TenantState{TenantID: "t1", Budget: 10}
	decision := Decide(Policy{StrictMode: false, AllowQueue: true, BlockOnDrift: true}, tenant, []Candidate{
		{Model: "m1", UtilityScore: 1, ReservationCost: 1, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: true},
	})
	if decision.Action != ActionAbstain {
		t.Fatalf("expected abstain, got %s", decision.Action)
	}
	if decision.ReasonCode != ReasonCodeDriftBlocked {
		t.Fatalf("expected drift blocked, got %s", decision.ReasonCode)
	}
}
