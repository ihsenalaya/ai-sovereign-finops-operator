/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics registers the ai_finops_* Prometheus metrics on the
// controller-runtime registry (exposed via the manager's /metrics endpoint).
//
// Values are derived aggregates set from the latest reconcile, so they are
// modelled as gauges even where the requested name carries a _total suffix
// (kept to match the product spec and to mirror gateway-side counters).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// RequestsTotal is the request volume attributed to a namespace.
	RequestsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_requests_total",
		Help: "LLM requests attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// InputTokensTotal is the input token volume per namespace.
	InputTokensTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_input_tokens_total",
		Help: "Input tokens attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// OutputTokensTotal is the output token volume per namespace.
	OutputTokensTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_output_tokens_total",
		Help: "Output tokens attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// CostEURTotal is the spend in EUR per namespace.
	CostEURTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_cost_eur_total",
		Help: "Spend in EUR actually observed for a namespace over the reporting window.",
	}, []string{"namespace"})

	// PotentialSavingsEUR is the total estimated saving from cost-saving
	// recommendations over the observation window.
	PotentialSavingsEUR = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ai_finops_potential_savings_eur",
		Help: "Total estimated saving from cost-saving recommendations over the observation window.",
	})

	// ProjectedMonthlyCostEUR is the run-rate forecast of monthly spend per namespace.
	ProjectedMonthlyCostEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_projected_monthly_cost_eur",
		Help: "Run-rate forecast of full-month spend in EUR per namespace (observed x 30.4/window-days).",
	}, []string{"namespace"})

	// BudgetUsagePercent is the percent of budget consumed per policy.
	BudgetUsagePercent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_budget_usage_percent",
		Help: "Percent of budget consumed for an AIBudgetPolicy.",
	}, []string{"namespace", "policy"})

	// SovereigntyFindings is the count of findings per policy, namespace,
	// application and severity (flow-aware attribution).
	SovereigntyFindings = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_sovereignty_findings_total",
		Help: "Sovereignty findings detected for an AISovereigntyPolicy, by namespace, application and severity.",
	}, []string{"namespace", "application", "policy", "severity"})

	// SovereigntyRequests is the number of requests affected by sovereignty
	// findings, by namespace, application and severity (the real volume at risk).
	SovereigntyRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_sovereignty_requests_total",
		Help: "Requests affected by sovereignty findings, by namespace, application and severity.",
	}, []string{"namespace", "application", "policy", "severity"})

	// BreakevenSavingsEUR is the estimated monthly savings of self-hosting.
	BreakevenSavingsEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_breakeven_savings_eur",
		Help: "Estimated monthly EUR savings from self-hosting for an AIBreakEvenAnalysis.",
	}, []string{"namespace", "analysis"})

	// Recommendations is the number of recommendations emitted, by type.
	Recommendations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_recommendations_total",
		Help: "Recommendations emitted by the operator, by type.",
	}, []string{"type"})
)

func init() {
	crmetrics.Registry.MustRegister(
		RequestsTotal,
		InputTokensTotal,
		OutputTokensTotal,
		CostEURTotal,
		PotentialSavingsEUR,
		ProjectedMonthlyCostEUR,
		BudgetUsagePercent,
		SovereigntyFindings,
		SovereigntyRequests,
		BreakevenSavingsEUR,
		Recommendations,
	)
}
