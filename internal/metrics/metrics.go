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
// Every value here is a derived aggregate over a reporting window, set from the
// latest reconcile — i.e. a gauge, not a monotonic counter. The names therefore
// avoid the _total suffix (Prometheus convention: _total == counter); using
// _total on a gauge silently breaks rate()/increase() and confuses tooling. The
// cumulative gateway-side counters keep _total in their own exporters; these
// operator-side snapshots do not.
package metrics

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// RequestsTotal is the request volume attributed to a namespace.
	RequestsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_requests",
		Help: "LLM requests attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// InputTokensTotal is the input token volume per namespace.
	InputTokensTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_input_tokens",
		Help: "Input tokens attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// OutputTokensTotal is the output token volume per namespace.
	OutputTokensTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_output_tokens",
		Help: "Output tokens attributed to a namespace over the reporting window.",
	}, []string{"namespace"})

	// CostEURTotal is the spend in EUR per namespace.
	CostEURTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_cost_eur",
		Help: "Spend in EUR actually observed for a namespace over the reporting window.",
	}, []string{"namespace"})

	// CostByZoneEUR is the spend in EUR per sovereignty zone (e.g. EU vs US),
	// resolved from each model's provider data residency. It surfaces how much
	// spend stays in a compliant zone versus leaves it.
	CostByZoneEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_cost_by_zone_eur",
		Help: "Spend in EUR per sovereignty zone (provider data-residency) over the reporting window.",
	}, []string{"zone"})

	// PotentialSavingsEUR is the total estimated saving from cost-saving
	// recommendations over the observation window.
	PotentialSavingsEUR = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ai_finops_potential_savings_eur",
		Help: "Total estimated saving from cost-saving recommendations over the observation window.",
	})

	// PotentialSavingsByAppEUR is the estimated cost-saving per namespace/app,
	// so a dashboard can show which workload each saving belongs to.
	PotentialSavingsByAppEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_potential_savings_by_app_eur",
		Help: "Estimated cost-saving in EUR per namespace/application over the observation window.",
	}, []string{"namespace", "application"})

	// CostSavingEUR is the estimated EUR saving of one concrete cost-saving model
	// swap, labelled with the workload and the current/recommended models — so a
	// dashboard can show the actual action ("gpt-4o → gpt-4o-mini") and its gain.
	CostSavingEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_cost_saving_eur",
		Help: "Estimated EUR saving of a cost-saving recommendation, by namespace/application and current/recommended model.",
	}, []string{"namespace", "application", "current_model", "recommended_model"})

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
		Name: "ai_finops_sovereignty_findings",
		Help: "Sovereignty findings detected for an AISovereigntyPolicy, by namespace, application and severity.",
	}, []string{"namespace", "application", "policy", "severity"})

	// SovereigntyRequests is the number of requests affected by sovereignty
	// findings, by namespace, application and severity (the real volume at risk).
	SovereigntyRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_sovereignty_requests",
		Help: "Requests affected by sovereignty findings, by namespace, application and severity.",
	}, []string{"namespace", "application", "policy", "severity"})

	// BreakevenSavingsEUR is the estimated monthly savings of self-hosting.
	BreakevenSavingsEUR = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_breakeven_savings_eur",
		Help: "Estimated monthly EUR savings from self-hosting for an AIBreakEvenAnalysis.",
	}, []string{"namespace", "analysis"})

	// Recommendations is the number of recommendations emitted, by type and (when
	// the recommendation targets a workload) namespace, application and severity.
	// The per-app labels let a dashboard list each actionable recommendation with
	// its owner instead of only a count per type.
	Recommendations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_recommendations",
		Help: "Recommendations emitted by the operator, by type, namespace, application and severity.",
	}, []string{"type", "namespace", "application", "severity"})

	// EnforcementActions is the enforcement decision taken for a policy violation,
	// by policy, namespace, application, mode (reportOnly/warn/enforce), action
	// (report/warn/block/reroute) and whether it was actually actuated end-to-end
	// (warn/report are; block/reroute await the gateway integration). It turns the
	// operator from an observer into a control plane: a dashboard/alert can see what
	// the operator decided to do about each non-compliant workload.
	EnforcementActions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ai_finops_enforcement_actions",
		Help: "Enforcement decisions by policy, namespace, application, mode, action and actuated.",
	}, []string{"policy", "namespace", "application", "mode", "action", "actuated"})
)

// all is every collector this package owns. init() registers them and Names()
// derives their metric names — so the set of exposed names has a single source of
// truth (used by the dashboard-consistency test to catch metric/dashboard drift).
var all = []prometheus.Collector{
	RequestsTotal,
	InputTokensTotal,
	OutputTokensTotal,
	CostEURTotal,
	CostByZoneEUR,
	PotentialSavingsEUR,
	PotentialSavingsByAppEUR,
	CostSavingEUR,
	ProjectedMonthlyCostEUR,
	BudgetUsagePercent,
	SovereigntyFindings,
	SovereigntyRequests,
	BreakevenSavingsEUR,
	Recommendations,
	EnforcementActions,
}

func init() {
	crmetrics.Registry.MustRegister(all...)
}

var fqNameRe = regexp.MustCompile(`fqName: "([^"]+)"`)

// Names returns the fully-qualified names of every metric this package registers.
// It Describe()s each collector and extracts the fqName, so it stays correct
// automatically when metrics are added, removed or renamed.
func Names() []string {
	var names []string
	for _, c := range all {
		ch := make(chan *prometheus.Desc, 4)
		c.Describe(ch)
		close(ch)
		for d := range ch {
			if m := fqNameRe.FindStringSubmatch(d.String()); m != nil {
				names = append(names, m[1])
			}
		}
	}
	return names
}
