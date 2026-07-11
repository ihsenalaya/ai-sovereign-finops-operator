package main

import (
	"fmt"
	"path/filepath"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
	tracegateway "github.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway"
)

type ablationVariant struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Config      predictor.ReservationConfig `json:"config"`
	Policy      admission.Policy           `json:"policy"`
}

type ablationRow struct {
	Scenario string               `json:"scenario"`
	Variant  ablationVariant      `json:"variant"`
	Metrics  tracegateway.Metrics `json:"metrics"`
}

type ablationOutput struct {
	ExperimentID string        `json:"experiment_id"`
	Rows         []ablationRow `json:"rows"`
}

func runE7() error {
	variants := []ablationVariant{
		{
			Name:        "quantile",
			Description: "Empirical quantile reservation with queue enabled.",
			Config:      predictor.ReservationConfig{Mode: predictor.ModeQuantile, Quantile: 0.75},
			Policy:      admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		},
		{
			Name:        "mean_std",
			Description: "Mean plus standard deviation reservation with queue enabled.",
			Config:      predictor.ReservationConfig{Mode: predictor.ModeMeanStd, ZScore: 1.0},
			Policy:      admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		},
		{
			Name:        "min_reservation",
			Description: "Aggressive low reservation ablation approximating no risk buffer.",
			Config:      predictor.ReservationConfig{Mode: predictor.ModeQuantile, Quantile: 0.0},
			Policy:      admission.Policy{StrictMode: true, AllowQueue: true, RequireFreshSignals: true},
		},
		{
			Name:        "no_queue",
			Description: "Queue disabled while keeping quantile reservation.",
			Config:      predictor.ReservationConfig{Mode: predictor.ModeQuantile, Quantile: 0.75},
			Policy:      admission.Policy{StrictMode: true, AllowQueue: false, RequireFreshSignals: true},
		},
	}

	scenarios := []tracegateway.ScenarioConfig{
		{
			Kind:         tracegateway.ScenarioBudgetDelay,
			Seed:         500,
			RequestCount: 12,
			TenantBudget: 18,
			QueueRetries: 1,
			Bursty:       true,
		},
		{
			Kind:               tracegateway.ScenarioMultitenant,
			Seed:               600,
			RequestCount:       14,
			TenantBudget:       14,
			SecondTenantBudget: 10,
			QueueRetries:       1,
			Bursty:             true,
		},
	}

	var rows []ablationRow
	for _, scenario := range scenarios {
		for _, variant := range variants {
			requests, tenantCfgs, err := tracegateway.BuildScenario(scenario)
			if err != nil {
				return err
			}
			tenants := make(map[string]ledger.TenantState, len(tenantCfgs))
			for k, v := range tenantCfgs {
				tenants[k] = ledger.TenantState{TenantID: v.TenantID, Budget: v.Budget}
			}
			result, err := tracegateway.Replay(requests, tenants, tracegateway.ReplayConfig{
				Policy:            variant.Policy,
				QueueRetryDelay:   1,
				ReservationConfig: variant.Config,
			})
			if err != nil {
				return err
			}
			rows = append(rows, ablationRow{
				Scenario: string(scenario.Kind),
				Variant:  variant,
				Metrics:  tracegateway.Summarize(result),
			})
		}
	}

	output := ablationOutput{
		ExperimentID: "E7",
		Rows:         rows,
	}
	if err := writeJSON(filepath.Join("experiments", "raw", "E7_ablation.json"), output); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join("experiments", "processed", "E7_ablation_summary.json"), output); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E7_ABLATION_RESULTS.md"),
		"E7 Ablation Results",
		[]string{
			fmt.Sprintf("Generated %d ablation rows over budget-delay and multitenant scenarios.", len(rows)),
			"```json\n" + prettyJSON(output) + "\n```",
		},
	)
}
