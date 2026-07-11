package admission

import (
	"sort"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
)

type Action string

const (
	ActionAdmit   Action = "admit"
	ActionQueue   Action = "queue"
	ActionReject  Action = "reject"
	ActionAbstain Action = "abstain"
)

type ReasonCode string

const (
	ReasonCodeHighestUtility            ReasonCode = "highest_utility_feasible"
	ReasonCodeNoFeasibleCandidate       ReasonCode = "no_feasible_candidate"
	ReasonCodeInsufficientEvidence      ReasonCode = "insufficient_evidence"
	ReasonCodeGovernanceFiltered        ReasonCode = "governance_filtered"
	ReasonCodeBudgetUnavailable         ReasonCode = "budget_unavailable"
	ReasonCodeTelemetryStale            ReasonCode = "telemetry_stale"
	ReasonCodeCalibrationUntrusted      ReasonCode = "calibration_untrusted"
	ReasonCodeDriftBlocked              ReasonCode = "drift_blocked"
)

type Candidate struct {
	Model              string
	UtilityScore       float64
	ReservationCost    float64
	GovernanceAllowed  bool
	TelemetryFresh     bool
	CalibrationTrusted bool
	EvidenceStrong     bool
	Drifted            bool
}

type Policy struct {
	StrictMode            bool
	AllowQueue            bool
	RequireFreshSignals   bool
	RequireStrongEvidence bool
	BlockOnDrift          bool
}

type Decision struct {
	Action          Action
	Model           string
	ReservationCost float64
	ReasonCode      ReasonCode
	Reason          string
}

func Decide(policy Policy, tenant ledger.TenantState, candidates []Candidate) Decision {
	filtered := make([]Candidate, 0, len(candidates))
	reasonCounts := map[ReasonCode]int{}
	for _, c := range candidates {
		if !c.GovernanceAllowed {
			reasonCounts[ReasonCodeGovernanceFiltered]++
			continue
		}
		if policy.RequireFreshSignals && !c.TelemetryFresh {
			reasonCounts[ReasonCodeTelemetryStale]++
			continue
		}
		if policy.StrictMode && !c.CalibrationTrusted {
			reasonCounts[ReasonCodeCalibrationUntrusted]++
			continue
		}
		if policy.RequireStrongEvidence && !c.EvidenceStrong {
			reasonCounts[ReasonCodeInsufficientEvidence]++
			continue
		}
		if policy.BlockOnDrift && c.Drifted {
			reasonCounts[ReasonCodeDriftBlocked]++
			continue
		}
		if tenant.Available() < c.ReservationCost {
			reasonCounts[ReasonCodeBudgetUnavailable]++
			continue
		}
		filtered = append(filtered, c)
	}

	if len(filtered) == 0 {
		if reasonCounts[ReasonCodeInsufficientEvidence] > 0 || reasonCounts[ReasonCodeDriftBlocked] > 0 || reasonCounts[ReasonCodeCalibrationUntrusted] > 0 || reasonCounts[ReasonCodeTelemetryStale] > 0 {
			return Decision{
				Action:     ActionAbstain,
				ReasonCode: dominantReason(reasonCounts),
				Reason:     "insufficient evidence strength for a governed routing decision",
			}
		}
		if policy.AllowQueue {
			return Decision{
				Action:     ActionQueue,
				ReasonCode: dominantReason(reasonCounts, ReasonCodeNoFeasibleCandidate),
				Reason:     "no feasible candidate under current budget or evidence constraints",
			}
		}
		return Decision{
			Action:     ActionReject,
			ReasonCode: dominantReason(reasonCounts, ReasonCodeNoFeasibleCandidate),
			Reason:     "no feasible candidate under current budget or evidence constraints",
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].UtilityScore > filtered[j].UtilityScore
	})
	best := filtered[0]
	return Decision{
		Action:          ActionAdmit,
		Model:           best.Model,
		ReservationCost: best.ReservationCost,
		ReasonCode:      ReasonCodeHighestUtility,
		Reason:          "highest utility among feasible candidates",
	}
}

func dominantReason(counts map[ReasonCode]int, fallback ...ReasonCode) ReasonCode {
	priorities := []ReasonCode{
		ReasonCodeInsufficientEvidence,
		ReasonCodeDriftBlocked,
		ReasonCodeCalibrationUntrusted,
		ReasonCodeTelemetryStale,
		ReasonCodeBudgetUnavailable,
		ReasonCodeGovernanceFiltered,
		ReasonCodeNoFeasibleCandidate,
	}
	best := ReasonCode("")
	bestCount := 0
	for _, code := range priorities {
		if counts[code] > bestCount {
			best = code
			bestCount = counts[code]
		}
	}
	if best != "" {
		return best
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ReasonCodeNoFeasibleCandidate
}
