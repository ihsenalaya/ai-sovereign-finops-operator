package qualitygate

import (
	"testing"
	"time"
)

func gates() []GateVerdict {
	now := time.Now()
	return []GateVerdict{
		{Name: "safe", SourceModel: "gpt-us-mini", CandidateModel: "gpt-france-mini", Verdict: "candidate-safe", EvaluatedAt: now.Add(-time.Hour)},
		{Name: "risk", SourceModel: "gpt-us-mini", CandidateModel: "mistral-small", Verdict: "candidate-risk", EvaluatedAt: now.Add(-time.Hour)},
		{Name: "insufficient", SourceModel: "gpt-us-mini", CandidateModel: "cohere-eu", Verdict: "insufficient-data", EvaluatedAt: time.Time{}},
	}
}

func TestAllowsFreshSafe(t *testing.T) {
	d := EvaluateReroute(gates(), "gpt-us-mini", "gpt-france-mini", time.Now(), 24*time.Hour)
	if !d.Allowed || d.GateName != "safe" {
		t.Fatalf("expected allowed via gate 'safe', got %+v", d)
	}
}

func TestBlocksCandidateRisk(t *testing.T) {
	d := EvaluateReroute(gates(), "gpt-us-mini", "mistral-small", time.Now(), 24*time.Hour)
	if d.Allowed {
		t.Fatalf("candidate-risk must block automatic actuation, got %+v", d)
	}
}

func TestBlocksInsufficientData(t *testing.T) {
	d := EvaluateReroute(gates(), "gpt-us-mini", "cohere-eu", time.Now(), 24*time.Hour)
	if d.Allowed {
		t.Fatalf("insufficient-data must block automatic actuation, got %+v", d)
	}
}

func TestBlocksWhenNoGate(t *testing.T) {
	d := EvaluateReroute(gates(), "gpt-us-mini", "unknown-model", time.Now(), 24*time.Hour)
	if d.Allowed {
		t.Fatalf("missing gate must block automatic actuation, got %+v", d)
	}
}

func TestBlocksStaleSafe(t *testing.T) {
	g := gates()
	g[0].EvaluatedAt = time.Now().Add(-48 * time.Hour) // stale
	d := EvaluateReroute(g, "gpt-us-mini", "gpt-france-mini", time.Now(), 24*time.Hour)
	if d.Allowed {
		t.Fatalf("stale candidate-safe must block when maxAge is set, got %+v", d)
	}
}

func TestZeroMaxAgeIgnoresFreshness(t *testing.T) {
	g := gates()
	g[0].EvaluatedAt = time.Now().Add(-1000 * time.Hour)
	d := EvaluateReroute(g, "gpt-us-mini", "gpt-france-mini", time.Now(), 0)
	if !d.Allowed {
		t.Fatalf("maxAge=0 should ignore staleness, got %+v", d)
	}
}
