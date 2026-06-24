package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/qualityengine"
)

func gateWith(effect, insufficient string, probeStatus *aiopsv1alpha1.AIQualityContinuousProbesStatus) *aiopsv1alpha1.AIQualityGate {
	g := &aiopsv1alpha1.AIQualityGate{}
	g.Namespace, g.Name = "ns", "gate"
	g.Spec.SourceModel, g.Spec.CandidateModel = "src", "cand"
	if effect != "" || probeStatus != nil {
		g.Spec.ContinuousProbes = &aiopsv1alpha1.AIQualityContinuousProbesSpec{Enabled: true}
		if effect != "" {
			g.Spec.ContinuousProbes.DecisionPolicy = &aiopsv1alpha1.AIQualityDecisionPolicy{
				Effect:                 effect,
				InsufficientDataPolicy: insufficient,
			}
		}
	}
	g.Status.ContinuousProbes = probeStatus
	return g
}

// 1. Backward compatibility: no continuousProbes -> effective == baseline.
func TestDecisionBackwardCompatNoProbes(t *testing.T) {
	r := &AIQualityGateReconciler{}
	g := gateWith("", "", nil)
	g.Spec.ContinuousProbes = nil
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictCandidateSafe {
		t.Fatalf("effective should equal baseline, got %s", g.Status.Verdict)
	}
	if g.Status.Decision == nil || g.Status.Decision.Reason != "BaselineGate" {
		t.Fatalf("reason should be BaselineGate, got %+v", g.Status.Decision)
	}
}

// 2. Observe: probe risk does NOT change the effective verdict.
func TestDecisionObserveDoesNotBlock(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictCandidateRisk,
		EvidenceFresh: true, Blocking: true,
		LastRunStatus: "Succeeded", LastRunAt: &metav1.Time{Time: time.Now()},
	}
	g := gateWith(effectObserve, "", probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictCandidateSafe {
		t.Fatalf("Observe must not change verdict, got %s", g.Status.Verdict)
	}
	if g.Status.Decision.Reason != "Observe" {
		t.Fatalf("reason should be Observe, got %s", g.Status.Decision.Reason)
	}
}

// 3. BlockOnRisk: baseline safe + probe risk -> effective risk, reason ContinuousProbeRisk.
func TestDecisionBlockOnRiskFlips(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictCandidateRisk,
		EvidenceFresh: true, Blocking: true,
		LastRunStatus: "Succeeded", LastRunAt: &metav1.Time{Time: time.Now()},
	}
	g := gateWith(effectBlockOnRisk, insufficientBlock, probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictCandidateRisk {
		t.Fatalf("BlockOnRisk should flip to risk, got %s", g.Status.Verdict)
	}
	if g.Status.Decision.Reason != "ContinuousProbeRisk" {
		t.Fatalf("reason should be ContinuousProbeRisk, got %s", g.Status.Decision.Reason)
	}
}

// 3b. BlockOnRisk: probe SAFE must NOT approve a risky baseline alone.
func TestDecisionProbesNeverApproveAlone(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictCandidateSafe,
		EvidenceFresh: true,
		LastRunStatus: "Succeeded", LastRunAt: &metav1.Time{Time: time.Now()},
	}
	g := gateWith(effectBlockOnRisk, insufficientBlock, probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateRisk)
	if g.Status.Verdict != qualityengine.VerdictCandidateRisk {
		t.Fatalf("safe probe must not approve a risky baseline, got %s", g.Status.Verdict)
	}
}

// 4. Insufficient data blocking.
func TestDecisionInsufficientBlocks(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictInsufficientData, EvidenceFresh: false,
	}
	g := gateWith(effectBlockOnRisk, insufficientBlock, probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictInsufficientData {
		t.Fatalf("insufficient+Block should block, got %s", g.Status.Verdict)
	}
	if g.Status.Decision.Reason != "ContinuousProbeInsufficientData" {
		t.Fatalf("reason should be ContinuousProbeInsufficientData, got %s", g.Status.Decision.Reason)
	}
}

// 5. Insufficient data ignored -> follows baseline.
func TestDecisionInsufficientIgnored(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictInsufficientData, EvidenceFresh: false,
	}
	g := gateWith(effectBlockOnRisk, insufficientIgnore, probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictCandidateSafe {
		t.Fatalf("insufficient+Ignore should follow baseline, got %s", g.Status.Verdict)
	}
}

// 6. Stale evidence is treated as inconclusive.
func TestDecisionStaleEvidenceBlocks(t *testing.T) {
	staleAfter := 2 * time.Hour
	st := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, RunsObserved: 1, LastRunStatus: "Succeeded",
		LastRunAt: &metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
	}
	if probeEvidenceFresh(st, time.Now(), staleAfter) {
		t.Fatal("3h-old evidence with 2h staleAfter should be stale")
	}
	st.EvidenceFresh = probeEvidenceFresh(st, time.Now(), staleAfter)
	st.Verdict = qualityengine.VerdictCandidateSafe
	r := &AIQualityGateReconciler{}
	g := gateWith(effectBlockOnRisk, insufficientBlock, st)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	if g.Status.Verdict != qualityengine.VerdictInsufficientData {
		t.Fatalf("stale evidence with Block should block, got %s", g.Status.Verdict)
	}
}

// 7. Anti-flapping: one risky run does not flip if minConsecutiveRisk=2.
func TestAntiFlappingRequiresConsecutiveRuns(t *testing.T) {
	r := &AIQualityGateReconciler{}
	policy := &aiopsv1alpha1.AIQualityDecisionPolicy{MinConsecutiveRisk: 2, MinConsecutiveSafe: 2}
	g := &aiopsv1alpha1.AIQualityGate{}
	st := &aiopsv1alpha1.AIQualityContinuousProbesStatus{Verdict: qualityengine.VerdictCandidateRisk}

	r.advanceAntiFlapping(g, st, policy)
	if st.Blocking {
		t.Fatal("one risky run must not block with minConsecutiveRisk=2")
	}
	r.advanceAntiFlapping(g, st, policy)
	if !st.Blocking {
		t.Fatal("two consecutive risky runs must block")
	}
	// One safe run must not clear with minConsecutiveSafe=2.
	st.Verdict = qualityengine.VerdictCandidateSafe
	r.advanceAntiFlapping(g, st, policy)
	if !st.Blocking {
		t.Fatal("one safe run must not clear with minConsecutiveSafe=2")
	}
	r.advanceAntiFlapping(g, st, policy)
	if st.Blocking {
		t.Fatal("two consecutive safe runs must clear the block")
	}
}

// 8. Status traceability: sources + reason populated.
func TestDecisionTraceabilitySources(t *testing.T) {
	r := &AIQualityGateReconciler{}
	probe := &aiopsv1alpha1.AIQualityContinuousProbesStatus{
		Enabled: true, Verdict: qualityengine.VerdictCandidateRisk,
		EvidenceFresh: true, Blocking: true,
		LastRunStatus: "Succeeded", LastRunAt: &metav1.Time{Time: time.Now()},
	}
	g := gateWith(effectBlockOnRisk, insufficientBlock, probe)
	r.applyQualityDecision(g, qualityengine.VerdictCandidateSafe)
	d := g.Status.Decision
	if d == nil || d.Sources.BaselineGate.Verdict != qualityengine.VerdictCandidateSafe {
		t.Fatalf("baselineGate source not populated: %+v", d)
	}
	if d.Sources.ContinuousProbes.Verdict != qualityengine.VerdictCandidateRisk || d.Sources.ContinuousProbes.Effect != effectBlockOnRisk {
		t.Fatalf("continuousProbes source not populated: %+v", d.Sources.ContinuousProbes)
	}
	if d.Reason == "" || d.EffectiveVerdict == "" {
		t.Fatal("reason and effectiveVerdict must be populated")
	}
}
