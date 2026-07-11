package tracegateway

import (
	"fmt"
	"math/rand"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
)

type ScenarioKind string

const (
	ScenarioBudgetDelay ScenarioKind = "budget_delay"
	ScenarioMultitenant ScenarioKind = "multitenant"
)

type ScenarioConfig struct {
	Kind                ScenarioKind
	Seed                int64
	RequestCount        int
	TenantBudget        float64
	SecondTenantBudget  float64
	QueueRetries        int
	Bursty              bool
}

func BuildScenario(cfg ScenarioConfig) ([]TraceRequest, map[string]TenantBudgetConfig, error) {
	switch cfg.Kind {
	case ScenarioBudgetDelay:
		return buildBudgetDelayScenario(cfg), map[string]TenantBudgetConfig{
			"team-a": {TenantID: "team-a", Budget: cfg.TenantBudget},
		}, nil
	case ScenarioMultitenant:
		return buildMultitenantScenario(cfg), map[string]TenantBudgetConfig{
			"team-a": {TenantID: "team-a", Budget: cfg.TenantBudget},
			"team-b": {TenantID: "team-b", Budget: cfg.SecondTenantBudget},
		}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported scenario kind %q", cfg.Kind)
	}
}

type TenantBudgetConfig struct {
	TenantID string
	Budget   float64
}

func buildBudgetDelayScenario(cfg ScenarioConfig) []TraceRequest {
	rng := rand.New(rand.NewSource(cfg.Seed))
	n := cfg.RequestCount
	if n <= 0 {
		n = 8
	}
	requests := make([]TraceRequest, 0, n)
	for i := 0; i < n; i++ {
		profile := i % 3
		app, premiumPreds, economyPreds, actualBase := budgetDelayProfile(rng, profile)
		actual := actualBase + float64(rng.Intn(3))
		arrival := i / 2
		if cfg.Bursty {
			arrival = i / 4
		}
		requests = append(requests, TraceRequest{
			RequestID:       fmt.Sprintf("bd-%02d", i),
			TenantID:        "team-a",
			Application:     app,
			ArrivalStep:     arrival,
			SettlementDelay: 1 + rng.Intn(3),
			ActualCost:      actual,
			MaxQueueRetries: cfg.QueueRetries,
			Candidates: []CandidateSpec{
				{
					Base: admission.Candidate{
						Model:              "m-budget-premium",
						UtilityScore:       0.82 + float64(rng.Intn(10))/100.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
					},
					PredictedOutputTokens: premiumPreds,
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
				{
					Base: admission.Candidate{
						Model:              "m-budget-economy",
						UtilityScore:       0.68 + float64(rng.Intn(8))/100.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
					},
					PredictedOutputTokens: economyPreds,
					InputCost:             0.7,
					OutputCostPerToken:    0.35,
				},
			},
		})
	}
	return requests
}

func buildMultitenantScenario(cfg ScenarioConfig) []TraceRequest {
	rng := rand.New(rand.NewSource(cfg.Seed))
	n := cfg.RequestCount
	if n <= 0 {
		n = 10
	}
	requests := make([]TraceRequest, 0, n)
	for i := 0; i < n; i++ {
		tenant := "team-a"
		profile := "analytical"
		if i%2 == 1 {
			tenant = "team-b"
			profile = "chatbot"
		}
		app, premiumPreds, economyPreds, actualBase := multitenantProfile(rng, profile)
		actual := actualBase + float64(rng.Intn(2))
		arrival := i / 3
		if cfg.Bursty {
			arrival = i / 5
		}
		requests = append(requests, TraceRequest{
			RequestID:       fmt.Sprintf("mt-%02d", i),
			TenantID:        tenant,
			Application:     app,
			ArrivalStep:     arrival,
			SettlementDelay: 1 + rng.Intn(2),
			ActualCost:      actual,
			MaxQueueRetries: cfg.QueueRetries,
			Candidates: []CandidateSpec{
				{
					Base: admission.Candidate{
						Model:              app + "-premium",
						UtilityScore:       0.80 + float64(rng.Intn(10))/100.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
					},
					PredictedOutputTokens: premiumPreds,
					InputCost:             1,
					OutputCostPerToken:    0.5,
				},
				{
					Base: admission.Candidate{
						Model:              app + "-economy",
						UtilityScore:       0.66 + float64(rng.Intn(10))/100.0,
						GovernanceAllowed:  true,
						TelemetryFresh:     true,
						CalibrationTrusted: true,
						EvidenceStrong:     true,
					},
					PredictedOutputTokens: economyPreds,
					InputCost:             0.6,
					OutputCostPerToken:    0.35,
				},
			},
		})
	}
	return requests
}

func budgetDelayProfile(rng *rand.Rand, profile int) (string, []int, []int, float64) {
	switch profile {
	case 0:
		return "hr-chatbot", []int{6 + rng.Intn(3), 8 + rng.Intn(3), 9 + rng.Intn(3), 11 + rng.Intn(3)}, []int{4 + rng.Intn(2), 5 + rng.Intn(2), 6 + rng.Intn(2), 7 + rng.Intn(2)}, 4
	case 1:
		return "rag-docs", []int{9 + rng.Intn(3), 11 + rng.Intn(3), 13 + rng.Intn(3), 15 + rng.Intn(3)}, []int{6 + rng.Intn(2), 8 + rng.Intn(2), 9 + rng.Intn(2), 10 + rng.Intn(2)}, 6
	default:
		return "analytical-agent", []int{12 + rng.Intn(4), 14 + rng.Intn(4), 16 + rng.Intn(4), 18 + rng.Intn(4)}, []int{8 + rng.Intn(3), 10 + rng.Intn(3), 11 + rng.Intn(3), 13 + rng.Intn(3)}, 7
	}
}

func multitenantProfile(rng *rand.Rand, profile string) (string, []int, []int, float64) {
	switch profile {
	case "chatbot":
		return "support-chatbot", []int{4 + rng.Intn(3), 5 + rng.Intn(3), 7 + rng.Intn(3), 8 + rng.Intn(3)}, []int{3 + rng.Intn(2), 4 + rng.Intn(2), 5 + rng.Intn(2), 6 + rng.Intn(2)}, 3
	default:
		return "finance-analyst", []int{7 + rng.Intn(3), 9 + rng.Intn(3), 11 + rng.Intn(3), 13 + rng.Intn(3)}, []int{5 + rng.Intn(2), 6 + rng.Intn(2), 8 + rng.Intn(2), 9 + rng.Intn(2)}, 5
	}
}
