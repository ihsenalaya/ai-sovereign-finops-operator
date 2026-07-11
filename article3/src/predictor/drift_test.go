package predictor

import "testing"

func TestDetectMeanRatioDrift(t *testing.T) {
	report := DetectMeanRatioDrift([]int{10, 12, 11, 9}, []int{25, 28, 26, 27}, 2.0)
	if !report.Drifted {
		t.Fatal("expected drift to be detected")
	}
	if report.Ratio < 2.0 {
		t.Fatalf("expected ratio >= 2.0, got %.3f", report.Ratio)
	}
}

func TestDetectMeanRatioDriftNoReference(t *testing.T) {
	report := DetectMeanRatioDrift(nil, []int{5, 6}, 2.0)
	if report.Drifted {
		t.Fatal("did not expect drift without reference window")
	}
}
