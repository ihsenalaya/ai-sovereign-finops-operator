package predictor

import "testing"

func TestReservationBoundQuantile(t *testing.T) {
	got := ReservationBound([]int{10, 20, 30, 40}, ReservationConfig{
		Mode:     ModeQuantile,
		Quantile: 0.75,
	})
	if got != 30 {
		t.Fatalf("expected 30, got %d", got)
	}
}

func TestReservationBoundMeanStd(t *testing.T) {
	got := ReservationBound([]int{10, 20, 30, 40}, ReservationConfig{
		Mode:   ModeMeanStd,
		ZScore: 1.0,
	})
	if got < 32 {
		t.Fatalf("expected conservative mean+std bound, got %d", got)
	}
}
