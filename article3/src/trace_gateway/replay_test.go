package tracegateway

import (
	"testing"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
)

func TestReplayDelayedSettlement(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 2,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.8, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
		{
			RequestID:       "r2",
			TenantID:        "team-a",
			ArrivalStep:     1,
			SettlementDelay: 1,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.7, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 10},
	}, ReplayConfig{
		Policy: admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if len(result.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(result.Events))
	}

	tenant := result.Tenants["team-a"]
	if tenant.Settled != 8 {
		t.Fatalf("expected settled 8, got %.2f", tenant.Settled)
	}
	if tenant.Reserved != 0 {
		t.Fatalf("expected reserved 0, got %.2f", tenant.Reserved)
	}
}

func TestReplayQueuesWhenBudgetStillReserved(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 3,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.9, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
		{
			RequestID:       "r2",
			TenantID:        "team-a",
			ArrivalStep:     1,
			SettlementDelay: 1,
			ActualCost:      3,
			MaxQueueRetries: 0,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.8, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 7},
	}, ReplayConfig{
		Policy:          admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		QueueRetryDelay: 1,
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	queueSeen := false
	for _, ev := range result.Events {
		if ev.RequestID == "r2" && ev.Type == EventQueued {
			queueSeen = true
		}
	}
	if !queueSeen {
		t.Fatal("expected second request to be queued")
	}
}

func TestReplayQueuedRequestCanBeRetriedAndAdmitted(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.9, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
		{
			RequestID:       "r2",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      3,
			MaxQueueRetries: 2,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.8, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: false}},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 9},
	}, ReplayConfig{
		Policy:          admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		QueueRetryDelay: 1,
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	queueSeen := false
	admitSeen := false
	for _, ev := range result.Events {
		if ev.RequestID == "r2" && ev.Type == EventQueued {
			queueSeen = true
		}
		if ev.RequestID == "r2" && ev.Type == EventAdmitted {
			admitSeen = true
		}
	}
	if !queueSeen {
		t.Fatal("expected queued event for r2")
	}
	if !admitSeen {
		t.Fatal("expected retried request r2 to be admitted later")
	}
}

func TestReplayAbstainsWhenCandidateIsDriftedAndPolicyBlocksIt(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{Base: admission.Candidate{Model: "m1", UtilityScore: 0.9, ReservationCost: 5, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true, Drifted: true}},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 10},
	}, ReplayConfig{
		Policy: admission.Policy{AllowQueue: true, BlockOnDrift: true},
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].Type != EventAbstained {
		t.Fatalf("expected single abstained event, got %+v", result.Events)
	}
}

func TestReplayUsesProbabilisticReservationConfig(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      5,
			Candidates: []CandidateSpec{
				{
					Base:                admission.Candidate{Model: "m1", UtilityScore: 0.9, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true},
					PredictedOutputTokens: []int{10, 12, 14, 16},
					InputCost:           1,
					OutputCostPerToken:  0.25,
				},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 10},
	}, ReplayConfig{
		Policy: admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		ReservationConfig: predictor.ReservationConfig{
			Mode:   predictor.ModeMeanStd,
			ZScore: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(result.Events) == 0 || result.Events[0].Type != EventAdmitted {
		t.Fatalf("expected admitted event, got %+v", result.Events)
	}
	if result.Events[0].Amount <= 4.5 {
		t.Fatalf("expected reservation amount to be derived from predictor config, got %.2f", result.Events[0].Amount)
	}
}

func TestReplayBudgetDelayScenarioRegression(t *testing.T) {
	requests, tenantCfgs, err := BuildScenario(ScenarioConfig{
		Kind:         ScenarioBudgetDelay,
		Seed:         100,
		RequestCount: 12,
		TenantBudget: 18,
		QueueRetries: 1,
		Bursty:       true,
	})
	if err != nil {
		t.Fatalf("build scenario failed: %v", err)
	}
	tenants := make(map[string]ledger.TenantState, len(tenantCfgs))
	for k, v := range tenantCfgs {
		tenants[k] = ledger.TenantState{TenantID: v.TenantID, Budget: v.Budget}
	}
	_, err = Replay(requests, tenants, ReplayConfig{
		Policy: admission.Policy{
			StrictMode:          true,
			AllowQueue:          true,
			RequireFreshSignals: true,
		},
		QueueRetryDelay: 1,
		ReservationConfig: predictor.ReservationConfig{
			Mode:   predictor.ModeMeanStd,
			ZScore: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("budget-delay replay regression: %v", err)
	}
}

func TestReplayFallsBackToNonDriftedCandidate(t *testing.T) {
	result, err := Replay([]TraceRequest{
		{
			RequestID:       "d1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      4,
			Candidates: []CandidateSpec{
				{
					Base: admission.Candidate{
						Model:              "premium-drifted",
						UtilityScore:       0.95,
						ReservationCost:    7,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
						Drifted:            true,
					},
				},
				{
					Base: admission.Candidate{
						Model:              "economy-safe",
						UtilityScore:       0.72,
						ReservationCost:    5,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
						Drifted:            false,
					},
				},
			},
		},
	}, map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 10},
	}, ReplayConfig{
		Policy: admission.Policy{
			StrictMode:          true,
			AllowQueue:          true,
			RequireFreshSignals: true,
			BlockOnDrift:        true,
		},
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(result.Events) == 0 || result.Events[0].Type != EventAdmitted {
		t.Fatalf("expected admitted fallback event, got %+v", result.Events)
	}
	if result.Events[0].Model != "economy-safe" {
		t.Fatalf("expected non-drifted fallback model, got %q", result.Events[0].Model)
	}
}
