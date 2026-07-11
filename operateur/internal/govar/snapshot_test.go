package govar

import (
	"testing"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildCandidatesFiltersByGovernanceAndTarget(t *testing.T) {
	models := []aiopsv1alpha1.AIModel{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "gpt-fr"},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef:          "azure-fr",
				ModelName:            "gpt-4.1-mini",
				QualityTier:          aiopsv1alpha1.TierHigh,
				CostTier:             aiopsv1alpha1.TierMedium,
				SensitiveDataAllowed: true,
				ServesNamespace:      "finance",
				ServesApplication:    "copilot",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "gpt-us"},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef:          "azure-us",
				ModelName:            "gpt-4.1-mini",
				SensitiveDataAllowed: false,
				ServesNamespace:      "finance",
				ServesApplication:    "copilot",
			},
		},
	}
	providers := map[string]aiopsv1alpha1.AIProvider{
		"azure-fr": {
			ObjectMeta: metav1.ObjectMeta{Name: "azure-fr"},
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
		},
		"azure-us": {
			ObjectMeta: metav1.ObjectMeta{Name: "azure-us"},
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
		},
	}

	got := BuildCandidates(RequestContext{
		Namespace:     "finance",
		Application:   "copilot",
		SensitiveData: true,
		AllowedZones:  []string{"eu"},
	}, models, providers)

	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
	if got[0].ProviderRef != "azure-fr" {
		t.Fatalf("expected azure-fr candidate, got %s", got[0].ProviderRef)
	}
	if got[0].InputPricePerMillion != 0.4 || got[0].OutputPricePerMillion != 1.6 {
		t.Fatalf("unexpected pricing snapshot: %+v", got[0])
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
	if got.BudgetEUR != 125.5 {
		t.Fatalf("expected exact budget snapshot, got %v", got.BudgetEUR)
	}
	if got.Objective != "cost" || got.MinQualityScore != 0.82 || got.MaxLatencyMillis != 1200 {
		t.Fatalf("unexpected policy snapshot: %+v", got)
	}
	if got.BudgetScope.Namespace != "finance" || got.BudgetScope.Team != "treasury" {
		t.Fatalf("unexpected budget scope: %+v", got.BudgetScope)
	}
}
