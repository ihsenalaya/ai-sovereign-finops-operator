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

type campaignSummary struct {
	ExperimentID string                          `json:"experiment_id"`
	Variant      string                          `json:"variant"`
	Runs         int                             `json:"runs"`
	MeanMetrics  tracegateway.Metrics            `json:"mean_metrics"`
	RunMetrics   []tracegateway.Metrics          `json:"run_metrics"`
	Config       predictor.ReservationConfig     `json:"config"`
}

func runCampaign(experimentID string, scenario tracegateway.ScenarioConfig, variants []struct {
	name string
	cfg  predictor.ReservationConfig
}, runs int) error {
	for _, variant := range variants {
		summary := campaignSummary{
			ExperimentID: experimentID,
			Variant:      variant.name,
			Runs:         runs,
			Config:       variant.cfg,
		}
		for i := 0; i < runs; i++ {
			scenario.Seed += int64(i)
			requests, tenantCfgs, err := tracegateway.BuildScenario(scenario)
			if err != nil {
				return err
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
				ReservationConfig: variant.cfg,
			})
			if err != nil {
				return err
			}
			metrics := tracegateway.Summarize(result)
			summary.RunMetrics = append(summary.RunMetrics, metrics)
		}
		summary.MeanMetrics = averageMetrics(summary.RunMetrics)
		if err := writeJSON(filepath.Join("experiments", "processed", experimentID+"_"+variant.name+"_campaign.json"), summary); err != nil {
			return err
		}
	}
	return nil
}

func averageMetrics(in []tracegateway.Metrics) tracegateway.Metrics {
	if len(in) == 0 {
		return tracegateway.Metrics{}
	}
	var out tracegateway.Metrics
	out.TenantSettledTotal = make(map[string]float64)
	for _, m := range in {
		out.AdmittedCount += m.AdmittedCount
		out.QueuedCount += m.QueuedCount
		out.RejectedCount += m.RejectedCount
		out.AbstainedCount += m.AbstainedCount
		out.SettledCount += m.SettledCount
		out.ReservedTotal += m.ReservedTotal
		out.SettledTotal += m.SettledTotal
		out.SlackTotal += m.SlackTotal
		out.OvershootTotal += m.OvershootTotal
		for tenant, value := range m.TenantSettledTotal {
			out.TenantSettledTotal[tenant] += value
		}
	}
	div := float64(len(in))
	out.AdmittedCount /= len(in)
	out.QueuedCount /= len(in)
	out.RejectedCount /= len(in)
	out.AbstainedCount /= len(in)
	out.SettledCount /= len(in)
	out.ReservedTotal /= div
	out.SettledTotal /= div
	out.SlackTotal /= div
	out.OvershootTotal /= div
	for tenant, value := range out.TenantSettledTotal {
		out.TenantSettledTotal[tenant] = value / div
	}
	return out
}

func writeMarkdownReport(path string, title string, sections []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := "# " + title + "\n\n"
	for _, section := range sections {
		content += section + "\n\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func prettyJSON(v any) string {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return string(raw)
}

func metricsSection(name string, metrics tracegateway.Metrics) string {
	return fmt.Sprintf("## %s\n\n```json\n%s\n```", name, prettyJSON(metrics))
}
