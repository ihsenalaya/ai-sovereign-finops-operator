package govar

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

const (
	AnnotationReservationMethod        = "aiops.imperium.io/govar-reservation-method"
	AnnotationMeanOutputTokens         = "aiops.imperium.io/govar-mean-output-tokens"
	AnnotationMarginTokens             = "aiops.imperium.io/govar-margin-tokens"
	AnnotationQuantileTokens           = "aiops.imperium.io/govar-quantile-output-tokens"
	AnnotationAdaptiveTokens           = "aiops.imperium.io/govar-adaptive-output-tokens"
	AnnotationCalibrationSupport       = "aiops.imperium.io/govar-calibration-support"
	AnnotationCalibrationDrift         = "aiops.imperium.io/govar-calibration-drift"
	AnnotationCohortSize               = "aiops.imperium.io/govar-cohort-size"
	AnnotationTenantRiskPPB            = "aiops.imperium.io/govar-tenant-risk-ppb"
	minimumCalibrationSupport    int64 = 100
)

type admissionChoice struct {
	Candidate        Candidate
	Reservation      MoneyMicros
	Method           string
	AllocatedRiskPPB int64
	InputTokensBound int64
}

func validatePolicyAndTarget(req AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy) ReasonCode {
	if budget.Status.ObservedGeneration != budget.Generation || !conditionTrue(budget.Status.Conditions, aiopsv1alpha1.ConditionReady) ||
		routing.Status.ObservedGeneration != routing.Generation || !conditionTrue(routing.Status.Conditions, aiopsv1alpha1.ConditionReady) {
		return ReasonPolicyNotReady
	}
	if routing.Status.LastEvaluatedAt == nil || time.Since(routing.Status.LastEvaluatedAt.Time) > observationFreshnessLimit {
		return ReasonPolicyNotReady
	}
	target := budget.Spec.Target
	if target.Namespace != "" && target.Namespace != req.Namespace {
		return ReasonBudgetTargetMismatch
	}
	if target.Team != "" && target.Team != req.Team {
		return ReasonBudgetTargetMismatch
	}
	if target.Application != "" && target.Application != req.Application {
		return ReasonBudgetTargetMismatch
	}
	return ""
}

func chooseAdmission(req AdmitRequest, routing aiopsv1alpha1.AIRoutingPolicy, candidates []Candidate, available MoneyMicros) (admissionChoice, ReasonCode, error) {
	method := strings.TrimSpace(routing.Annotations[AnnotationReservationMethod])
	if method == "" {
		method = "strict_provider_cap"
	}
	var choices []admissionChoice
	lastReason := ReasonNoCandidate
	feasibleBeforeBudget := false
	for _, candidate := range candidates {
		if !candidate.Feasible {
			if candidate.InfeasibleReason != "" {
				lastReason = candidate.InfeasibleReason
			}
			continue
		}
		if candidate.QualityScore < routing.Spec.Guardrails.MinQualityScore {
			lastReason = ReasonQualityBelowMinimum
			continue
		}
		if (routing.Spec.Guardrails.MaxLatencyMillis > 0 || strings.EqualFold(routing.Spec.Objective, "latency")) && candidate.LatencyObservedAt.IsZero() {
			lastReason = ReasonLatencyUnavailable
			continue
		}
		if routing.Spec.Guardrails.MaxLatencyMillis > 0 && candidate.LatencyMillis > int64(routing.Spec.Guardrails.MaxLatencyMillis) {
			lastReason = ReasonLatencyExceeded
			continue
		}
		if candidate.ContextWindow <= 0 || (req.InputTokensExact && req.InputTokens+req.MaxOutputTokens > candidate.ContextWindow) || (!req.InputTokensExact && req.MaxOutputTokens >= candidate.ContextWindow) {
			lastReason = ReasonContextLimit
			continue
		}
		outputTokens, allocatedRisk, effectiveMethod, reason := reservationTokens(method, req, routing, candidate)
		if reason != "" {
			lastReason = reason
			continue
		}
		inputBound := req.InputTokens
		if strings.HasPrefix(effectiveMethod, "strict_") && !req.InputTokensExact {
			inputBound = candidate.ContextWindow - outputTokens
			effectiveMethod = "strict_context_window_bound"
		}
		reservation, err := costFromPriceMicros(candidate.InputPriceMicrosPerMillion, candidate.OutputPriceMicrosPerMillion, inputBound, outputTokens)
		if err != nil {
			return admissionChoice{}, ReasonPricingIncomplete, err
		}
		feasibleBeforeBudget = true
		if reservation > available {
			continue
		}
		choices = append(choices, admissionChoice{Candidate: candidate, Reservation: reservation, Method: effectiveMethod, AllocatedRiskPPB: allocatedRisk, InputTokensBound: inputBound})
	}
	if len(choices) == 0 {
		if feasibleBeforeBudget {
			return admissionChoice{}, ReasonBudgetUnavailable, nil
		}
		return admissionChoice{}, lastReason, nil
	}
	sort.Slice(choices, func(i, j int) bool {
		if strings.EqualFold(routing.Spec.Objective, "quality") && choices[i].Candidate.QualityScore != choices[j].Candidate.QualityScore {
			return choices[i].Candidate.QualityScore > choices[j].Candidate.QualityScore
		}
		if strings.EqualFold(routing.Spec.Objective, "latency") && choices[i].Candidate.LatencyMillis != choices[j].Candidate.LatencyMillis {
			return choices[i].Candidate.LatencyMillis < choices[j].Candidate.LatencyMillis
		}
		if choices[i].Reservation != choices[j].Reservation {
			return choices[i].Reservation < choices[j].Reservation
		}
		return choices[i].Candidate.ModelRef < choices[j].Candidate.ModelRef
	})
	return choices[0], "", nil
}

func reservationTokens(method string, req AdmitRequest, routing aiopsv1alpha1.AIRoutingPolicy, candidate Candidate) (int64, int64, string, ReasonCode) {
	capTokens := req.MaxOutputTokens
	if capTokens <= 0 {
		return 0, 0, method, ReasonInsufficientEvidence
	}
	strict := func() (int64, int64, string, ReasonCode) {
		if !candidate.VerifiedOutputCap {
			return 0, 0, "strict_provider_cap", ReasonStrictCapUnverified
		}
		return capTokens, 0, "strict_provider_cap", ""
	}
	if !req.InputTokensExact && method != "strict_provider_cap" {
		return strict()
	}
	value := func(key string) (int64, bool) {
		v, err := strconv.ParseInt(strings.TrimSpace(routing.Annotations[key]), 10, 64)
		return v, err == nil && v >= 0
	}
	clamp := func(v int64) int64 {
		if v > capTokens {
			return capTokens
		}
		return v
	}
	switch method {
	case "strict_provider_cap":
		return strict()
	case "mean":
		v, ok := value(AnnotationMeanOutputTokens)
		if !ok {
			return 0, 0, method, ReasonInsufficientCalibration
		}
		return clamp(v), 0, method, ""
	case "fixed_margin":
		mean, okMean := value(AnnotationMeanOutputTokens)
		margin, okMargin := value(AnnotationMarginTokens)
		if !okMean || !okMargin || mean > capTokens-margin {
			return 0, 0, method, ReasonInsufficientCalibration
		}
		return clamp(mean + margin), 0, method, ""
	case "fixed_quantile":
		v, ok := value(AnnotationQuantileTokens)
		if !ok {
			return 0, 0, method, ReasonInsufficientCalibration
		}
		return clamp(v), 0, method, ""
	case "adaptive_quantile", "govar_fixed_cohort":
		support, okSupport := value(AnnotationCalibrationSupport)
		adaptive, okAdaptive := value(AnnotationAdaptiveTokens)
		if !okSupport || !okAdaptive || support < minimumCalibrationSupport || strings.EqualFold(routing.Annotations[AnnotationCalibrationDrift], "true") {
			return strict()
		}
		if method == "adaptive_quantile" {
			return clamp(adaptive), 0, method, ""
		}
		cohort, okCohort := value(AnnotationCohortSize)
		risk, okRisk := value(AnnotationTenantRiskPPB)
		if !okCohort || cohort <= 0 || !okRisk || risk > 1_000_000_000 || req.CohortID == "" || req.CohortIndex < 0 || req.CohortIndex >= cohort {
			return 0, 0, method, ReasonInsufficientCalibration
		}
		return clamp(adaptive), risk / cohort, method, ""
	default:
		return 0, 0, method, ReasonReservationMethodUnknown
	}
}

var errBudgetWindowChanged = errors.New("budget policy or window changed while liabilities remain")
