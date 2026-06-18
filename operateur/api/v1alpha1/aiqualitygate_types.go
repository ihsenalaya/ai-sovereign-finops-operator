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

// ConfigMapDataReference points to one key in a ConfigMap. Namespace is optional
// and defaults to the owning resource namespace.
type ConfigMapDataReference struct {
	// Name of the ConfigMap.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace of the ConfigMap. Defaults to the owning resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key inside the ConfigMap. Defaults are resource-specific.
	// +optional
	Key string `json:"key,omitempty"`
}

// AIQualityRequiredChecks configures the deterministic and operational gates a
// candidate model must satisfy before it can be considered safe for an app.
type AIQualityRequiredChecks struct {
	// SchemaValid requires golden-run evidence that candidate responses match the expected schema.
	// +optional
	SchemaValid bool `json:"schemaValid,omitempty"`

	// NoUnexpectedRefusal requires evidence that the candidate did not refuse valid prompts.
	// +optional
	NoUnexpectedRefusal bool `json:"noUnexpectedRefusal,omitempty"`

	// NoSensitiveDataLeak requires evidence that candidate responses did not leak sensitive data.
	// +optional
	NoSensitiveDataLeak bool `json:"noSensitiveDataLeak,omitempty"`

	// RequiredKeywords requires evidence that expected domain terms are present.
	// +optional
	RequiredKeywords []string `json:"requiredKeywords,omitempty"`

	// MaxErrorRatePercent is the maximum observed candidate HTTP error rate.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxErrorRatePercent int32 `json:"maxErrorRatePercent,omitempty"`

	// MaxLatencyIncreasePercent is the maximum candidate latency increase versus the source model.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxLatencyIncreasePercent int32 `json:"maxLatencyIncreasePercent,omitempty"`

	// MaxRetryIncreasePercent is reserved for retry telemetry when available.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxRetryIncreasePercent int32 `json:"maxRetryIncreasePercent,omitempty"`
}

// AIQualityCanarySpec describes the intended canary before a complete reroute.
// The first implementation records the desired canary contract; traffic actuation
// is handled by routing work planned separately.
type AIQualityCanarySpec struct {
	// Enabled requires canary validation before a full reroute.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Percent of traffic intended for the candidate model during canary.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Percent int32 `json:"percent,omitempty"`

	// Duration of the canary window, for example "30m".
	// +optional
	Duration string `json:"duration,omitempty"`
}

// AIQualityRollbackSpec describes rollback thresholds after a candidate is enabled.
type AIQualityRollbackSpec struct {
	// Enabled allows automatic rollback once routing actuation is wired.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// OnErrorRatePercent is the error-rate threshold that should trigger rollback.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	OnErrorRatePercent int32 `json:"onErrorRatePercent,omitempty"`

	// OnLatencyIncreasePercent is the latency regression threshold that should trigger rollback.
	// +optional
	// +kubebuilder:validation:Minimum=0
	OnLatencyIncreasePercent int32 `json:"onLatencyIncreasePercent,omitempty"`
}

// AIQualityGateTarget identifies exactly one application protected by a quality gate.
type AIQualityGateTarget struct {
	// Namespace is the application namespace.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// Application is the application label/value in telemetry samples.
	// +kubebuilder:validation:MinLength=1
	Application string `json:"application"`

	// Team optionally narrows telemetry to one team when the same app name is shared.
	// +optional
	Team string `json:"team,omitempty"`
}

// AIQualityGateSpec defines the desired quality validation for one application.
type AIQualityGateSpec struct {
	// Target is the application this gate protects. The intended granularity is
	// one AIQualityGate per application.
	Target AIQualityGateTarget `json:"target"`

	// SourceModel is the current provider-side model identifier.
	// +kubebuilder:validation:MinLength=1
	SourceModel string `json:"sourceModel"`

	// CandidateModel is the provider-side model being evaluated as a replacement.
	// +kubebuilder:validation:MinLength=1
	CandidateModel string `json:"candidateModel"`

	// GoldenDatasetRef points to the ConfigMap holding prompts.yaml/prompts.json.
	GoldenDatasetRef ConfigMapDataReference `json:"goldenDatasetRef"`

	// EvidenceRef optionally points to deterministic golden-run results produced
	// by a CI job or future prompt runner. It is required when response-level
	// checks such as schemaValid or requiredKeywords are enabled.
	// +optional
	EvidenceRef *ConfigMapDataReference `json:"evidenceRef,omitempty"`

	// GatewayRef optionally pins telemetry to a specific AIGateway.
	// +optional
	GatewayRef string `json:"gatewayRef,omitempty"`

	// Period is the telemetry window used for operational checks.
	// +kubebuilder:validation:Enum=daily;weekly;monthly
	// +kubebuilder:default=monthly
	// +optional
	Period string `json:"period,omitempty"`

	// RequiredChecks configures the deterministic and operational guardrails.
	// +optional
	RequiredChecks AIQualityRequiredChecks `json:"requiredChecks,omitempty"`

	// Canary configures the desired canary validation.
	// +optional
	Canary AIQualityCanarySpec `json:"canary,omitempty"`

	// Rollback configures rollback thresholds.
	// +optional
	Rollback AIQualityRollbackSpec `json:"rollback,omitempty"`
}

// AIQualityGatePhase is the high-level status of a quality gate.
// +kubebuilder:validation:Enum=Pending;Passed;Failed
type AIQualityGatePhase string

const (
	AIQualityGatePending AIQualityGatePhase = "Pending"
	AIQualityGatePassed  AIQualityGatePhase = "Passed"
	AIQualityGateFailed  AIQualityGatePhase = "Failed"
)

// AIQualityModelObservation summarizes live telemetry for one model in the target app.
type AIQualityModelObservation struct {
	// Model is the provider-side model identifier.
	Model string `json:"model,omitempty"`
	// Requests observed for the target app/model.
	Requests int64 `json:"requests,omitempty"`
	// Errors observed for the target app/model.
	Errors int64 `json:"errors,omitempty"`
	// ErrorRatePercent is Errors / Requests * 100, serialized as a decimal string.
	ErrorRatePercent string `json:"errorRatePercent,omitempty"`
	// ObservedLatencyMillis is the measured mean latency when telemetry exists.
	ObservedLatencyMillis string `json:"observedLatencyMillis,omitempty"`
	// LatencyTelemetryAvailable is true when ObservedLatencyMillis is real telemetry.
	LatencyTelemetryAvailable bool `json:"latencyTelemetryAvailable,omitempty"`
}

// AIQualityGateStatus defines the observed state of AIQualityGate.
type AIQualityGateStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase summarizes the gate result.
	// +optional
	Phase AIQualityGatePhase `json:"phase,omitempty"`

	// Verdict is a stable machine-readable decision: candidate-safe,
	// candidate-risk or insufficient-data.
	// +optional
	Verdict string `json:"verdict,omitempty"`

	// CheckedSamples is the number of golden dataset prompts parsed.
	// +optional
	CheckedSamples int32 `json:"checkedSamples,omitempty"`

	// FailedChecks is the number of failed deterministic or operational checks.
	// +optional
	FailedChecks int32 `json:"failedChecks,omitempty"`

	// FailureMessages explains failed or missing checks for human review.
	// +optional
	FailureMessages []string `json:"failureMessages,omitempty"`

	// CanaryStatus summarizes the canary contract state.
	// +optional
	CanaryStatus string `json:"canaryStatus,omitempty"`

	// SourceObservation is the observed source model telemetry for the target app.
	// +optional
	SourceObservation *AIQualityModelObservation `json:"sourceObservation,omitempty"`

	// CandidateObservation is the observed candidate model telemetry for the target app.
	// +optional
	CandidateObservation *AIQualityModelObservation `json:"candidateObservation,omitempty"`

	// Conditions represent the latest available observations of the gate state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=aiqgate
//+kubebuilder:printcolumn:name="TargetNS",type=string,JSONPath=`.spec.target.namespace`
//+kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.target.application`
//+kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceModel`
//+kubebuilder:printcolumn:name="Candidate",type=string,JSONPath=`.spec.candidateModel`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Verdict",type=string,JSONPath=`.status.verdict`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AIQualityGate validates whether a candidate model is safe enough for one app.
type AIQualityGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIQualityGateSpec   `json:"spec,omitempty"`
	Status AIQualityGateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIQualityGateList contains a list of AIQualityGate.
type AIQualityGateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIQualityGate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIQualityGate{}, &AIQualityGateList{})
}
