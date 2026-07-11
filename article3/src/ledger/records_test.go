package ledger

import "testing"

func TestRequestLedgerSettleOnceIsIdempotentForSameEvent(t *testing.T) {
	state := TenantState{TenantID: "tenant-a", Budget: 100}
	ledger := NewRequestLedger()

	if err := ledger.Reserve(&state, "req-1", "gpt-x", 30); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if err := ledger.SettleOnce(&state, "req-1", "evt-1", 18); err != nil {
		t.Fatalf("first settle failed: %v", err)
	}
	settledAfterFirst := state.Settled
	if err := ledger.SettleOnce(&state, "req-1", "evt-1", 18); err != nil {
		t.Fatalf("idempotent settle failed: %v", err)
	}
	if state.Settled != settledAfterFirst {
		t.Fatalf("expected settled amount to stay at %.2f, got %.2f", settledAfterFirst, state.Settled)
	}
}

func TestRequestLedgerRejectsSecondSettlementEvent(t *testing.T) {
	state := TenantState{TenantID: "tenant-a", Budget: 100}
	ledger := NewRequestLedger()

	if err := ledger.Reserve(&state, "req-1", "gpt-x", 30); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if err := ledger.SettleOnce(&state, "req-1", "evt-1", 18); err != nil {
		t.Fatalf("first settle failed: %v", err)
	}
	if err := ledger.SettleOnce(&state, "req-1", "evt-2", 19); err == nil {
		t.Fatal("expected second distinct settlement event to fail")
	}
}

func TestRequestLedgerExpireReleasesReservation(t *testing.T) {
	state := TenantState{TenantID: "tenant-a", Budget: 100}
	ledger := NewRequestLedger()

	if err := ledger.Reserve(&state, "req-2", "gpt-y", 25); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if err := ledger.Expire(&state, "req-2"); err != nil {
		t.Fatalf("expire failed: %v", err)
	}
	if state.Reserved != 0 {
		t.Fatalf("expected reserved budget to be 0, got %.2f", state.Reserved)
	}
	rec, ok := ledger.Get("req-2")
	if !ok {
		t.Fatal("expected reservation record to exist")
	}
	if rec.Status != StatusExpired {
		t.Fatalf("expected expired status, got %s", rec.Status)
	}
}
