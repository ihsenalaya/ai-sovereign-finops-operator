package tracegateway

import "testing"

func TestSummarize(t *testing.T) {
	s := Summarize(ReplayResult{
		Events: []Event{
			{Type: EventAdmitted, TenantID: "t1", Amount: 5},
			{Type: EventQueued, TenantID: "t1"},
			{Type: EventSettled, TenantID: "t1", RequestID: "r1", Amount: 4},
			{Type: EventAbstained, TenantID: "t2"},
		},
	})
	if s.AdmittedCount != 1 || s.QueuedCount != 1 || s.SettledCount != 1 || s.AbstainedCount != 1 {
		t.Fatalf("unexpected summary counts: %+v", s)
	}
	if s.ReservedTotal != 5 || s.SettledTotal != 4 {
		t.Fatalf("unexpected totals: %+v", s)
	}
	if s.SlackTotal != 0 {
		t.Fatalf("expected no slack without matching request ids, got %+v", s)
	}
}

func TestSummarizeTracksSlackAndOvershoot(t *testing.T) {
	s := Summarize(ReplayResult{
		Events: []Event{
			{Type: EventAdmitted, RequestID: "r1", TenantID: "t1", Amount: 5},
			{Type: EventSettled, RequestID: "r1", TenantID: "t1", Amount: 4},
			{Type: EventAdmitted, RequestID: "r2", TenantID: "t1", Amount: 3},
			{Type: EventSettled, RequestID: "r2", TenantID: "t1", Amount: 4},
		},
	})
	if s.SlackTotal != 1 {
		t.Fatalf("expected slack 1, got %+v", s)
	}
	if s.OvershootTotal != 1 {
		t.Fatalf("expected overshoot 1, got %+v", s)
	}
}
