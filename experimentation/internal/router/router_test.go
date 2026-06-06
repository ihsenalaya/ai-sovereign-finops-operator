package router

import (
	"testing"

	"github.com/imperium/ai-sovereign-finops-operator/experimentation/internal/catalog"
)

func testModels() map[string]catalog.Model {
	return catalog.ByID(catalog.Default())
}

func baseCtx() RequestContext {
	return RequestContext{
		Team: "rag", Namespace: "rag", Sensitive: false,
		AllowedModels:  []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1-nano", "selfhosted-eu-llama"},
		PremiumModel:   "gpt-4o",
		MinQuality:     0.75,
		BudgetTotalEUR: 100, BudgetUsedEUR: 0,
		Scenario:        SovScenario{Name: "global", ExternalProvidersAllowed: true},
		EstInputTokens:  500, EstOutputTokens: 200,
	}
}

func TestPremiumStaticAlwaysPremium(t *testing.T) {
	d := PremiumStatic{}.Choose(baseCtx(), testModels())
	if d.ModelID != "gpt-4o" {
		t.Fatalf("got %s, want gpt-4o", d.ModelID)
	}
}

func TestLeastCostPicksCheapest(t *testing.T) {
	d := LeastCost{}.Choose(baseCtx(), testModels())
	// self-hosted has the lowest per-token price in the catalog.
	if d.ModelID != "selfhosted-eu-llama" {
		t.Fatalf("got %s, want selfhosted-eu-llama", d.ModelID)
	}
}

func TestSovereigntyFranceOnlyRejectsUS(t *testing.T) {
	ctx := baseCtx()
	ctx.Scenario = SovScenario{Name: "france-only", AllowedZones: []string{"FR"}, ExternalProvidersAllowed: true}
	ok, rejected := candidates(ctx, testModels())
	if len(ok) != 1 || ok[0].ID != "selfhosted-eu-llama" {
		t.Fatalf("france-only candidates = %v, want [selfhosted-eu-llama]", ids(ok))
	}
	if len(rejected) != 3 {
		t.Fatalf("rejected = %v, want 3 US models", rejected)
	}
}

func TestNoExternalSensitiveForcesSelfHosted(t *testing.T) {
	ctx := baseCtx()
	ctx.Sensitive = true
	ctx.Scenario = SovScenario{Name: "no-external-sensitive", ExternalProvidersAllowed: false}
	d := Ours{W: DefaultWeights()}.Choose(ctx, testModels())
	if d.ModelID != "selfhosted-eu-llama" {
		t.Fatalf("got %s, want selfhosted-eu-llama (only non-external option)", d.ModelID)
	}
	if d.Blocked {
		t.Fatal("should not block; a compliant model exists")
	}
}

func TestOursPrefersPremiumWhenBudgetFreeAndQualityMatters(t *testing.T) {
	ctx := baseCtx()
	ctx.MinQuality = 0.90 // only gpt-4o qualifies
	d := Ours{W: DefaultWeights()}.Choose(ctx, testModels())
	if d.ModelID != "gpt-4o" {
		t.Fatalf("got %s, want gpt-4o under high min-quality", d.ModelID)
	}
}

func TestOursGracefulDegradationWhenBudgetExhausted(t *testing.T) {
	ctx := baseCtx()
	ctx.BudgetUsedEUR = 100 // exhausted
	d := Ours{W: DefaultWeights()}.Choose(ctx, testModels())
	if d.Blocked {
		t.Fatal("Ours must not hard-block; it degrades to keep service available")
	}
	if d.ModelID == "gpt-4o" {
		t.Fatalf("expected degradation to a cheaper model, got premium %s", d.ModelID)
	}
}

func TestBudgetHardBlockBlocksWhenExhausted(t *testing.T) {
	ctx := baseCtx()
	ctx.BudgetUsedEUR = 100
	d := BudgetHardBlock{}.Choose(ctx, testModels())
	if !d.Blocked {
		t.Fatal("B5 must block when budget exhausted")
	}
}

func TestRoundRobinCycles(t *testing.T) {
	rr := &RoundRobin{}
	models := testModels()
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		seen[rr.Choose(baseCtx(), models).ModelID] = true
	}
	if len(seen) < 2 {
		t.Fatalf("round-robin did not cycle, saw %v", seen)
	}
}

func ids(ms []catalog.Model) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}
