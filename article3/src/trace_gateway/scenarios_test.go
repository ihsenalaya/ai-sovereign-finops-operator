package tracegateway

import "testing"

func TestBuildScenarioBudgetDelay(t *testing.T) {
	reqs, tenants, err := BuildScenario(ScenarioConfig{
		Kind:         ScenarioBudgetDelay,
		Seed:         1,
		RequestCount: 4,
		TenantBudget: 20,
		QueueRetries: 1,
	})
	if err != nil {
		t.Fatalf("build scenario failed: %v", err)
	}
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(reqs))
	}
	if tenants["team-a"].Budget != 20 {
		t.Fatalf("unexpected tenant budget map: %+v", tenants)
	}
}

func TestBuildScenarioMultitenant(t *testing.T) {
	reqs, tenants, err := BuildScenario(ScenarioConfig{
		Kind:               ScenarioMultitenant,
		Seed:               2,
		RequestCount:       6,
		TenantBudget:       15,
		SecondTenantBudget: 12,
	})
	if err != nil {
		t.Fatalf("build scenario failed: %v", err)
	}
	if len(reqs) != 6 {
		t.Fatalf("expected 6 requests, got %d", len(reqs))
	}
	if len(tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %+v", tenants)
	}
}
