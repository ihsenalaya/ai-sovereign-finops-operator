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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tier is a coarse low/medium/high ranking used for quality and cost.
// +kubebuilder:validation:Enum=low;medium;high
type Tier string

const (
	TierLow    Tier = "low"
	TierMedium Tier = "medium"
	TierHigh   Tier = "high"
)

// AIModelSpec defines the desired state of AIModel.
type AIModelSpec struct {
	// ProviderRef is the name of the AIProvider serving this model (same namespace).
	// +kubebuilder:validation:MinLength=1
	ProviderRef string `json:"providerRef"`

	// ModelName is the provider-side model identifier (e.g. gpt-4o, mistral-small).
	// +kubebuilder:validation:MinLength=1
	ModelName string `json:"modelName"`

	// Type categorizes the model.
	// +kubebuilder:validation:Enum=llm;embedding;reranker;vision;audio
	// +kubebuilder:default=llm
	Type string `json:"type"`

	// ContextWindow is the maximum context length in tokens.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ContextWindow int32 `json:"contextWindow,omitempty"`

	// QualityTier ranks output quality.
	// +optional
	QualityTier Tier `json:"qualityTier,omitempty"`

	// CostTier ranks relative cost.
	// +optional
	CostTier Tier `json:"costTier,omitempty"`

	// SensitiveDataAllowed states whether this model may process sensitive data.
	// +optional
	SensitiveDataAllowed bool `json:"sensitiveDataAllowed,omitempty"`
}

// AIModelStatus defines the observed state of AIModel.
type AIModelStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ResolvedProvider echoes the provider type once ProviderRef is resolved.
	// +optional
	ResolvedProvider string `json:"resolvedProvider,omitempty"`

	// Conditions represent the latest available observations of the model state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.providerRef`
//+kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelName`
//+kubebuilder:printcolumn:name="CostTier",type=string,JSONPath=`.spec.costTier`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AIModel catalogs an AI model available through a provider.
type AIModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIModelSpec   `json:"spec,omitempty"`
	Status AIModelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIModelList contains a list of AIModel.
type AIModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIModel{}, &AIModelList{})
}
