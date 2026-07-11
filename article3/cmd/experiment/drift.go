package main

import (
	"path/filepath"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
	tracegateway "github.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway"
)

type driftExperimentOutput struct {
	ExperimentID string                `json:"experiment_id"`
	Variant      string                `json:"variant"`
	Drift        predictor.DriftReport `json:"drift"`
	Metrics      tracegateway.Metrics  `json:"metrics"`
	Events       []tracegateway.Event  `json:"events"`
}

func runE3() error {
	reference := []int{8, 9, 10, 11, 10}
	recent := []int{18, 19, 20, 21, 22}
	drift := predictor.DetectMeanRatioDrift(reference, recent, 1.8)

	requests := []tracegateway.TraceRequest{
		{
			RequestID:       "drift-01",
			TenantID:        "team-a",
			Application:     "assistant-risky",
			ArrivalStep:     0,
			SettlementDelay: 1,
			ActualCost:      4.5,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model:              "premium-drifted",
						UtilityScore:       0.96,
						ReservationCost:    7.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
						Drifted:            drift.Drifted,
					},
				},
				{
					Base: admission.Candidate{
						Model:              "economy-safe",
						UtilityScore:       0.73,
						ReservationCost:    5.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
						Drifted:            false,
					},
				},
			},
		},
		{
			RequestID:       "drift-02",
			TenantID:        "team-a",
			Application:     "assistant-risky",
			ArrivalStep:     1,
			SettlementDelay: 1,
			ActualCost:      5.0,
			Candidates: []tracegateway.CandidateSpec{
				{
					Base: admission.Candidate{
						Model:              "premium-drifted-only",
						UtilityScore:       0.92,
						ReservationCost:    6.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
						Drifted:            drift.Drifted,
					},
				},
			},
		},
	}

	tenants := map[string]ledger.TenantState{
		"team-a": {TenantID: "team-a", Budget: 12},
	}

	result, err := tracegateway.Replay(requests, tenants, tracegateway.ReplayConfig{
		Policy: admission.Policy{
			StrictMode:          true,
			AllowQueue:          true,
			RequireFreshSignals: true,
			BlockOnDrift:        true,
		},
	})
	if err != nil {
		return err
	}

	output := driftExperimentOutput{
		ExperimentID: "E3",
		Variant:      "drift_block_and_fallback",
		Drift:        drift,
		Metrics:      tracegateway.Summarize(result),
		Events:       result.Events,
	}
	if err := writeJSON(filepath.Join("experiments", "raw", "E3_drift.json"), output); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join("experiments", "processed", "E3_drift_summary.json"), output); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E3_DRIFT_RESULTS.md"),
		"E3 Drift Results",
		[]string{
			"Drift was injected by increasing the recent output-length mean above the configured threshold.",
			metricsSection("E3 drift_block_and_fallback", output.Metrics),
		},
	)
}
