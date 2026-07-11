package main

import (
	"fmt"
	"path/filepath"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
	tracegateway "github.com/imperium/ai-sovereign-finops-operator/article3/src/trace_gateway"
)

type matrixRow struct {
	ExperimentID        string               `json:"experiment_id"`
	Variant             string               `json:"variant"`
	Scenario            string               `json:"scenario"`
	Seed                int64                `json:"seed"`
	PrimaryBudget       float64              `json:"primary_budget"`
	SecondaryBudget     float64              `json:"secondary_budget,omitempty"`
	RequestCount        int                  `json:"request_count"`
	Metrics             tracegateway.Metrics `json:"metrics"`
	ReservationConfig   predictor.ReservationConfig `json:"reservation_config"`
}

type matrixSummary struct {
	ExperimentID string      `json:"experiment_id"`
	Rows         []matrixRow `json:"rows"`
}

func runE1Matrix() error {
	var rows []matrixRow
	variants := defaultVariants()
	seeds := []int64{300, 301, 302}
	budgets := []float64{14, 16, 18}
	for _, seed := range seeds {
		for _, budget := range budgets {
			for _, variant := range variants {
				row, err := matrixRun("E1", tracegateway.ScenarioConfig{
					Kind:         tracegateway.ScenarioBudgetDelay,
					Seed:         seed,
					RequestCount: 12,
					TenantBudget: budget,
					QueueRetries: 1,
					Bursty:       true,
				}, variant.name, variant.cfg)
				if err != nil {
					return err
				}
				rows = append(rows, row)
			}
		}
	}
	summary := matrixSummary{ExperimentID: "E1", Rows: rows}
	if err := writeJSON(filepath.Join("experiments", "processed", "E1_matrix.json"), summary); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E1_MATRIX_RESULTS.md"),
		"E1 Matrix Results",
		[]string{
			matrixHighlights("E1", rows),
		},
	)
}

func runE2Matrix() error {
	var rows []matrixRow
	variants := defaultVariants()
	seeds := []int64{400, 401, 402}
	primaryBudgets := []float64{12, 14, 16}
	secondaryBudgets := []float64{8, 10}
	for _, seed := range seeds {
		for _, primary := range primaryBudgets {
			for _, secondary := range secondaryBudgets {
				for _, variant := range variants {
					row, err := matrixRun("E2", tracegateway.ScenarioConfig{
						Kind:               tracegateway.ScenarioMultitenant,
						Seed:               seed,
						RequestCount:       14,
						TenantBudget:       primary,
						SecondTenantBudget: secondary,
						QueueRetries:       1,
						Bursty:             true,
					}, variant.name, variant.cfg)
					if err != nil {
						return err
					}
					rows = append(rows, row)
				}
			}
		}
	}
	summary := matrixSummary{ExperimentID: "E2", Rows: rows}
	if err := writeJSON(filepath.Join("experiments", "processed", "E2_matrix.json"), summary); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E2_MATRIX_RESULTS.md"),
		"E2 Matrix Results",
		[]string{
			matrixHighlights("E2", rows),
		},
	)
}

func matrixRun(experimentID string, scenario tracegateway.ScenarioConfig, variantName string, cfg predictor.ReservationConfig) (matrixRow, error) {
	requests, tenantCfgs, err := tracegateway.BuildScenario(scenario)
	if err != nil {
		return matrixRow{}, err
	}
	tenants := make(map[string]ledger.TenantState, len(tenantCfgs))
	for k, v := range tenantCfgs {
		tenants[k] = ledger.TenantState{TenantID: v.TenantID, Budget: v.Budget}
	}
	result, err := tracegateway.Replay(requests, tenants, tracegateway.ReplayConfig{
		Policy: admission.Policy{
			StrictMode:          true,
			AllowQueue:          true,
			RequireFreshSignals: true,
		},
		QueueRetryDelay:   1,
		ReservationConfig: cfg,
	})
	if err != nil {
		return matrixRow{}, err
	}
	return matrixRow{
		ExperimentID:      experimentID,
		Variant:           variantName,
		Scenario:          string(scenario.Kind),
		Seed:              scenario.Seed,
		PrimaryBudget:     scenario.TenantBudget,
		SecondaryBudget:   scenario.SecondTenantBudget,
		RequestCount:      scenario.RequestCount,
		Metrics:           tracegateway.Summarize(result),
		ReservationConfig: cfg,
	}, nil
}

func matrixHighlights(experimentID string, rows []matrixRow) string {
	bestThroughputVariant := ""
	bestThroughput := -1
	bestSafetyVariant := ""
	bestSafetyOvershoot := 1e18
	for _, row := range rows {
		if row.Metrics.AdmittedCount > bestThroughput {
			bestThroughput = row.Metrics.AdmittedCount
			bestThroughputVariant = row.Variant
		}
		if row.Metrics.OvershootTotal < bestSafetyOvershoot {
			bestSafetyOvershoot = row.Metrics.OvershootTotal
			bestSafetyVariant = row.Variant
		}
	}
	return fmt.Sprintf(
		"Matrix `%s` generated %d rows. Best single-row throughput variant: `%s` with admitted count `%d`. Lowest overshoot row variant: `%s` with overshoot `%.2f`.",
		experimentID, len(rows), bestThroughputVariant, bestThroughput, bestSafetyVariant, bestSafetyOvershoot,
	)
}
