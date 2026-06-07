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

// Package reporting renders an AIFinOpsReport into human- and machine-readable
// formats (Markdown and JSON). It is pure: given a Data snapshot it returns
// deterministic output, so it is easy to unit-test and to embed in a ConfigMap.
package reporting

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
)

// Data is the snapshot a report is rendered from.
type Data struct {
	Name        string
	Namespace   string
	Period      string
	GeneratedAt time.Time
	Collector   string
	Breakdown   costengine.Breakdown
	Sovereignty []aiopsv1alpha1.SovereigntyFinding
	Recommends  []aiopsv1alpha1.Recommendation
}

// jsonReport is the curated JSON shape (stable contract for downstream tooling).
type jsonReport struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace"`
	Period            string                 `json:"period"`
	GeneratedAt       string                 `json:"generatedAt"`
	Currency          string                 `json:"currency"`
	TotalCost         float64                `json:"totalCost"`
	TotalInputTokens  int64                  `json:"totalInputTokens"`
	TotalOutputTokens int64                  `json:"totalOutputTokens"`
	AvgCostPerRequest float64                `json:"avgCostPerRequest"`
	CostPerToken      float64                `json:"costPerToken"`
	ByModel           []lineJSON             `json:"byModel"`
	ByProvider        []lineJSON             `json:"byProvider"`
	ByTeam            []lineJSON             `json:"byTeam"`
	Sovereignty       []findingJSON          `json:"sovereigntyFindings"`
	Recommendations   []recommendationJSON   `json:"recommendations"`
	Assumptions       []string               `json:"assumptions"`
	Meta              map[string]interface{} `json:"meta,omitempty"`
}

type lineJSON struct {
	Key          string  `json:"key"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	Cost         float64 `json:"cost"`
}

type findingJSON struct {
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Namespace   string `json:"namespace,omitempty"`
	Application string `json:"application,omitempty"`
	Model       string `json:"model,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Zone        string `json:"zone,omitempty"`
	Requests    int64  `json:"requests,omitempty"`
}

type recommendationJSON struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Assumptions documents the MVP limitations surfaced in every report.
func Assumptions() []string {
	return []string{
		"Usage is sourced from the configured telemetry collector; the fake collector returns demo data.",
		"Costs use each model's AIProvider price per million tokens (the demo uses real published list prices, EUR = USD x 0.92); models without an AIProvider are reported as unpriced.",
		"Mixed currencies are not converted; the dominant currency is reported as-is.",
		"This report supports audit preparation; it is not a legal compliance attestation.",
		"The MVP is reportOnly and never blocks or modifies gateway traffic.",
	}
}

func toLines(items []costengine.LineItem) []lineJSON {
	out := make([]lineJSON, 0, len(items))
	for _, li := range items {
		out = append(out, lineJSON{
			Key: li.Key, Requests: li.Requests,
			InputTokens: li.InputTokens, OutputTokens: li.OutputTokens, Cost: li.CostTotal,
		})
	}
	return out
}

// RenderJSON renders the report as stable JSON.
func RenderJSON(d Data) ([]byte, error) {
	b := d.Breakdown
	r := jsonReport{
		Name: d.Name, Namespace: d.Namespace, Period: d.Period,
		GeneratedAt:       d.GeneratedAt.UTC().Format(time.RFC3339),
		Currency:          b.Currency,
		TotalCost:         b.Total.CostTotal,
		TotalInputTokens:  b.Total.InputTokens,
		TotalOutputTokens: b.Total.OutputTokens,
		AvgCostPerRequest: b.AvgCostPerRequest(),
		CostPerToken:      b.CostPerToken(),
		ByModel:           toLines(costengine.TopByCost(b.ByModel, 0)),
		ByProvider:        toLines(costengine.TopByCost(b.ByProvider, 0)),
		ByTeam:            toLines(costengine.TopByCost(b.ByTeam, 0)),
		Assumptions:       Assumptions(),
	}
	for _, f := range d.Sovereignty {
		r.Sovereignty = append(r.Sovereignty, findingJSON{
			Severity: string(f.Severity), Message: f.Message,
			Namespace: f.Namespace, Application: f.Application, Model: f.Model,
			Provider: f.Provider, Zone: f.Zone, Requests: f.Requests,
		})
	}
	for _, rec := range d.Recommends {
		r.Recommendations = append(r.Recommendations, recommendationJSON{Type: rec.Type, Message: rec.Message})
	}
	return json.MarshalIndent(r, "", "  ")
}

// RenderMarkdown renders a readable executive report.
func RenderMarkdown(d Data) string {
	b := d.Breakdown
	cur := b.Currency
	if cur == "" {
		cur = "EUR"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# AI FinOps Report — %s\n\n", d.Name)
	fmt.Fprintf(&sb, "- **Namespace:** %s\n- **Period:** %s\n- **Generated:** %s\n- **Collector:** %s\n\n",
		nonEmpty(d.Namespace, "(all)"), d.Period, d.GeneratedAt.UTC().Format(time.RFC3339), d.Collector)

	sb.WriteString("## Executive summary\n\n")
	fmt.Fprintf(&sb, "- **Total cost:** %.2f %s\n", b.Total.CostTotal, cur)
	fmt.Fprintf(&sb, "- **Requests:** %d  ·  **Input tokens:** %d  ·  **Output tokens:** %d\n",
		b.Total.Requests, b.Total.InputTokens, b.Total.OutputTokens)
	fmt.Fprintf(&sb, "- **Avg cost / request:** %.4f %s  ·  **Cost / token:** %.6f %s\n\n",
		b.AvgCostPerRequest(), cur, b.CostPerToken(), cur)

	writeTable(&sb, "Cost by model", costengine.TopByCost(b.ByModel, 0), cur)
	writeTable(&sb, "Cost by provider", costengine.TopByCost(b.ByProvider, 0), cur)
	writeTable(&sb, "Consumption by team", costengine.TopByCost(b.ByTeam, 0), cur)

	sb.WriteString("## Sovereignty findings\n\n")
	if len(d.Sovereignty) == 0 {
		sb.WriteString("_No findings._\n\n")
	} else {
		for _, f := range d.Sovereignty {
			suffix := ""
			if f.Requests > 0 {
				suffix = fmt.Sprintf(" _(%d request(s) affected)_", f.Requests)
			}
			fmt.Fprintf(&sb, "- **%s** — %s%s\n", strings.ToUpper(string(f.Severity)), f.Message, suffix)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Recommendations\n\n")
	if len(d.Recommends) == 0 {
		sb.WriteString("_No recommendations._\n\n")
	} else {
		for _, rec := range d.Recommends {
			fmt.Fprintf(&sb, "- _(%s)_ %s\n", rec.Type, rec.Message)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Limits & assumptions\n\n")
	for _, a := range Assumptions() {
		fmt.Fprintf(&sb, "- %s\n", a)
	}
	return sb.String()
}

func writeTable(sb *strings.Builder, title string, items []costengine.LineItem, cur string) {
	fmt.Fprintf(sb, "## %s\n\n", title)
	if len(items) == 0 {
		sb.WriteString("_No data._\n\n")
		return
	}
	sb.WriteString("| Key | Requests | Input tokens | Output tokens | Cost (" + cur + ") |\n")
	sb.WriteString("|-----|---------:|-------------:|--------------:|---------:|\n")
	for _, li := range items {
		if li.Key == "" {
			continue
		}
		fmt.Fprintf(sb, "| %s | %d | %d | %d | %.2f |\n",
			li.Key, li.Requests, li.InputTokens, li.OutputTokens, li.CostTotal)
	}
	sb.WriteString("\n")
}

func nonEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
