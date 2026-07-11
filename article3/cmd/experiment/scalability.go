package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
	tracegateway "github.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway"
)

type scalabilityRow struct {
	Scenario        string               `json:"scenario"`
	RequestCount    int                  `json:"request_count"`
	DurationMillis  float64              `json:"duration_millis"`
	RequestsPerSec  float64              `json:"requests_per_sec"`
	Metrics         tracegateway.Metrics `json:"metrics"`
}

type scalabilityOutput struct {
	ExperimentID string           `json:"experiment_id"`
	Rows         []scalabilityRow `json:"rows"`
}

func runE5() error {
	scenarios := []tracegateway.ScenarioConfig{
		{
			Kind:         tracegateway.ScenarioBudgetDelay,
			Seed:         700,
			RequestCount: 50,
			TenantBudget: 90,
			QueueRetries: 1,
			Bursty:       true,
		},
		{
			Kind:         tracegateway.ScenarioBudgetDelay,
			Seed:         701,
			RequestCount: 250,
			TenantBudget: 420,
			QueueRetries: 1,
			Bursty:       true,
		},
		{
			Kind:               tracegateway.ScenarioMultitenant,
			Seed:               702,
			RequestCount:       500,
			TenantBudget:       450,
			SecondTenantBudget: 350,
			QueueRetries:       1,
			Bursty:             true,
		},
		{
			Kind:               tracegateway.ScenarioMultitenant,
			Seed:               703,
			RequestCount:       1000,
			TenantBudget:       900,
			SecondTenantBudget: 700,
			QueueRetries:       1,
			Bursty:             true,
		},
	}

	var rows []scalabilityRow
	for _, scenario := range scenarios {
		requests, tenantCfgs, err := tracegateway.BuildScenario(scenario)
		if err != nil {
			return err
		}
		tenants := make(map[string]ledger.TenantState, len(tenantCfgs))
		for k, v := range tenantCfgs {
			tenants[k] = ledger.TenantState{TenantID: v.TenantID, Budget: v.Budget}
		}

		start := time.Now()
		result, err := tracegateway.Replay(requests, tenants, tracegateway.ReplayConfig{
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
			return err
		}
		elapsed := time.Since(start)
		durationMillis := float64(elapsed.Microseconds()) / 1000.0
		rps := 0.0
		if elapsed > 0 {
			rps = float64(len(requests)) / elapsed.Seconds()
		}

		rows = append(rows, scalabilityRow{
			Scenario:       string(scenario.Kind),
			RequestCount:   len(requests),
			DurationMillis: durationMillis,
			RequestsPerSec: rps,
			Metrics:        tracegateway.Summarize(result),
		})
	}

	output := scalabilityOutput{
		ExperimentID: "E5",
		Rows:         rows,
	}
	if err := writeJSON(filepath.Join("experiments", "raw", "E5_scalability.json"), output); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join("experiments", "processed", "E5_scalability_summary.json"), output); err != nil {
		return err
	}

	highlights := make([]string, 0, len(rows))
	for _, row := range rows {
		highlights = append(highlights, fmt.Sprintf(
			"Scenario `%s` with `%d` requests completed in `%.3f ms` at `%.2f req/s`.",
			row.Scenario, row.RequestCount, row.DurationMillis, row.RequestsPerSec,
		))
	}
	highlights = append(highlights, "```json\n"+prettyJSON(output)+"\n```")
	return writeMarkdownReport(
		filepath.Join("reports", "E5_SCALABILITY_RESULTS.md"),
		"E5 Scalability Results",
		highlights,
	)
}
