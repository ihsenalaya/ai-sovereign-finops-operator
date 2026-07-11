package govar

import (
	"slices"
	"strings"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

type RequestContext struct {
	Namespace     string
	Team          string
	Application   string
	SensitiveData bool
	AllowedZones  []string
}

type Candidate struct {
	ModelRef              string
	ModelName             string
	ProviderRef           string
	ProviderType          string
	Region                string
	DataResidency         string
	Managed               bool
	InputPricePerMillion  float64
	OutputPricePerMillion float64
	QualityTier           aiopsv1alpha1.Tier
	CostTier              aiopsv1alpha1.Tier
	SensitiveDataAllowed  bool
}

type PolicySnapshot struct {
	BudgetScope                  aiopsv1alpha1.BudgetTarget
	BudgetEUR                    float64
	FallbackModelRef             string
	EnforcementMode              aiopsv1alpha1.EnforcementMode
	FallbackOnPhase              aiopsv1alpha1.BudgetFallbackPhase
	Objective                    string
	MinQualityScore              float64
	MaxLatencyMillis             int32
	RequireSovereigntyCompliance bool
}

func BuildPolicySnapshot(budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) PolicySnapshot {
	return PolicySnapshot{
		BudgetScope:                  budget.Spec.Target,
		BudgetEUR:                    budget.Spec.BudgetEUR.AsApproximateFloat64(),
		FallbackModelRef:             budget.Spec.FallbackModelRef,
		EnforcementMode:              budget.Spec.EnforcementMode,
		FallbackOnPhase:              budget.Spec.FallbackOnPhase,
		Objective:                    routing.Spec.Objective,
		MinQualityScore:              routing.Spec.Guardrails.MinQualityScore,
		MaxLatencyMillis:             routing.Spec.Guardrails.MaxLatencyMillis,
		RequireSovereigntyCompliance: routing.Spec.Guardrails.RequireSovereigntyCompliance,
	}
}

func BuildCandidates(req RequestContext, models []aiopsv1alpha1.AIModel, providers map[string]aiopsv1alpha1.AIProvider) []Candidate {
	var out []Candidate
	for i := range models {
		model := models[i]
		provider, ok := providers[model.Spec.ProviderRef]
		if !ok {
			continue
		}
		if !matchesTarget(req, model) {
			continue
		}
		if req.SensitiveData && !model.Spec.SensitiveDataAllowed && !provider.Spec.Compliance.AllowedForSensitiveData {
			continue
		}
		if len(req.AllowedZones) > 0 && !zoneAllowed(req.AllowedZones, provider.Spec.DataResidency, provider.Spec.Region) {
			continue
		}
		out = append(out, Candidate{
			ModelRef:              model.Name,
			ModelName:             model.Spec.ModelName,
			ProviderRef:           provider.Name,
			ProviderType:          provider.Spec.Type,
			Region:                provider.Spec.Region,
			DataResidency:         provider.Spec.DataResidency,
			Managed:               provider.Spec.Managed,
			InputPricePerMillion:  provider.Spec.Pricing.InputTokenPricePerMillion.AsApproximateFloat64(),
			OutputPricePerMillion: provider.Spec.Pricing.OutputTokenPricePerMillion.AsApproximateFloat64(),
			QualityTier:           model.Spec.QualityTier,
			CostTier:              model.Spec.CostTier,
			SensitiveDataAllowed:  model.Spec.SensitiveDataAllowed || provider.Spec.Compliance.AllowedForSensitiveData,
		})
	}
	return out
}

func matchesTarget(req RequestContext, model aiopsv1alpha1.AIModel) bool {
	if model.Spec.ServesNamespace != "" && model.Spec.ServesNamespace != req.Namespace {
		return false
	}
	if model.Spec.ServesTeam != "" && model.Spec.ServesTeam != req.Team {
		return false
	}
	if model.Spec.ServesApplication != "" && model.Spec.ServesApplication != req.Application {
		return false
	}
	return true
}

func zoneAllowed(allowed []string, residency, region string) bool {
	normalized := make([]string, 0, len(allowed))
	for _, zone := range allowed {
		normalized = append(normalized, normalizeZone(zone))
	}
	residency = normalizeZone(residency)
	region = normalizeZone(region)
	return slices.Contains(normalized, residency) || slices.Contains(normalized, region)
}

func normalizeZone(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
