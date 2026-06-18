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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// AIRoutingPolicyGuardrails configures the minimum quality, latency and
// sovereignty requirements that must hold before any automatic reroute is applied.
type AIRoutingPolicyGuardrails struct {
	// MinQualityScore is the minimum routing score (0–1) a candidate model must
	// reach before it can be automatically selected (default 0.70).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	MinQualityScore float64 `json:"minQualityScore,omitempty"`

	// MaxLatencyMillis is the maximum measured mean latency (ms) allowed for a
	// candidate model. Models above this threshold are rejected.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxLatencyMillis int32 `json:"maxLatencyMillis,omitempty"`

	// RequireSovereigntyCompliance, when true, restricts candidate selection to
	// sovereignty-compliant models only (default true).
	// +optional
	// +kubebuilder:default=true
	RequireSovereigntyCompliance bool `json:"requireSovereigntyCompliance,omitempty"`
}

// AIRoutingPolicyCanary configures a canary phase before full promotion.
type AIRoutingPolicyCanary struct {
	// Enabled requires canary validation before full reroute (default false).
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Percent of traffic to send to the candidate during canary (1–50).
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=50
	Percent int32 `json:"percent,omitempty"`
}

// AIRoutingPolicySpec defines the desired automatic routing behaviour.
type AIRoutingPolicySpec struct {
	// Objective is the primary optimisation dimension.
	// +kubebuilder:validation:Enum=cost;quality;latency
	// +kubebuilder:default=cost
	// +optional
	Objective string `json:"objective,omitempty"`

	// Guardrails defines the minimum bar that any candidate must clear.
	// +optional
	Guardrails AIRoutingPolicyGuardrails `json:"guardrails,omitempty"`

	// Canary configures the canary phase (declared intent; actuation is wired
	// through AIChangeRequest approval when human-in-the-loop is active).
	// +optional
	Canary AIRoutingPolicyCanary `json:"canary,omitempty"`
}

// AIRoutingPolicyRecommendation is one candidate model the policy considers safe.
type AIRoutingPolicyRecommendation struct {
	// Application is the workload/model pair evaluated.
	Application string `json:"application,omitempty"`
	// CurrentModel is the model currently used.
	CurrentModel string `json:"currentModel,omitempty"`
	// CandidateModel is the model recommended by the scoring engine.
	CandidateModel string `json:"candidateModel,omitempty"`
	// CandidateScore is the routing score of the candidate (higher is better).
	CandidateScore string `json:"candidateScore,omitempty"`
	// EstimatedSavingsEUR is the projected saving of switching over the observation window.
	EstimatedSavingsEUR string `json:"estimatedSavingsEUR,omitempty"`
	// Blocked is true when the candidate did not pass the guardrails.
	Blocked bool `json:"blocked,omitempty"`
	// BlockReason explains why the candidate was blocked (empty when not blocked).
	BlockReason string `json:"blockReason,omitempty"`
}

// AIRoutingPolicyStatus reflects the observed state of the routing policy.
type AIRoutingPolicyStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Recommendations lists candidates evaluated by the routing engine.
	// +optional
	Recommendations []AIRoutingPolicyRecommendation `json:"recommendations,omitempty"`

	// LastEvaluatedAt is when the routing engine last ran.
	// +optional
	LastEvaluatedAt *metav1.Time `json:"lastEvaluatedAt,omitempty"`

	// Conditions represent the latest available observations of the policy state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=airpolicy
//+kubebuilder:printcolumn:name="Objective",type=string,JSONPath=`.spec.objective`
//+kubebuilder:printcolumn:name="Canary",type=boolean,JSONPath=`.spec.canary.enabled`
//+kubebuilder:printcolumn:name="Recommendations",type=integer,JSONPath=`.status.recommendations`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AIRoutingPolicy continuously evaluates all observed app/model pairs and surfaces
// the best routing candidate according to the multi-criteria routing score.
// Unlike AIRouteOverride (manual), it computes recommendations automatically and
// can create AIChangeRequest objects for human approval before actuation.
type AIRoutingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIRoutingPolicySpec   `json:"spec,omitempty"`
	Status AIRoutingPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIRoutingPolicyList contains a list of AIRoutingPolicy.
type AIRoutingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIRoutingPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIRoutingPolicy{}, &AIRoutingPolicyList{})
}
