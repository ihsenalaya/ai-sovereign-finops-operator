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

// AIChangeRequestAction describes what action the operator should take once approved.
// +kubebuilder:validation:Enum=reroute
type AIChangeRequestAction string

const (
	// AIChangeRequestActionReroute asks the operator to reroute traffic from
	// SourceModel to TargetModel in the Envoy AI Gateway data plane.
	AIChangeRequestActionReroute AIChangeRequestAction = "reroute"
)

// AIChangeRequestApproval is the human decision on a change request.
// +kubebuilder:validation:Enum=Pending;Approved;Rejected
type AIChangeRequestApproval string

const (
	AIChangeRequestApprovalPending  AIChangeRequestApproval = "Pending"
	AIChangeRequestApprovalApproved AIChangeRequestApproval = "Approved"
	AIChangeRequestApprovalRejected AIChangeRequestApproval = "Rejected"
)

// AIChangeRequestPhase is the lifecycle state of the change request.
// +kubebuilder:validation:Enum=Pending;Approved;Actuated;Rejected;Expired;Failed
type AIChangeRequestPhase string

const (
	AIChangeRequestPhasePending  AIChangeRequestPhase = "Pending"
	AIChangeRequestPhaseApproved AIChangeRequestPhase = "Approved"
	AIChangeRequestPhaseActuated AIChangeRequestPhase = "Actuated"
	AIChangeRequestPhaseRejected AIChangeRequestPhase = "Rejected"
	AIChangeRequestPhaseExpired  AIChangeRequestPhase = "Expired"
	AIChangeRequestPhaseFailed   AIChangeRequestPhase = "Failed"
)

// AIChangeRequestSpec describes a proposed change for human review.
type AIChangeRequestSpec struct {
	// Action is the type of change proposed.
	Action AIChangeRequestAction `json:"action"`

	// SourceModel is the current provider-side model id.
	// +kubebuilder:validation:MinLength=1
	SourceModel string `json:"sourceModel"`

	// TargetModel is the proposed replacement model id.
	// +kubebuilder:validation:MinLength=1
	TargetModel string `json:"targetModel"`

	// Reason is a human-readable justification for the change.
	// +optional
	Reason string `json:"reason,omitempty"`

	// ExpectedSavingEUR is the projected saving over the observation window.
	// +optional
	ExpectedSavingEUR string `json:"expectedSavingEUR,omitempty"`

	// QualityScore is the routing score of the target model at proposal time.
	// +optional
	QualityScore string `json:"qualityScore,omitempty"`

	// LatencyImpact is a human-readable estimated latency delta (e.g. "+120ms").
	// +optional
	LatencyImpact string `json:"latencyImpact,omitempty"`

	// RiskLevel is the operator's assessment of the change risk.
	// +kubebuilder:validation:Enum=low;medium;high
	// +optional
	RiskLevel string `json:"riskLevel,omitempty"`

	// ExpiresAfter is how long this request waits for approval before it expires
	// (e.g. "24h"). Defaults to "48h" when not set.
	// +optional
	ExpiresAfter string `json:"expiresAfter,omitempty"`

	// Approval is set by a human reviewer to approve or reject the request.
	// Initially left empty (Pending). The operator acts only when Approved.
	// +kubebuilder:default=Pending
	// +optional
	Approval AIChangeRequestApproval `json:"approval,omitempty"`
}

// AIChangeRequestStatus reflects the observed state of the change request.
type AIChangeRequestStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the current lifecycle state of the request.
	// +optional
	Phase AIChangeRequestPhase `json:"phase,omitempty"`

	// Message is a human-readable description of the current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ActuatedAt is when the operator applied the change.
	// +optional
	ActuatedAt *metav1.Time `json:"actuatedAt,omitempty"`

	// ExpiresAt is when the request will automatically expire if not acted upon.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// ActuatedRoutes lists the AIGatewayRoute names modified by this request.
	// +optional
	ActuatedRoutes []string `json:"actuatedRoutes,omitempty"`

	// Conditions represent the latest available observations of the request state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=aicrq
//+kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
//+kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceModel`
//+kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetModel`
//+kubebuilder:printcolumn:name="Approval",type=string,JSONPath=`.spec.approval`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Risk",type=string,JSONPath=`.spec.riskLevel`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AIChangeRequest represents a proposed model routing change that requires
// human approval before the operator actuates it. The workflow is:
//
//	Pending → Approved (human sets spec.approval=Approved) → Actuated
//	Pending → Rejected (human sets spec.approval=Rejected)
//	Pending → Expired  (expiresAfter elapsed without decision)
//
// Every decision is auditable via status conditions and Kubernetes Events.
type AIChangeRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIChangeRequestSpec   `json:"spec,omitempty"`
	Status AIChangeRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIChangeRequestList contains a list of AIChangeRequest.
type AIChangeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIChangeRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIChangeRequest{}, &AIChangeRequestList{})
}
