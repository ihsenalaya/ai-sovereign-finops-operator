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

// Package fake provides a deterministic TelemetryCollector for tests and demos.
// It needs no gateway and returns a realistic, fixed usage profile.
package fake

import (
	"context"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
)

// Collector returns a fixed set of samples. If Samples is nil, a built-in demo
// profile (RH chatbot across two models/providers) is used.
type Collector struct {
	Samples []collectors.UsageSample
}

// New returns a fake collector using the built-in demo profile.
func New() *Collector { return &Collector{Samples: DemoSamples()} }

// DemoSamples is the canonical demo dataset, aligned with config/samples. The
// token volumes are illustrative monthly figures; cost is computed from the real
// per-model list prices declared on the AIProviders (see config/samples), so the
// demo reflects genuine token economics rather than invented prices:
//   - gpt-4o:        2.50 USD in / 10.00 USD out per 1M tokens (OpenAI / Azure)
//   - mistral-small: 0.10 USD in /  0.30 USD out per 1M tokens (Mistral)
// (EUR in the samples = USD list price x 0.92, June 2026.) Each sample's Provider
// matches the model's cataloged AIProvider so sovereignty zones resolve correctly.
func DemoSamples() []collectors.UsageSample {
	return []collectors.UsageSample{
		{
			Namespace: "rh", Application: "chatbot-rh", Team: "rh",
			Provider: "azure-openai-france", Model: "gpt-4o",
			Requests: 40000, InputTokens: 9_000_000, OutputTokens: 2_200_000,
			LatencyMillis: 850, Errors: 120,
		},
		{
			Namespace: "rh", Application: "chatbot-rh", Team: "rh",
			Provider: "mistral-france", Model: "mistral-small",
			Requests: 60000, InputTokens: 3_000_000, OutputTokens: 800_000,
			LatencyMillis: 420, Errors: 30,
		},
		{
			Namespace: "finance", Application: "risk-assistant", Team: "finance",
			Provider: "openai-us", Model: "gpt-4o",
			Requests: 12000, InputTokens: 4_000_000, OutputTokens: 1_000_000,
			LatencyMillis: 910, Errors: 80,
		},
	}
}

// Collect implements collectors.TelemetryCollector.
func (c *Collector) Collect(_ context.Context, _ time.Duration) ([]collectors.UsageSample, error) {
	out := make([]collectors.UsageSample, len(c.Samples))
	copy(out, c.Samples)
	return out, nil
}

// Name implements collectors.TelemetryCollector.
func (c *Collector) Name() string { return "fake" }
