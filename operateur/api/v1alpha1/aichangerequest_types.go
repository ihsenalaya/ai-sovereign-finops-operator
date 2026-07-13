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

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AIChangeRequestAction describes what action the operator should take once approved.
// +kubebuilder:validation:Enum=reroute;authorize-gov-ar-route
type AIChangeRequestAction string

const (
	// AIChangeRequestActionReroute asks the operator to reroute traffic from
	// SourceModel to TargetModel in the Envoy AI Gateway data plane.
	AIChangeRequestActionReroute AIChangeRequestAction = "reroute"

	// AIChangeRequestActionAuthorizeGOVARRoute approves a bounded, reusable
	// governance scope. It does not actuate a route and is never created per
	// request. Admission still revalidates the complete live candidate.
	AIChangeRequestActionAuthorizeGOVARRoute AIChangeRequestAction = "authorize-gov-ar-route"
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

// GOVARRouteApprovalScope is a bounded policy-level authorization. Its route
// snapshot digest already commits to the model/provider resource versions,
// pricing version, compliance inputs, and provider-owned actuation fields. The
// explicit references make stale UID/generation checks possible without
// trusting the digest alone.
type GOVARRouteApprovalScope struct {
	RoutingPolicy AIWorkloadBindingResolvedReference `json:"routingPolicy"`
	Model         AIWorkloadBindingResolvedReference `json:"model"`
	Provider      AIWorkloadBindingResolvedReference `json:"provider"`

	// RouteSnapshotDigest is the lower-case SHA-256 digest of the exact
	// controller-validated GOVARRouteSnapshot admitted under this scope.
	// +kubebuilder:validation:Pattern=`^[a-f0-9]{64}$`
	RouteSnapshotDigest string `json:"routeSnapshotDigest"`

	// ValidUntil is an absolute, reviewer-selected upper bound. An approval is
	// not valid at or after this instant.
	ValidUntil metav1.Time `json:"validUntil"`

	// ScopeDigest is SHA-256 over every other field in this scope. Authors use
	// ComputeDigest to populate it; the controller independently recomputes it.
	// +kubebuilder:validation:Pattern=`^[a-f0-9]{64}$`
	ScopeDigest string `json:"scopeDigest"`
}

// ComputeDigest returns a canonical digest without trusting ScopeDigest.
func (s GOVARRouteApprovalScope) ComputeDigest() string {
	s.ScopeDigest = ""
	raw, _ := json.Marshal(s)
	digest := sha256.New()
	_, _ = digest.Write([]byte("govar-route-approval-scope-v1\x00"))
	_, _ = digest.Write(raw)
	return hex.EncodeToString(digest.Sum(nil))
}

// +kubebuilder:validation:XValidation:rule="self.action != 'reroute' || (has(self.sourceModel) && has(self.targetModel))",message="sourceModel and targetModel are required for reroute"
// +kubebuilder:validation:XValidation:rule="self.action != 'authorize-gov-ar-route' || has(self.govarRouteApproval)",message="govarRouteApproval is required for authorize-gov-ar-route"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.govarRouteApproval) || self.govarRouteApproval == oldSelf.govarRouteApproval",message="govarRouteApproval is immutable; create a new change request for a new scope"
// AIChangeRequestSpec describes a proposed change for human review.
type AIChangeRequestSpec struct {
	// Action is the type of change proposed.
	Action AIChangeRequestAction `json:"action"`

	// SourceModel is the current provider-side model id.
	// +optional
	// +kubebuilder:validation:MinLength=1
	SourceModel string `json:"sourceModel,omitempty"`

	// TargetModel is the proposed replacement model id.
	// +optional
	// +kubebuilder:validation:MinLength=1
	TargetModel string `json:"targetModel,omitempty"`

	// GOVARRouteApproval is present only for authorize-gov-ar-route. One
	// approved object may govern many requests under the exact frozen scope;
	// requests never create or consume Kubernetes objects.
	// +optional
	GOVARRouteApproval *GOVARRouteApprovalScope `json:"govarRouteApproval,omitempty"`

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

	// ApprovedAt is when the controller verified a human-approved GOV-AR route scope.
	// +optional
	ApprovedAt *metav1.Time `json:"approvedAt,omitempty"`

	// ApprovedScopeDigest is the controller-recomputed digest of the exact
	// reusable GOV-AR route scope. Admission rejects an absent or stale value.
	// +optional
	// +kubebuilder:validation:Pattern=`^[a-f0-9]{64}$`
	ApprovedScopeDigest string `json:"approvedScopeDigest,omitempty"`

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

// AIChangeRequest represents either a proposed model reroute or a bounded
// policy-level GOV-AR route authorization that requires human approval. The
// workflow is:
//
//	Pending → Approved (human sets spec.approval=Approved) → Actuated
//	Pending → Rejected (human sets spec.approval=Rejected)
//	Pending → Expired  (expiresAfter elapsed without decision)
//
// authorize-gov-ar-route ends in Approved without actuation and may be reused
// only inside its exact immutable scope. Every decision is auditable via status
// conditions and Kubernetes Events.
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
