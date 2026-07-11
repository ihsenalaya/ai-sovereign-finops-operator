package ledger

import "testing"

func TestTenantStateReserveAndSettle(t *testing.T) {
	s := TenantState{TenantID: "team-a", Budget: 100}
	if err := s.Reserve(25); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if s.Available() != 75 {
		t.Fatalf("expected 75 available, got %.2f", s.Available())
	}
	if err := s.Settle(20, 25); err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	if s.Reserved != 0 {
		t.Fatalf("expected reserved 0, got %.2f", s.Reserved)
	}
	if s.Settled != 20 {
		t.Fatalf("expected settled 20, got %.2f", s.Settled)
	}
}

func TestTenantStateSettleToleratesFloatRoundingNoise(t *testing.T) {
	s := TenantState{TenantID: "team-a", Budget: 20}
	if err := s.Reserve(4.2); err != nil {
		t.Fatalf("reserve 4.2 failed: %v", err)
	}
	if err := s.Reserve(7.35); err != nil {
		t.Fatalf("reserve 7.35 failed: %v", err)
	}
	if err := s.Settle(4.2, 4.2); err != nil {
		t.Fatalf("first settle failed: %v", err)
	}
	if err := s.Settle(7.0, 7.35); err != nil {
		t.Fatalf("second settle failed despite benign rounding noise: %v", err)
	}
	if s.Reserved != 0 {
		t.Fatalf("expected reserved to normalize to 0, got %.12f", s.Reserved)
	}
}
