package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type comparisonReport struct {
	ExperimentID string             `json:"experiment_id"`
	Comparisons  []comparisonRecord `json:"comparisons"`
}

type comparisonRecord struct {
	Artifact                string  `json:"artifact"`
	QuantileAdmitted        float64 `json:"quantile_admitted"`
	MeanStdAdmitted         float64 `json:"mean_std_admitted"`
	QuantileOvershoot       float64 `json:"quantile_overshoot"`
	MeanStdOvershoot        float64 `json:"mean_std_overshoot"`
	QuantileSlack           float64 `json:"quantile_slack"`
	MeanStdSlack            float64 `json:"mean_std_slack"`
	AdmittedDelta           float64 `json:"admitted_delta"`
	OvershootDelta          float64 `json:"overshoot_delta"`
	SlackDelta              float64 `json:"slack_delta"`
}

func compareExperiment(experimentID string) error {
	artifacts := []string{
		experimentID + "_quantile_summary.json",
		experimentID + "_mean_std_summary.json",
		experimentID + "_quantile_campaign.json",
		experimentID + "_mean_std_campaign.json",
	}
	report := comparisonReport{ExperimentID: experimentID}
	summaryRecord, err := comparePair(
		filepath.Join("experiments", "processed", experimentID+"_quantile_summary.json"),
		filepath.Join("experiments", "processed", experimentID+"_mean_std_summary.json"),
		"summary",
	)
	if err == nil {
		report.Comparisons = append(report.Comparisons, summaryRecord)
	}
	campaignRecord, err := comparePair(
		filepath.Join("experiments", "processed", experimentID+"_quantile_campaign.json"),
		filepath.Join("experiments", "processed", experimentID+"_mean_std_campaign.json"),
		"campaign_mean",
	)
	if err == nil {
		report.Comparisons = append(report.Comparisons, campaignRecord)
	}
	_ = artifacts
	if err := writeJSON(filepath.Join("experiments", "processed", experimentID+"_comparison.json"), report); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", experimentID+"_COMPARISON.md"),
		experimentID+" Comparison",
		comparisonSections(report),
	)
}

func comparePair(quantilePath string, meanStdPath string, label string) (comparisonRecord, error) {
	q, err := loadMetricsCarrier(quantilePath)
	if err != nil {
		return comparisonRecord{}, err
	}
	m, err := loadMetricsCarrier(meanStdPath)
	if err != nil {
		return comparisonRecord{}, err
	}
	return comparisonRecord{
		Artifact:          label,
		QuantileAdmitted:  float64(q.AdmittedCount),
		MeanStdAdmitted:   float64(m.AdmittedCount),
		QuantileOvershoot: q.OvershootTotal,
		MeanStdOvershoot:  m.OvershootTotal,
		QuantileSlack:     q.SlackTotal,
		MeanStdSlack:      m.SlackTotal,
		AdmittedDelta:     float64(q.AdmittedCount - m.AdmittedCount),
		OvershootDelta:    q.OvershootTotal - m.OvershootTotal,
		SlackDelta:        q.SlackTotal - m.SlackTotal,
	}, nil
}

func comparisonSections(report comparisonReport) []string {
	sections := []string{
		"Auto-generated comparison between `quantile` and `mean_std` variants.",
	}
	for _, c := range report.Comparisons {
		sections = append(sections, fmt.Sprintf(
			"## %s\n\n- admitted delta (`quantile - mean_std`): `%.2f`\n- overshoot delta (`quantile - mean_std`): `%.2f`\n- slack delta (`quantile - mean_std`): `%.2f`",
			c.Artifact, c.AdmittedDelta, c.OvershootDelta, c.SlackDelta,
		))
	}
	return sections
}

func loadMetricsCarrier(path string) (metricsCarrier, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return metricsCarrier{}, err
	}
	var campaign campaignSummary
	if err := json.Unmarshal(raw, &campaign); err == nil && campaign.ExperimentID != "" {
		return metricsCarrier{
			AdmittedCount:  campaign.MeanMetrics.AdmittedCount,
			OvershootTotal: campaign.MeanMetrics.OvershootTotal,
			SlackTotal:     campaign.MeanMetrics.SlackTotal,
		}, nil
	}
	var metrics metricsCarrier
	if err := json.Unmarshal(raw, &metrics); err != nil {
		return metricsCarrier{}, err
	}
	return metrics, nil
}

type metricsCarrier struct {
	AdmittedCount  int     `json:"admitted_count"`
	OvershootTotal float64 `json:"overshoot_total"`
	SlackTotal     float64 `json:"slack_total"`
}
