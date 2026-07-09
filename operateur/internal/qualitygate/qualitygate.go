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

// Package qualitygate holds the pure decision logic for the "no route change
// without adequate evidence" invariant: a residency-driven reroute from a
// forbidden source model to a compliant candidate is allowed only when a
// matching AIQualityGate has a fresh candidate-safe verdict. It is pure (no
// Kubernetes dependency) so it is unit-testable; the sovereignty controller
// supplies the gate summaries.
package qualitygate

import (
	"fmt"
	"time"
)

// VerdictCandidateSafe mirrors qualityengine.VerdictCandidateSafe.
const VerdictCandidateSafe = "candidate-safe"

// GateVerdict is the minimal view of an AIQualityGate needed for the decision.
type GateVerdict struct {
	Name           string
	SourceModel    string
	CandidateModel string
	Verdict        string
	// EvaluatedAt is when the gate last reached its verdict (zero if never).
	EvaluatedAt time.Time
}

// Decision is the outcome of the evidence check for one desired reroute.
type Decision struct {
	Allowed bool
	Reason  string
	// GateName is the gate that authorized the reroute (when Allowed).
	GateName string
}

// EvaluateReroute decides whether a source->candidate reroute may be actuated.
// It requires a matching gate (same source and candidate models) whose verdict
// is candidate-safe; when maxAge > 0 the verdict must also be no older than
// maxAge relative to now. Any other case (candidate-risk, insufficient-data,
// stale, or no matching gate) returns Allowed=false so the caller escalates to a
// human-approved AIChangeRequest instead of actuating automatically.
func EvaluateReroute(gates []GateVerdict, source, candidate string, now time.Time, maxAge time.Duration) Decision {
	var matched *GateVerdict
	for i := range gates {
		g := gates[i]
		if g.SourceModel == source && g.CandidateModel == candidate {
			// Prefer the most recently evaluated matching gate.
			if matched == nil || g.EvaluatedAt.After(matched.EvaluatedAt) {
				matched = &gates[i]
			}
		}
	}
	if matched == nil {
		return Decision{Allowed: false, Reason: fmt.Sprintf(
			"no AIQualityGate found for %s -> %s; escalating to human approval", source, candidate)}
	}
	if matched.Verdict != VerdictCandidateSafe {
		return Decision{Allowed: false, GateName: matched.Name, Reason: fmt.Sprintf(
			"gate %q verdict is %q (not candidate-safe); escalating to human approval",
			matched.Name, matched.Verdict)}
	}
	if maxAge > 0 {
		if matched.EvaluatedAt.IsZero() || now.Sub(matched.EvaluatedAt) > maxAge {
			return Decision{Allowed: false, GateName: matched.Name, Reason: fmt.Sprintf(
				"gate %q candidate-safe verdict is stale (older than %s); escalating to human approval",
				matched.Name, maxAge)}
		}
	}
	return Decision{Allowed: true, GateName: matched.Name, Reason: fmt.Sprintf(
		"gate %q is a fresh candidate-safe verdict", matched.Name)}
}
