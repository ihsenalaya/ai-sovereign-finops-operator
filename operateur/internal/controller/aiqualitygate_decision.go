/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

// Decision engine: combine the baseline gate verdict with the continuous-probe
// verdict into a single, traceable EFFECTIVE verdict, governed by a decision
// policy with freshness and anti-flapping safeguards.
//
//   * status.continuousProbes.verdict  = raw/advisory probe verdict
//   * status.verdict                   = effective gate verdict (what callers act on)
//   * status.decision.*                = why, and which sources contributed
//
// Default effect is Observe (advisory only) so a gate without a decisionPolicy
// behaves exactly as before. Probes can BLOCK a candidate but never APPROVE one
// alone (a green probe only removes the continuous-probe blocker).

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/metrics"
	"github.com/imperium/ai-sovereign-finops-operator/internal/qualityengine"
)

const (
	effectObserve          = "Observe"
	effectBlockOnRisk      = "BlockOnRisk"
	effectContributeToGate = "ContributeToGate"
	effectEnforce          = "Enforce"

	insufficientBlock  = "Block"
	insufficientIgnore = "Ignore"

	defaultStaleAfter = 2 * time.Hour
)

// --- freshness + anti-flapping helpers -------------------------------------

func boolToMetric(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func probeStaleAfter(policy *aiopsv1alpha1.AIQualityDecisionPolicy) time.Duration {
	if policy != nil {
		return parseProbeDuration(policy.StaleAfter, defaultStaleAfter)
	}
	return defaultStaleAfter
}

// probeEvidenceFresh is true when the last successful run is recent enough.
func probeEvidenceFresh(st *aiopsv1alpha1.AIQualityContinuousProbesStatus, now time.Time, staleAfter time.Duration) bool {
	if st == nil || st.LastRunStatus != "Succeeded" || st.LastRunAt == nil || st.RunsObserved == 0 {
		return false
	}
	return now.Sub(st.LastRunAt.Time) <= staleAfter
}

func decisionMinRisk(policy *aiopsv1alpha1.AIQualityDecisionPolicy) int32 {
	if policy != nil && policy.MinConsecutiveRisk > 0 {
		return policy.MinConsecutiveRisk
	}
	return 1
}

func decisionMinSafe(policy *aiopsv1alpha1.AIQualityDecisionPolicy) int32 {
	if policy != nil && policy.MinConsecutiveSafe > 0 {
		return policy.MinConsecutiveSafe
	}
	return 1
}

// advanceAntiFlapping updates the consecutive risk/safe counters for one freshly
// completed run and flips the hysteresis "Blocking" state only when the
// configured threshold is reached. Insufficient-data runs leave counters intact.
func (r *AIQualityGateReconciler) advanceAntiFlapping(gate *aiopsv1alpha1.AIQualityGate, st *aiopsv1alpha1.AIQualityContinuousProbesStatus, policy *aiopsv1alpha1.AIQualityDecisionPolicy) {
	switch st.Verdict {
	case qualityengine.VerdictCandidateRisk:
		st.ConsecutiveRiskCount++
		st.ConsecutiveSafeCount = 0
	case qualityengine.VerdictCandidateSafe:
		st.ConsecutiveSafeCount++
		st.ConsecutiveRiskCount = 0
	default:
		// insufficient-data: a completed-but-inconclusive run, leave counters.
	}
	minRisk := decisionMinRisk(policy)
	minSafe := decisionMinSafe(policy)
	switch {
	case !st.Blocking && st.ConsecutiveRiskCount >= minRisk:
		st.Blocking = true
		r.event(gate, corev1.EventTypeWarning, "ContinuousProbeBlocking",
			"continuous probes reached %d consecutive risky runs; candidate is now blocked", st.ConsecutiveRiskCount)
	case st.Blocking && st.ConsecutiveSafeCount >= minSafe:
		st.Blocking = false
		r.event(gate, corev1.EventTypeNormal, "ContinuousProbeCleared",
			"continuous probes reached %d consecutive safe runs; continuous-probe block cleared", st.ConsecutiveSafeCount)
	}
}

// --- decision engine --------------------------------------------------------

// applyQualityDecision combines the already-computed baseline verdict with the
// continuous-probe state and writes status.decision + the effective status.verdict.
// It must run after the baseline verdict is set and after reconcileContinuousProbes.
func (r *AIQualityGateReconciler) applyQualityDecision(gate *aiopsv1alpha1.AIQualityGate, baselineVerdict string) {
	cp := gate.Spec.ContinuousProbes
	policy := decisionPolicyOrNil(cp)
	effect := effectObserve
	insufficientPolicy := insufficientBlock
	if policy != nil {
		if policy.Effect != "" {
			effect = policy.Effect
		}
		if policy.InsufficientDataPolicy != "" {
			insufficientPolicy = policy.InsufficientDataPolicy
		}
	}

	st := gate.Status.ContinuousProbes
	probesEnabled := cp != nil && cp.Enabled && st != nil
	probeVerdict := qualityengine.VerdictInsufficientData
	fresh := false
	var lastRunAt *metav1.Time
	if probesEnabled {
		probeVerdict = st.Verdict
		fresh = st.EvidenceFresh
		lastRunAt = st.LastRunAt
	}

	previousEffective := ""
	if gate.Status.Decision != nil {
		previousEffective = gate.Status.Decision.EffectiveVerdict
	}

	effective := baselineVerdict
	reason := "BaselineGate"
	blockedByProbe := false
	affecting := false

	switch {
	case !probesEnabled || effect == effectObserve:
		reason = reasonForObserve(cp, effect)
	case effect == effectContributeToGate:
		// Baseline already pooled the probe evidence (see Reconcile). The effective
		// verdict is the combined baseline; sources show both contributed.
		effective = baselineVerdict
		reason = "ContributeToGate"
		affecting = true
	default: // BlockOnRisk or Enforce
		probeBlocks := fresh && probeVerdict == qualityengine.VerdictCandidateRisk && st.Blocking
		probeInconclusive := !fresh || probeVerdict == qualityengine.VerdictInsufficientData
		switch {
		case probeBlocks:
			effective = qualityengine.VerdictCandidateRisk
			reason = "ContinuousProbeRisk"
			blockedByProbe = true
			affecting = true
		case probeInconclusive && insufficientPolicy == insufficientBlock:
			// Stale/missing probe evidence blocks, unless the baseline is already a
			// stronger risk signal (keep risk).
			if baselineVerdict == qualityengine.VerdictCandidateRisk {
				effective = qualityengine.VerdictCandidateRisk
			} else {
				effective = qualityengine.VerdictInsufficientData
			}
			reason = "ContinuousProbeInsufficientData"
			blockedByProbe = true
			affecting = true
		default:
			// Probe is safe, or inconclusive-but-ignored: probes only remove a
			// blocker, they never approve a candidate alone -> keep the baseline.
			effective = baselineVerdict
			reason = "BaselineGate"
		}
	}

	// Record the traceable decision.
	gate.Status.Decision = &aiopsv1alpha1.AIQualityDecisionStatus{
		EffectiveVerdict: effective,
		Reason:           reason,
		Sources: aiopsv1alpha1.AIQualityDecisionSources{
			BaselineGate: aiopsv1alpha1.AIQualityDecisionSource{Verdict: baselineVerdict},
			ContinuousProbes: aiopsv1alpha1.AIQualityDecisionSource{
				Verdict:       probeVerdict,
				Effect:        effect,
				LastRunAt:     lastRunAt,
				EvidenceFresh: fresh,
			},
		},
	}

	// The effective verdict is what callers act on.
	gate.Status.Verdict = effective
	gate.Status.Phase = phaseForVerdict(effective)

	// Conditions.
	meta.SetStatusCondition(&gate.Status.Conditions, metav1.Condition{
		Type:               "ContinuousProbeEvidenceFresh",
		Status:             condBool(fresh),
		Reason:             freshReason(probesEnabled, fresh),
		Message:            freshMessage(probesEnabled, fresh),
		ObservedGeneration: gate.Generation,
	})
	meta.SetStatusCondition(&gate.Status.Conditions, metav1.Condition{
		Type:               "ContinuousProbesAffectingGate",
		Status:             condBool(affecting),
		Reason:             affectingReason(effect, affecting),
		Message:            affectingMessage(effect, affecting, reason),
		ObservedGeneration: gate.Generation,
	})
	meta.SetStatusCondition(&gate.Status.Conditions, metav1.Condition{
		Type:               "EffectiveVerdictReady",
		Status:             metav1.ConditionTrue,
		Reason:             "DecisionComputed",
		Message:            "Effective verdict: " + effective + " (" + reason + ").",
		ObservedGeneration: gate.Generation,
	})

	// Metrics.
	metrics.QualityGateEffectiveVerdict.WithLabelValues(
		gate.Namespace, gate.Name, gate.Spec.SourceModel, gate.Spec.CandidateModel, effect, reason,
	).Set(verdictMetricValue(effective))
	metrics.QualityGateBlockedByContinuousProbe.WithLabelValues(
		gate.Namespace, gate.Name, gate.Spec.SourceModel, gate.Spec.CandidateModel,
	).Set(boolToMetric(blockedByProbe))

	// Events on transitions.
	if previousEffective != "" && previousEffective != effective {
		r.event(gate, corev1.EventTypeWarning, "EffectiveVerdictChanged",
			"effective verdict changed from %s to %s (%s)", previousEffective, effective, reason)
	}
	if blockedByProbe && reason == "ContinuousProbeRisk" {
		r.event(gate, corev1.EventTypeWarning, "ContinuousProbeBlockedCandidate",
			"continuous probes are blocking candidate %s for %s/%s", gate.Spec.CandidateModel, gate.Spec.Target.Namespace, gate.Spec.Target.Application)
	}
}

// --- small pure helpers -----------------------------------------------------

func decisionPolicyOrNil(cp *aiopsv1alpha1.AIQualityContinuousProbesSpec) *aiopsv1alpha1.AIQualityDecisionPolicy {
	if cp == nil {
		return nil
	}
	return cp.DecisionPolicy
}

func reasonForObserve(cp *aiopsv1alpha1.AIQualityContinuousProbesSpec, effect string) string {
	if cp != nil && cp.Enabled && effect == effectObserve {
		return "Observe"
	}
	return "BaselineGate"
}

func phaseForVerdict(verdict string) aiopsv1alpha1.AIQualityGatePhase {
	switch verdict {
	case qualityengine.VerdictCandidateSafe:
		return aiopsv1alpha1.AIQualityGatePassed
	case qualityengine.VerdictCandidateRisk:
		return aiopsv1alpha1.AIQualityGateFailed
	default:
		return aiopsv1alpha1.AIQualityGatePending
	}
}

func verdictMetricValue(verdict string) float64 {
	switch verdict {
	case qualityengine.VerdictCandidateSafe:
		return 1
	case qualityengine.VerdictCandidateRisk:
		return 0
	default:
		return -1
	}
}

func condBool(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func freshReason(enabled, fresh bool) string {
	switch {
	case !enabled:
		return "ProbesDisabled"
	case fresh:
		return "EvidenceFresh"
	default:
		return "EvidenceStaleOrMissing"
	}
}

func freshMessage(enabled, fresh bool) string {
	switch {
	case !enabled:
		return "Continuous probes are not enabled."
	case fresh:
		return "Continuous probe evidence is within staleAfter."
	default:
		return "Continuous probe evidence is stale or missing."
	}
}

func affectingReason(effect string, affecting bool) string {
	if affecting {
		return effect
	}
	return "Advisory"
}

func affectingMessage(effect string, affecting bool, reason string) string {
	if affecting {
		return "Continuous probes affect the effective verdict (" + effect + "/" + reason + ")."
	}
	return "Continuous probes are advisory; they do not affect the effective verdict."
}

// event is a nil-safe Recorder helper.
func (r *AIQualityGateReconciler) event(gate *aiopsv1alpha1.AIQualityGate, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(gate, eventType, reason, messageFmt, args...)
}
