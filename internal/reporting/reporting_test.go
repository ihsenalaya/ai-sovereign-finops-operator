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

package reporting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
)

func sampleData() Data {
	samples := []collectors.UsageSample{
		{Namespace: "rh", Team: "rh", Provider: "az", Model: "gpt-4o",
			Requests: 100, InputTokens: 2_000_000, OutputTokens: 1_000_000},
	}
	prices := costengine.PriceBook{"gpt-4o": {Currency: "EUR", InputPerMillion: 5, OutputPerMillion: 15}}
	return Data{
		Name: "monthly-rh", Namespace: "rh", Period: "monthly",
		GeneratedAt: time.Unix(1_700_000_000, 0),
		Collector:   "fake",
		Breakdown:   costengine.Compute(samples, prices),
		Sovereignty: []aiopsv1alpha1.SovereigntyFinding{{Severity: "critical", Message: "US provider in use"}},
		Recommends:  []aiopsv1alpha1.Recommendation{{Type: "cost-optimization", Message: "route to cheaper model"}},
	}
}

func TestRenderMarkdown(t *testing.T) {
	md := RenderMarkdown(sampleData())
	for _, want := range []string{
		"# AI FinOps Report — monthly-rh",
		"## Executive summary",
		"Cost by model",
		"gpt-4o",
		"## Sovereignty findings",
		"CRITICAL",
		"## Recommendations",
		"## Limits & assumptions",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	b, err := RenderJSON(sampleData())
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["totalCost"].(float64) != 25 {
		t.Errorf("totalCost = %v, want 25", got["totalCost"])
	}
	if got["currency"].(string) != "EUR" {
		t.Errorf("currency = %v, want EUR", got["currency"])
	}
}
