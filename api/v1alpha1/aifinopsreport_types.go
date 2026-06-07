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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReportTarget scopes a report.
type ReportTarget struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Period the report covers.
	// +kubebuilder:validation:Enum=daily;weekly;monthly
	// +kubebuilder:default=monthly
	Period string `json:"period"`
}

// Severity ranks a finding.
// +kubebuilder:validation:Enum=info;warning;critical
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// ModelCost is the cost attributed to a single model.
type ModelCost struct {
	Name string `json:"name"`
	// CostEUR is the cost attributed to this model over the period.
	CostEUR resource.Quantity `json:"costEUR"`
}

// SovereigntyFinding is a sovereignty/compliance observation. When produced by
// flow-aware evaluation it is attributed to the namespace/application/model whose
// traffic triggered it, on which provider/zone, and how many requests were affected.
type SovereigntyFinding struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Namespace is the governed namespace whose flow triggered the finding.
	Namespace string `json:"namespace,omitempty"`
	// Application is the app whose flow triggered the finding.
	Application string `json:"application,omitempty"`
	// Model is the provider-side model used by the flow.
	Model string `json:"model,omitempty"`
	// Provider is the provider the flow was routed to.
	Provider string `json:"provider,omitempty"`
	// Zone is the resolved provider residency zone.
	Zone string `json:"zone,omitempty"`
	// Requests is the number of requests affected by this finding.
	Requests int64 `json:"requests,omitempty"`
}

// Recommendation is an optimization suggestion surfaced by the engines.
type Recommendation struct {
	// Type categorizes the recommendation (cost-saving, sovereignty, data-quality, ...).
	Type    string `json:"type"`
	Message string `json:"message"`
	// Severity ranks urgency (info, warning, critical).
	// +optional
	Severity string `json:"severity,omitempty"`
	// EstimatedSavingsEUR is the projected saving of acting on this recommendation
	// over the observation window (0 when not a cost saving).
	// +optional
	EstimatedSavingsEUR *resource.Quantity `json:"estimatedSavingsEUR,omitempty"`
}

// AIFinOpsReportSpec defines the desired state of AIFinOpsReport.
type AIFinOpsReportSpec struct {
	// Target scopes the report.
	Target ReportTarget `json:"target"`

	// GatewayRef optionally pins the report to a specific AIGateway.
	// +optional
	GatewayRef string `json:"gatewayRef,omitempty"`
}

// AIFinOpsReportStatus holds the generated report results.
type AIFinOpsReportStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// GeneratedAt is when this report was last produced.
	// +optional
	GeneratedAt *metav1.Time `json:"generatedAt,omitempty"`

	// TotalCostEUR is the total spend actually observed over the period.
	// +optional
	TotalCostEUR *resource.Quantity `json:"totalCostEUR,omitempty"`

	// ProjectedMonthlyCostEUR forecasts a full month from the observed spend using
	// a run-rate: observed x (30.4 / observation-window-days). For a monthly period
	// it equals TotalCostEUR.
	// +optional
	ProjectedMonthlyCostEUR *resource.Quantity `json:"projectedMonthlyCostEUR,omitempty"`

	// TotalInputTokens over the period.
	// +optional
	TotalInputTokens int64 `json:"totalInputTokens,omitempty"`

	// TotalOutputTokens over the period.
	// +optional
	TotalOutputTokens int64 `json:"totalOutputTokens,omitempty"`

	// TopModels ranks models by cost.
	// +optional
	TopModels []ModelCost `json:"topModels,omitempty"`

	// SovereigntyFindings surfaced during the period.
	// +optional
	SovereigntyFindings []SovereigntyFinding `json:"sovereigntyFindings,omitempty"`

	// Recommendations surfaced by the engines.
	// +optional
	Recommendations []Recommendation `json:"recommendations,omitempty"`

	// Conditions represent the latest available observations of the report state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=aireport
//+kubebuilder:printcolumn:name="Namespace-Target",type=string,JSONPath=`.spec.target.namespace`
//+kubebuilder:printcolumn:name="Period",type=string,JSONPath=`.spec.target.period`
//+kubebuilder:printcolumn:name="TotalCost(EUR)",type=string,JSONPath=`.status.totalCostEUR`
//+kubebuilder:printcolumn:name="Generated",type=date,JSONPath=`.status.generatedAt`

// AIFinOpsReport is a generated FinOps & sovereignty report. Results live in
// .status; export to Markdown/JSON is handled out-of-band by the reporting package.
type AIFinOpsReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIFinOpsReportSpec   `json:"spec,omitempty"`
	Status AIFinOpsReportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIFinOpsReportList contains a list of AIFinOpsReport.
type AIFinOpsReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIFinOpsReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIFinOpsReport{}, &AIFinOpsReportList{})
}
