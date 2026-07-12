package govar

import (
	"testing"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildCandidatesFiltersByGovernanceAndTarget(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	ready := []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}
	models := []aiopsv1alpha1.AIModel{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "gpt-fr", Generation: 1, ResourceVersion: "m1", Annotations: map[string]string{AnnotationRoutable: "true", AnnotationOutputCapVerified: "true"}},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef:          "azure-fr",
				ModelName:            "gpt-4.1-mini",
				ContextWindow:        128000,
				QualityTier:          aiopsv1alpha1.TierHigh,
				CostTier:             aiopsv1alpha1.TierMedium,
				SensitiveDataAllowed: true,
				ServesNamespace:      "finance",
				ServesApplication:    "copilot",
			},
			Status: aiopsv1alpha1.AIModelStatus{ObservedGeneration: 1, LastQualityScore: 0.9, LastEvaluatedAt: &now, Conditions: ready},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "gpt-us", Generation: 1, ResourceVersion: "m2", Annotations: map[string]string{AnnotationRoutable: "true", AnnotationOutputCapVerified: "true"}},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef:          "azure-us",
				ModelName:            "gpt-4.1-mini",
				ContextWindow:        128000,
				SensitiveDataAllowed: false,
				ServesNamespace:      "finance",
				ServesApplication:    "copilot",
			},
			Status: aiopsv1alpha1.AIModelStatus{ObservedGeneration: 1, LastQualityScore: 0.9, LastEvaluatedAt: &now, Conditions: ready},
		},
	}
	providers := map[string]aiopsv1alpha1.AIProvider{
		"azure-fr": {
			ObjectMeta: metav1.ObjectMeta{Name: "azure-fr", Generation: 1, ResourceVersion: "p1", Annotations: map[string]string{AnnotationPricingVersion: "prices-v1", AnnotationPricingObservedAt: time.Now().UTC().Format(time.RFC3339)}},
			Spec: aiopsv1alpha1.AIProviderSpec{
				Type:          "azure-openai",
				Region:        "francecentral",
				DataResidency: "eu",
				Managed:       true,
				Pricing: aiopsv1alpha1.ProviderPricing{
					Currency:                   "EUR",
					InputTokenPricePerMillion:  resource.MustParse("0.4"),
					OutputTokenPricePerMillion: resource.MustParse("1.6"),
				},
				Compliance: aiopsv1alpha1.ProviderCompliance{
					AllowedForSensitiveData: true,
				},
			},
			Status: aiopsv1alpha1.AIProviderStatus{ObservedGeneration: 1, Conditions: ready},
		},
		"azure-us": {
			ObjectMeta: metav1.ObjectMeta{Name: "azure-us", Generation: 1, ResourceVersion: "p2", Annotations: map[string]string{AnnotationPricingVersion: "prices-v1", AnnotationPricingObservedAt: time.Now().UTC().Format(time.RFC3339)}},
			Spec: aiopsv1alpha1.AIProviderSpec{
				Type:          "azure-openai",
				Region:        "eastus",
				DataResidency: "us",
				Managed:       true,
				Pricing: aiopsv1alpha1.ProviderPricing{
					Currency:                   "EUR",
					InputTokenPricePerMillion:  resource.MustParse("0.2"),
					OutputTokenPricePerMillion: resource.MustParse("0.8"),
				},
			},
			Status: aiopsv1alpha1.AIProviderStatus{ObservedGeneration: 1, Conditions: ready},
		},
	}

	got := BuildCandidates(RequestContext{
		Namespace:     "finance",
		Application:   "copilot",
		SensitiveData: true,
		AllowedZones:  []string{"eu"},
	}, models, providers)

	var feasible []Candidate
	for _, candidate := range got {
		if candidate.Feasible {
			feasible = append(feasible, candidate)
		}
	}
	if len(feasible) != 1 {
		t.Fatalf("expected 1 feasible candidate, got %+v", got)
	}
	if feasible[0].ProviderRef != "azure-fr" {
		t.Fatalf("expected azure-fr candidate, got %s", feasible[0].ProviderRef)
	}
	if feasible[0].InputPriceMicrosPerMillion != 400_000 || feasible[0].OutputPriceMicrosPerMillion != 1_600_000 {
		t.Fatalf("unexpected pricing snapshot: %+v", feasible[0])
	}
}

func TestBuildPolicySnapshotCarriesBudgetAndRoutingGuardrails(t *testing.T) {
	budget := aiopsv1alpha1.AIBudgetPolicy{
		Spec: aiopsv1alpha1.AIBudgetPolicySpec{
			Target:           aiopsv1alpha1.BudgetTarget{Namespace: "finance", Team: "treasury"},
			BudgetEUR:        resource.MustParse("125.5"),
			FallbackModelRef: "cheap-model",
			EnforcementMode:  aiopsv1alpha1.EnforcementMode("enforce"),
			FallbackOnPhase:  aiopsv1alpha1.BudgetFallbackOnCritical,
		},
	}
	routing := aiopsv1alpha1.AIRoutingPolicy{
		Spec: aiopsv1alpha1.AIRoutingPolicySpec{
			Objective: "cost",
			Guardrails: aiopsv1alpha1.AIRoutingPolicyGuardrails{
				MinQualityScore:              0.82,
				MaxLatencyMillis:             1200,
				RequireSovereigntyCompliance: true,
			},
		},
	}

	got := BuildPolicySnapshot(budget, routing)
	if got.BudgetMicros != 125_500_000 {
		t.Fatalf("expected exact integer budget snapshot, got %v", got.BudgetMicros)
	}
	if got.Objective != "cost" || got.MinQualityScore != 0.82 || got.MaxLatencyMillis != 1200 {
		t.Fatalf("unexpected policy snapshot: %+v", got)
	}
	if got.BudgetScope.Namespace != "finance" || got.BudgetScope.Team != "treasury" {
		t.Fatalf("unexpected budget scope: %+v", got.BudgetScope)
	}
}
