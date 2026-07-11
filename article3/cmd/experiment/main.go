package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
	tracegateway "github.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway"
)

type experimentOutput struct {
	ExperimentID string                   `json:"experiment_id"`
	Variant      string                   `json:"variant"`
	Config       predictor.ReservationConfig `json:"config"`
	Metrics      tracegateway.Metrics     `json:"metrics"`
	Events       []tracegateway.Event     `json:"events"`
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: experiment <e1|e2>")
	}
	switch os.Args[1] {
	case "e0":
		if err := runE0(); err != nil {
			fail(err.Error())
		}
	case "e1":
		if err := runE1(); err != nil {
			fail(err.Error())
		}
	case "e2":
		if err := runE2(); err != nil {
			fail(err.Error())
		}
	case "e3":
		if err := runE3(); err != nil {
			fail(err.Error())
		}
	case "e4":
		if err := runE4(); err != nil {
			fail(err.Error())
		}
	case "e5":
		if err := runE5(); err != nil {
			fail(err.Error())
		}
	case "e7":
		if err := runE7(); err != nil {
			fail(err.Error())
		}
	case "e1-campaign":
		if err := runE1Campaign(); err != nil {
			fail(err.Error())
		}
	case "e2-campaign":
		if err := runE2Campaign(); err != nil {
			fail(err.Error())
		}
	case "e1-matrix":
		if err := runE1Matrix(); err != nil {
			fail(err.Error())
		}
	case "e2-matrix":
		if err := runE2Matrix(); err != nil {
			fail(err.Error())
		}
	case "e1-compare":
		if err := compareExperiment("E1"); err != nil {
			fail(err.Error())
		}
	case "e2-compare":
		if err := compareExperiment("E2"); err != nil {
			fail(err.Error())
		}
	default:
		fail("unknown experiment")
	}
}

func runE1() error {
	requests := []tracegateway.TraceRequest{
		{
			RequestID:       "r1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 3,
			ActualCost:      7,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m1", UtilityScore: 0.9, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{8, 10, 12, 14},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
		{
			RequestID:       "r2",
			TenantID:        "team-a",
			ArrivalStep:     1,
			SettlementDelay: 2,
			ActualCost:      5,
			MaxQueueRetries: 0,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m1", UtilityScore: 0.8, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{8, 10, 12, 14},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
	}
	tenants := map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 14},
	}
	return runVariants("E1", requests, tenants)
}

func runE2() error {
	requests := []tracegateway.TraceRequest{
		{
			RequestID:       "a1",
			TenantID:        "team-a",
			ArrivalStep:     0,
			SettlementDelay: 2,
			ActualCost:      6,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m1", UtilityScore: 0.95, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{7, 8, 9, 10},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
		{
			RequestID:       "b1",
			TenantID:        "team-b",
			ArrivalStep:     0,
			SettlementDelay: 2,
			ActualCost:      4,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m2", UtilityScore: 0.85, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{4, 5, 6, 7},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
		{
			RequestID:       "a2",
			TenantID:        "team-a",
			ArrivalStep:     1,
			SettlementDelay: 2,
			ActualCost:      5,
			MaxQueueRetries: 0,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m1", UtilityScore: 0.8, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{7, 8, 9, 10},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
		{
			RequestID:       "a3",
			TenantID:        "team-a",
			ArrivalStep:     1,
			SettlementDelay: 1,
			ActualCost:      4.5,
			MaxQueueRetries: 0,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model: "m1", UtilityScore: 0.75, GovernanceAllowed: true, TelemetryFresh: true, CalibrationTrusted: true, EvidenceStrong: true,
					},
					PredictedOutputTokens: []int{7, 8, 9, 10},
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
			},
		},
	}
	tenants := map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 15},
		"team-b": {TenantID: "team-b", Budget: 8},
	}
	return runVariants("E2", requests, tenants)
}

func runE1Campaign() error {
	variants := defaultVariants()
	if err := runCampaign("E1", tracegateway.ScenarioConfig{
		Kind:         tracegateway.ScenarioBudgetDelay,
		Seed:         100,
		RequestCount: 12,
		TenantBudget: 18,
		QueueRetries: 1,
		Bursty:       true,
	}, variants, 5); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E1_CAMPAIGN_RESULTS.md"),
		"E1 Campaign Results",
		[]string{
			"Generated from 5 deterministic seeded runs of the budget-delay scenario.",
		},
	)
}

func runE2Campaign() error {
	variants := defaultVariants()
	if err := runCampaign("E2", tracegateway.ScenarioConfig{
		Kind:               tracegateway.ScenarioMultitenant,
		Seed:               200,
		RequestCount:       14,
		TenantBudget:       14,
		SecondTenantBudget: 10,
		QueueRetries:       1,
		Bursty:             true,
	}, variants, 5); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E2_CAMPAIGN_RESULTS.md"),
		"E2 Campaign Results",
		[]string{
			"Generated from 5 deterministic seeded runs of the multitenant scenario.",
		},
	)
}

func runVariants(experimentID string, requests []tracegateway.TraceRequest, tenants map[string]ledger.TenantState) error {
	variants := defaultVariants()

	for _, variant := range variants {
		result, err := tracegateway.Replay(requests, tenants, tracegateway.ReplayConfig{
			Policy: admission.Policy{
				StrictMode:          true,
				AllowQueue:          true,
				RequireFreshSignals: true,
			},
			QueueRetryDelay:   1,
			ReservationConfig: variant.cfg,
		})
		if err != nil {
			return err
		}
		out := experimentOutput{
			ExperimentID: experimentID,
			Variant:      variant.name,
			Config:       variant.cfg,
			Metrics:      tracegateway.Summarize(result),
			Events:       result.Events,
		}
		if err := writeJSON(filepath.Join("experiments", "raw", experimentID+"_"+variant.name+".json"), out); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join("experiments", "processed", experimentID+"_"+variant.name+"_summary.json"), out.Metrics); err != nil {
			return err
		}
	}
	return nil
}

func defaultVariants() []struct {
	name string
	cfg  predictor.ReservationConfig
} {
	return []struct {
		name string
		cfg  predictor.ReservationConfig
	}{
		{name: "quantile", cfg: predictor.ReservationConfig{Mode: predictor.ModeQuantile, Quantile: 0.75}},
		{name: "mean_std", cfg: predictor.ReservationConfig{Mode: predictor.ModeMeanStd, ZScore: 1.0}},
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
