package govar

import (
	"testing"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEngineAdmitSettleAndLiability(t *testing.T) {
	engine := NewEngine()
	budget := aiopsv1alpha1.AIBudgetPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "budget"},
		Spec: aiopsv1alpha1.AIBudgetPolicySpec{
			Target:    aiopsv1alpha1.BudgetTarget{Namespace: "finance"},
			BudgetEUR: resource.MustParse("100"),
		},
	}
	routing := aiopsv1alpha1.AIRoutingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "routing"},
		Spec: aiopsv1alpha1.AIRoutingPolicySpec{
			Objective: "cost",
			Guardrails: aiopsv1alpha1.AIRoutingPolicyGuardrails{
				RequireSovereigntyCompliance: true,
			},
		},
	}
	candidates := []Candidate{{
		ModelRef:              "gpt-fr",
		InputPricePerMillion:  0.4,
		OutputPricePerMillion: 1.6,
	}}

	admit, err := engine.Admit(AdmitRequest{
		RequestID:         "r1",
		Namespace:         "finance",
		TenantID:          "tenant-a",
		BudgetPolicyName:  "budget",
		RoutingPolicyName: "routing",
		InputTokens:       1000,
		MaxOutputTokens:   2000,
	}, budget, routing, candidates)
	if err != nil {
		t.Fatalf("admit returned error: %v", err)
	}
	if admit.Decision != DecisionAdmit {
		t.Fatalf("expected admit, got %+v", admit)
	}
	if admit.ReasonCode != ReasonHighestUtility {
		t.Fatalf("expected highest utility reason, got %+v", admit)
	}

	liability := engine.Liability("tenant-a")
	if liability.ReservedLiability <= 0 {
		t.Fatalf("expected reserved liability after admit, got %+v", liability)
	}

	_, code, err := engine.Settle(SettleRequest{
		RequestID:    "r1",
		SettlementID: "s1",
		ActualCost:   0.01,
	})
	if err != nil {
		t.Fatalf("settle returned error: %v", err)
	}
	if code != ReasonHighestUtility {
		t.Fatalf("expected settle success code, got %s", code)
	}

	liability = engine.Liability("tenant-a")
	if liability.ReservedLiability != 0 {
		t.Fatalf("expected zero reserved liability after settlement, got %+v", liability)
	}
	if liability.SettledSpend != 0.01 {
		t.Fatalf("expected settled spend, got %+v", liability)
	}
}

func TestEngineQueuesWhenBudgetUnavailable(t *testing.T) {
	engine := NewEngine()
	budget := aiopsv1alpha1.AIBudgetPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "budget"},
		Spec: aiopsv1alpha1.AIBudgetPolicySpec{
			BudgetEUR: resource.MustParse("0.0001"),
		},
	}
	routing := aiopsv1alpha1.AIRoutingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "routing"},
	}

	admit, err := engine.Admit(AdmitRequest{
		RequestID:         "r-low",
		TenantID:          "tenant-low",
		BudgetPolicyName:  "budget",
		RoutingPolicyName: "routing",
		InputTokens:       100000,
		MaxOutputTokens:   100000,
	}, budget, routing, []Candidate{{
		ModelRef:              "expensive",
		InputPricePerMillion:  5,
		OutputPricePerMillion: 5,
	}})
	if err != nil {
		t.Fatalf("admit returned error: %v", err)
	}
	if admit.Decision != DecisionQueue || admit.ReasonCode != ReasonBudgetUnavailable {
		t.Fatalf("expected queue on budget unavailable, got %+v", admit)
	}
}
