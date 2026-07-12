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
	"k8s.io/apimachinery/pkg/types"
)

// ComputeDigest returns the canonical proposal digest without trusting the
// stored RequestDigest field itself.
func (r AIAdmissionApprovalRequest) ComputeDigest() string {
	r.RequestDigest = ""
	raw, _ := json.Marshal(r)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// AdmissionApprovalConsumptionName is the deterministic Lease name used as
// the atomic, one-use consumption marker for an approved proposal UID.
func AdmissionApprovalConsumptionName(uid types.UID) string {
	digest := sha256.Sum256([]byte(uid))
	return "govar-consumed-" + hex.EncodeToString(digest[:12])
}

// AIAdmissionApprovalRequest is the immutable input to a one-request approval.
// It never authorizes budget/debt adjustment or a policy mutation.
type AIAdmissionApprovalRequest struct {
	RequestID                string                             `json:"requestID"`
	Namespace                string                             `json:"namespace"`
	TenantID                 string                             `json:"tenantID"`
	WorkloadUID              string                             `json:"workloadUID"`
	Team                     string                             `json:"team,omitempty"`
	Application              string                             `json:"application,omitempty"`
	BudgetPolicy             AIWorkloadBindingResolvedReference `json:"budgetPolicy"`
	RoutingPolicy            AIWorkloadBindingResolvedReference `json:"routingPolicy"`
	CandidateModelRef        string                             `json:"candidateModelRef"`
	CandidateSnapshotVersion string                             `json:"candidateSnapshotVersion"`
	RouteSnapshot            GOVARRouteSnapshot                 `json:"routeSnapshot"`
	InputTokens              int64                              `json:"inputTokens"`
	InputTokensExact         bool                               `json:"inputTokensExact"`
	MaxOutputTokens          int64                              `json:"maxOutputTokens"`
	SensitiveData            bool                               `json:"sensitiveData"`
	AllowedZones             []string                           `json:"allowedZones,omitempty"`
	CohortID                 string                             `json:"cohortID,omitempty"`
	CohortIndex              int64                              `json:"cohortIndex,omitempty"`
	ExpiresAt                metav1.Time                        `json:"expiresAt"`
	// RequestDigest is SHA-256 over all other request, policy, candidate,
	// route, reservation-input, and expiry fields in canonical struct order.
	// +kubebuilder:validation:Pattern=`^[a-f0-9]{64}$`
	RequestDigest string `json:"requestDigest"`
}

type AIAdmissionApprovalSpec struct {
	Request AIAdmissionApprovalRequest `json:"request"`
}

// AIAdmissionApprovalPhase is controller-derived; proposal authors cannot set it.
// +kubebuilder:validation:Enum=Pending;Approved;Consumed;Rejected;Expired;Failed
type AIAdmissionApprovalPhase string

const (
	AIAdmissionApprovalPhasePending  AIAdmissionApprovalPhase = "Pending"
	AIAdmissionApprovalPhaseApproved AIAdmissionApprovalPhase = "Approved"
	AIAdmissionApprovalPhaseConsumed AIAdmissionApprovalPhase = "Consumed"
	AIAdmissionApprovalPhaseRejected AIAdmissionApprovalPhase = "Rejected"
	AIAdmissionApprovalPhaseExpired  AIAdmissionApprovalPhase = "Expired"
	AIAdmissionApprovalPhaseFailed   AIAdmissionApprovalPhase = "Failed"
)

// AIAdmissionApprovalStatus is verified and written only by the controller.
type AIAdmissionApprovalStatus struct {
	ObservedGeneration         int64                    `json:"observedGeneration,omitempty"`
	Phase                      AIAdmissionApprovalPhase `json:"phase,omitempty"`
	Message                    string                   `json:"message,omitempty"`
	RequestDigest              string                   `json:"requestDigest,omitempty"`
	DecisionResourceUID        types.UID                `json:"decisionResourceUID,omitempty"`
	DecisionResourceGeneration int64                    `json:"decisionResourceGeneration,omitempty"`
	ApprovedAt                 *metav1.Time             `json:"approvedAt,omitempty"`
	ConsumedAt                 *metav1.Time             `json:"consumedAt,omitempty"`
	ConsumedByRequestID        string                   `json:"consumedByRequestID,omitempty"`
	Conditions                 []metav1.Condition       `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=aiaa
//+kubebuilder:validation:XValidation:rule="self.spec == oldSelf.spec",message="admission approval proposal spec is immutable"
//+kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.request.tenantID`
//+kubebuilder:printcolumn:name="Request",type=string,JSONPath=`.spec.request.requestID`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// AIAdmissionApproval is an immutable proposal created by the admission
// service. It contains no human decision field.
type AIAdmissionApproval struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AIAdmissionApprovalSpec   `json:"spec,omitempty"`
	Status            AIAdmissionApprovalStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type AIAdmissionApprovalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIAdmissionApproval `json:"items"`
}

// AIAdmissionApprovalDecisionValue is the immutable human decision.
// +kubebuilder:validation:Enum=Approved;Rejected
type AIAdmissionApprovalDecisionValue string

const (
	AIAdmissionApprovalDecisionApproved AIAdmissionApprovalDecisionValue = "Approved"
	AIAdmissionApprovalDecisionRejected AIAdmissionApprovalDecisionValue = "Rejected"
)

type AIAdmissionApprovalDecisionSpec struct {
	ProposalName  string                           `json:"proposalName"`
	ProposalUID   types.UID                        `json:"proposalUID"`
	RequestDigest string                           `json:"requestDigest"`
	Decision      AIAdmissionApprovalDecisionValue `json:"decision"`
	Reason        string                           `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=aiaad
//+kubebuilder:validation:XValidation:rule="self.metadata.name == self.spec.proposalName",message="decision name must equal proposalName"
//+kubebuilder:validation:XValidation:rule="self.spec == oldSelf.spec",message="admission approval decision spec is immutable"
//+kubebuilder:printcolumn:name="Decision",type=string,JSONPath=`.spec.decision`

// AIAdmissionApprovalDecision is created only by a separately authorized human
// reviewer. The admission service has no permission on this resource.
type AIAdmissionApprovalDecision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AIAdmissionApprovalDecisionSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type AIAdmissionApprovalDecisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIAdmissionApprovalDecision `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&AIAdmissionApproval{}, &AIAdmissionApprovalList{},
		&AIAdmissionApprovalDecision{}, &AIAdmissionApprovalDecisionList{},
	)
}
