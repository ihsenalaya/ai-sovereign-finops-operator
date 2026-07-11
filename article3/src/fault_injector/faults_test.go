package faultinjector

import "testing"

func TestSimulateDuplicateSettlement(t *testing.T) {
	result := SimulateDuplicateSettlement()
	if result.Error == "" {
		t.Fatal("expected duplicate settlement path to record an error")
	}
	if !result.FirstAccepted || !result.SameEventAccepted || !result.SecondDistinctRejected {
		t.Fatalf("unexpected duplicate settlement result: %+v", result)
	}
}

func TestSimulateReservationExpiry(t *testing.T) {
	result := SimulateReservationExpiry()
	if !result.Expired || result.ReservedAfter != 0 || !result.SecondExpireFailed {
		t.Fatalf("unexpected expiry result: %+v", result)
	}
}

func TestSimulateTelemetryFault(t *testing.T) {
	result := SimulateTelemetryFault()
	if !result.Abstained {
		t.Fatalf("expected abstention on telemetry fault, got %+v", result)
	}
}
