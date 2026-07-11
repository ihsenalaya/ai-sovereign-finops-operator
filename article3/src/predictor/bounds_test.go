package predictor

import "testing"

func TestQuantileBound(t *testing.T) {
	got := QuantileBound([]int{40, 10, 20, 30, 50}, 0.75)
	if got != 40 {
		t.Fatalf("expected 40, got %d", got)
	}
}

func TestMeanStdBound(t *testing.T) {
	got := MeanStdBound([]int{10, 12, 14, 16}, 1.0)
	if got < 14 {
		t.Fatalf("expected conservative bound >= 14, got %d", got)
	}
}
