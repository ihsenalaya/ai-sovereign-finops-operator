package govar

import (
	"slices"
	"strconv"
	"strings"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationRoutable          = "aiops.imperium.io/routable"
	AnnotationPricingVersion    = "aiops.imperium.io/pricing-version"
	AnnotationPricingObservedAt = "aiops.imperium.io/pricing-observed-at"
	AnnotationOutputCapVerified = "aiops.imperium.io/output-cap-verified"
	AnnotationLatencyMillis     = "aiops.imperium.io/latency-millis"
	AnnotationLatencyObservedAt = "aiops.imperium.io/latency-observed-at"
	observationFreshnessLimit   = 24 * time.Hour
)

type RequestContext struct {
	Namespace     string
	Team          string
	Application   string
	SensitiveData bool
	AllowedZones  []string
}

type Candidate struct {
	ModelRef                    string
	ModelName                   string
	ProviderRef                 string
	ProviderType                string
	Region                      string
	DataResidency               string
	Managed                     bool
	InputPriceMicrosPerMillion  int64
	OutputPriceMicrosPerMillion int64
	QualityTier                 aiopsv1alpha1.Tier
	CostTier                    aiopsv1alpha1.Tier
	SensitiveDataAllowed        bool
	PricingVersion              string
	SnapshotVersion             string
	ContextWindow               int64
	QualityScore                float64
	QualityObservedAt           time.Time
	VerifiedOutputCap           bool
	Feasible                    bool
	InfeasibleReason            ReasonCode
	LatencyMillis               int64
	LatencyObservedAt           time.Time
}

type PolicySnapshot struct {
	BudgetScope                  aiopsv1alpha1.BudgetTarget
	BudgetMicros                 MoneyMicros
	FallbackModelRef             string
	EnforcementMode              aiopsv1alpha1.EnforcementMode
	FallbackOnPhase              aiopsv1alpha1.BudgetFallbackPhase
	Objective                    string
	MinQualityScore              float64
	MaxLatencyMillis             int32
	RequireSovereigntyCompliance bool
}

func BuildPolicySnapshot(budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) PolicySnapshot {
	budgetMicros, _ := quantityToMicros(budget.Spec.BudgetEUR)
	return PolicySnapshot{
		BudgetScope:                  budget.Spec.Target,
		BudgetMicros:                 budgetMicros,
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
			out = append(out, Candidate{ModelRef: model.Name, InfeasibleReason: ReasonProviderUnavailable})
			continue
		}
		candidate := Candidate{ModelRef: model.Name, ModelName: model.Spec.ModelName, ProviderRef: provider.Name,
			ProviderType: provider.Spec.Type, Region: provider.Spec.Region, DataResidency: provider.Spec.DataResidency,
			Managed: provider.Spec.Managed, ContextWindow: int64(model.Spec.ContextWindow), QualityTier: model.Spec.QualityTier,
			CostTier: model.Spec.CostTier, QualityScore: model.Status.LastQualityScore,
			SnapshotVersion:   model.ResourceVersion + "|" + provider.ResourceVersion,
			PricingVersion:    strings.TrimSpace(provider.Annotations[AnnotationPricingVersion]),
			VerifiedOutputCap: strings.EqualFold(strings.TrimSpace(model.Annotations[AnnotationOutputCapVerified]), "true")}
		if !matchesTarget(req, model) {
			candidate.InfeasibleReason = ReasonWorkloadTargetMismatch
			out = append(out, candidate)
			continue
		}
		if model.Status.ObservedGeneration != model.Generation || !conditionTrue(model.Status.Conditions, aiopsv1alpha1.ConditionReady) {
			candidate.InfeasibleReason = ReasonModelNotReady
			out = append(out, candidate)
			continue
		}
		if provider.Status.ObservedGeneration != provider.Generation || !conditionTrue(provider.Status.Conditions, aiopsv1alpha1.ConditionReady) {
			candidate.InfeasibleReason = ReasonProviderUnavailable
			out = append(out, candidate)
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(model.Annotations[AnnotationRoutable]), "true") {
			candidate.InfeasibleReason = ReasonNotRoutable
			out = append(out, candidate)
			continue
		}
		if req.SensitiveData && !model.Spec.SensitiveDataAllowed && !provider.Spec.Compliance.AllowedForSensitiveData {
			candidate.InfeasibleReason = ReasonGovernanceInfeasible
			out = append(out, candidate)
			continue
		}
		if len(req.AllowedZones) > 0 && !zoneAllowed(req.AllowedZones, provider.Spec.DataResidency, provider.Spec.Region) {
			candidate.InfeasibleReason = ReasonGovernanceInfeasible
			out = append(out, candidate)
			continue
		}
		if model.Status.LastEvaluatedAt == nil || time.Since(model.Status.LastEvaluatedAt.Time) > observationFreshnessLimit {
			candidate.InfeasibleReason = ReasonQualityStale
			out = append(out, candidate)
			continue
		}
		candidate.QualityObservedAt = model.Status.LastEvaluatedAt.Time
		latency, latencyErr := strconv.ParseInt(strings.TrimSpace(model.Annotations[AnnotationLatencyMillis]), 10, 64)
		latencyAt, latencyTimeErr := time.Parse(time.RFC3339, strings.TrimSpace(model.Annotations[AnnotationLatencyObservedAt]))
		if latencyErr == nil && latencyTimeErr == nil && latency >= 0 && time.Since(latencyAt) <= observationFreshnessLimit && time.Until(latencyAt) <= time.Minute {
			candidate.LatencyMillis, candidate.LatencyObservedAt = latency, latencyAt
		}
		if candidate.ContextWindow <= 0 || candidate.PricingVersion == "" || !strings.EqualFold(strings.TrimSpace(provider.Spec.Pricing.Currency), "EUR") {
			candidate.InfeasibleReason = ReasonPricingIncomplete
			out = append(out, candidate)
			continue
		}
		pricingObservedAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(provider.Annotations[AnnotationPricingObservedAt]))
		if parseErr != nil || time.Since(pricingObservedAt) > observationFreshnessLimit || time.Until(pricingObservedAt) > time.Minute {
			candidate.InfeasibleReason = ReasonPricingStale
			out = append(out, candidate)
			continue
		}
		inputPrice, err := quantityToMicros(provider.Spec.Pricing.InputTokenPricePerMillion)
		if err != nil {
			candidate.InfeasibleReason = ReasonPricingIncomplete
			out = append(out, candidate)
			continue
		}
		outputPrice, err := quantityToMicros(provider.Spec.Pricing.OutputTokenPricePerMillion)
		if err != nil {
			candidate.InfeasibleReason = ReasonPricingIncomplete
			out = append(out, candidate)
			continue
		}
		candidate.InputPriceMicrosPerMillion = int64(inputPrice)
		candidate.OutputPriceMicrosPerMillion = int64(outputPrice)
		candidate.SensitiveDataAllowed = model.Spec.SensitiveDataAllowed || provider.Spec.Compliance.AllowedForSensitiveData
		candidate.Feasible = true
		out = append(out, candidate)
	}
	return out
}

func conditionTrue(conditions []metav1.Condition, conditionType string) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
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
