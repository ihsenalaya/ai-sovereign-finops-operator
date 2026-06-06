// Package catalog defines the model/provider catalog used by the experiments and
// bridges it to the operator's pure engines (costengine, sovereigntyengine).
//
// Real models are served by a live LLM client (OpenAI). The self-hosted entry is
// MODELED (no GPU in the experiment host): its cost is computed from declared
// parameters and its responses come from a deterministic local stub. Outputs are
// tagged accordingly so real and modeled data are never conflated.
package catalog

import (
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/sovereigntyengine"
)

// Tier ranks a model's role.
const (
	TierPremium    = "premium"
	TierMedium     = "medium"
	TierCheap      = "cheap"
	TierSelfHosted = "self-hosted"
)

// Model is a routable model entry.
type Model struct {
	ID               string  // catalog id (also used as the cost key)
	APIModel         string  // provider-side model id for real calls
	Provider         string  // provider name
	Zone             string  // data-residency zone (US, FR, ...)
	Managed          bool    // managed API (true) vs self-hosted (false)
	Real             bool    // true: served by a live LLM client; false: modeled stub
	Tier             string  // premium|medium|cheap|self-hosted
	InPerMillion     float64 // EUR per 1M input tokens
	OutPerMillion    float64 // EUR per 1M output tokens
	QualityPrior     float64 // 0..1 expected relative quality
	LatencyPriorMS   float64 // prior latency estimate (ms)
	SensitiveAllowed bool    // may process sensitive data
}

// Default returns the experiment catalog. OpenAI prices are approximate public
// EUR rates at time of writing (documented in paper/methodology.md). The
// self-hosted entry is modeled.
// OpenAIModels are the US managed OpenAI tiers (real).
func OpenAIModels() []Model {
	return []Model{
		{ID: "gpt-4o", APIModel: "gpt-4o", Provider: "openai-us", Zone: "US", Managed: true, Real: true,
			Tier: TierPremium, InPerMillion: 2.50, OutPerMillion: 10.00, QualityPrior: 1.00, LatencyPriorMS: 900, SensitiveAllowed: false},
		{ID: "gpt-4o-mini", APIModel: "gpt-4o-mini", Provider: "openai-us", Zone: "US", Managed: true, Real: true,
			Tier: TierMedium, InPerMillion: 0.15, OutPerMillion: 0.60, QualityPrior: 0.88, LatencyPriorMS: 600, SensitiveAllowed: false},
		{ID: "gpt-4.1-nano", APIModel: "gpt-4.1-nano", Provider: "openai-us", Zone: "US", Managed: true, Real: true,
			Tier: TierCheap, InPerMillion: 0.10, OutPerMillion: 0.40, QualityPrior: 0.80, LatencyPriorMS: 450, SensitiveAllowed: false},
	}
}

// MistralModels are EU-hosted Mistral models (real, second provider). EU zone +
// allowed-for-sensitive makes RQ4 sovereignty real (not modeled). APIModel values
// match Mistral La Plateforme / Azure AI Foundry serverless deployment names.
func MistralModels() []Model {
	return []Model{
		{ID: "mistral-large", APIModel: "mistral-large-latest", Provider: "mistral-eu", Zone: "FR", Managed: true, Real: true,
			Tier: TierPremium, InPerMillion: 2.00, OutPerMillion: 6.00, QualityPrior: 0.96, LatencyPriorMS: 850, SensitiveAllowed: true},
		{ID: "mistral-small", APIModel: "mistral-small-latest", Provider: "mistral-eu", Zone: "FR", Managed: true, Real: true,
			Tier: TierMedium, InPerMillion: 0.20, OutPerMillion: 0.60, QualityPrior: 0.86, LatencyPriorMS: 550, SensitiveAllowed: true},
	}
}

// SelfHostedModeled is the modeled EU self-hosted fallback (no GPU; cost modeled,
// response stubbed). Used for the RQ6 break-even prediction only.
func SelfHostedModeled() []Model {
	return []Model{
		{ID: "selfhosted-eu-llama", APIModel: "selfhosted-eu-llama", Provider: "onprem-fr", Zone: "FR", Managed: false, Real: false,
			Tier: TierSelfHosted, InPerMillion: 0.05, OutPerMillion: 0.05, QualityPrior: 0.74, LatencyPriorMS: 700, SensitiveAllowed: true},
	}
}

// Default is OpenAI + modeled self-hosted (single-provider baseline catalog).
func Default() []Model {
	return append(OpenAIModels(), SelfHostedModeled()...)
}

// ByID indexes models by ID.
func ByID(models []Model) map[string]Model {
	m := make(map[string]Model, len(models))
	for _, x := range models {
		m[x.ID] = x
	}
	return m
}

// PriceBook builds a costengine price book keyed by model ID.
func PriceBook(models []Model) costengine.PriceBook {
	pb := costengine.PriceBook{}
	for _, m := range models {
		pb[m.ID] = costengine.TokenPricing{Currency: "EUR", InputPerMillion: m.InPerMillion, OutputPerMillion: m.OutPerMillion}
	}
	return pb
}

// ProviderInfos builds sovereignty provider info from the distinct providers.
func ProviderInfos(models []Model) []sovereigntyengine.ProviderInfo {
	seen := map[string]bool{}
	var out []sovereigntyengine.ProviderInfo
	for _, m := range models {
		if seen[m.Provider] {
			continue
		}
		seen[m.Provider] = true
		out = append(out, sovereigntyengine.ProviderInfo{
			Name: m.Provider, Zone: m.Zone, Managed: m.Managed, AllowedForSensitiveData: m.SensitiveAllowed,
		})
	}
	return out
}
