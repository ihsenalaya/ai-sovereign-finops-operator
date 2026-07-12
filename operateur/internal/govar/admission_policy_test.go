package govar

import "testing"

func TestExecutableReservationPolicies(t *testing.T) {
	req := admitRequest("policy", testTenant, testWorkload)
	candidate := defaultCandidates()[0]
	tests := []struct {
		method      string
		annotations map[string]string
		want        MoneyMicros
		wantMethod  string
	}{
		{"mean", map[string]string{AnnotationMeanOutputTokens: "500"}, 1_200, "mean"},
		{"fixed_margin", map[string]string{AnnotationMeanOutputTokens: "500", AnnotationMarginTokens: "250"}, 1_600, "fixed_margin"},
		{"fixed_quantile", map[string]string{AnnotationQuantileTokens: "1000"}, 2_000, "fixed_quantile"},
		{"adaptive_quantile", map[string]string{AnnotationAdaptiveTokens: "1200", AnnotationCalibrationSupport: "100"}, 2_320, "adaptive_quantile"},
		{"govar_fixed_cohort", map[string]string{AnnotationAdaptiveTokens: "1200", AnnotationCalibrationSupport: "100", AnnotationCohortSize: "10", AnnotationTenantRiskPPB: "1000000"}, 2_320, "govar_fixed_cohort"},
	}
	for _, tt := range tests {
		routing := defaultRouting()
		routing.Annotations[AnnotationReservationMethod] = tt.method
		for key, value := range tt.annotations {
			routing.Annotations[key] = value
		}
		policyReq := req
		if tt.method == "govar_fixed_cohort" {
			policyReq.CohortID, policyReq.CohortIndex = "fixed-cohort-a", 0
		}
		choice, reason, err := chooseAdmission(policyReq, routing, []Candidate{candidate}, 1_000_000)
		if err != nil || reason != "" || choice.Reservation != tt.want || choice.Method != tt.wantMethod {
			t.Fatalf("%s choice=%+v reason=%s err=%v", tt.method, choice, reason, err)
		}
	}
}

func TestDriftFallsBackToVerifiedStrictBound(t *testing.T) {
	req := admitRequest("drift", testTenant, testWorkload)
	routing := defaultRouting()
	routing.Annotations[AnnotationReservationMethod] = "adaptive_quantile"
	routing.Annotations[AnnotationAdaptiveTokens] = "500"
	routing.Annotations[AnnotationCalibrationSupport] = "100"
	routing.Annotations[AnnotationCalibrationDrift] = "true"
	choice, reason, err := chooseAdmission(req, routing, defaultCandidates(), 1_000_000)
	if err != nil || reason != "" || choice.Method != "strict_provider_cap" || choice.Reservation != 3_600 {
		t.Fatalf("fallback choice=%+v reason=%s err=%v", choice, reason, err)
	}
}

func TestJointSelectionConsidersAvailabilityBeforeQuality(t *testing.T) {
	req := admitRequest("joint", testTenant, testWorkload)
	routing := defaultRouting()
	routing.Spec.Objective = "quality"
	expensive := defaultCandidates()[0]
	expensive.ModelRef, expensive.QualityScore, expensive.OutputPriceMicrosPerMillion = "premium", 0.99, 10_000_000
	cheap := defaultCandidates()[0]
	cheap.ModelRef, cheap.QualityScore = "economy", 0.8
	choice, reason, err := chooseAdmission(req, routing, []Candidate{expensive, cheap}, 4_000)
	if err != nil || reason != "" || choice.Candidate.ModelRef != "economy" {
		t.Fatalf("joint choice=%+v reason=%s err=%v", choice, reason, err)
	}
}

func TestStrictRoughInputUsesContextWindowBound(t *testing.T) {
	req := admitRequest("rough", testTenant, testWorkload)
	req.InputTokensExact = false
	candidate := defaultCandidates()[0]
	candidate.ContextWindow = 10_000
	choice, reason, err := chooseAdmission(req, defaultRouting(), []Candidate{candidate}, 1_000_000)
	if err != nil || reason != "" || choice.Method != "strict_context_window_bound" || choice.InputTokensBound != 8_000 {
		t.Fatalf("rough strict choice=%+v reason=%s err=%v", choice, reason, err)
	}
}

func TestRoughInputForcesNonStrictPolicyToStrictContextBound(t *testing.T) {
	req := admitRequest("rough-mean", testTenant, testWorkload)
	req.InputTokensExact = false
	routing := defaultRouting()
	routing.Annotations[AnnotationReservationMethod] = "mean"
	routing.Annotations[AnnotationMeanOutputTokens] = "10"
	candidate := defaultCandidates()[0]
	candidate.ContextWindow = 10_000
	choice, reason, err := chooseAdmission(req, routing, []Candidate{candidate}, 1_000_000)
	if err != nil || reason != "" || choice.Method != "strict_context_window_bound" || choice.InputTokensBound != 8_000 {
		t.Fatalf("rough mean choice=%+v reason=%s err=%v", choice, reason, err)
	}
}

func TestLatencyObjectiveFailsClosedWithoutObservation(t *testing.T) {
	routing := defaultRouting()
	routing.Spec.Objective = "latency"
	_, reason, err := chooseAdmission(admitRequest("latency", testTenant, testWorkload), routing, defaultCandidates(), 1_000_000)
	if err != nil || reason != ReasonLatencyUnavailable {
		t.Fatalf("latency reason=%s err=%v", reason, err)
	}
}

func TestHardFeasibilityReasonsArePropagated(t *testing.T) {
	candidate := defaultCandidates()[0]
	candidate.Feasible = false
	candidate.InfeasibleReason = ReasonPricingStale
	_, reason, err := chooseAdmission(admitRequest("stale-price", testTenant, testWorkload), defaultRouting(), []Candidate{candidate}, 1_000_000)
	if err != nil || reason != ReasonPricingStale {
		t.Fatalf("reason=%s err=%v", reason, err)
	}
}
