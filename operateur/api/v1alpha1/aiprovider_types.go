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

// ProviderPricing captures the unit economics of a provider. Monetary values use
// resource.Quantity so decimal amounts (e.g. "2.5") are stored exactly and are
// idiomatic to the Kubernetes API (no IEEE-754 floats in the schema).
type ProviderPricing struct {
	// Currency is the ISO 4217 code for all prices below.
	// +kubebuilder:default=EUR
	Currency string `json:"currency"`

	// InputTokenPricePerMillion is the price per 1,000,000 input tokens.
	InputTokenPricePerMillion resource.Quantity `json:"inputTokenPricePerMillion"`

	// OutputTokenPricePerMillion is the price per 1,000,000 output tokens.
	OutputTokenPricePerMillion resource.Quantity `json:"outputTokenPricePerMillion"`

	// FixedMonthlyCost is any flat monthly fee for the provider (commitments,
	// reserved capacity, ...). Used by the break-even engine.
	// +optional
	FixedMonthlyCost *resource.Quantity `json:"fixedMonthlyCost,omitempty"`
}

// ProviderCompliance describes sovereignty/compliance attributes of a provider.
type ProviderCompliance struct {
	// AllowedForSensitiveData states whether sensitive data may be sent here.
	// +optional
	AllowedForSensitiveData bool `json:"allowedForSensitiveData,omitempty"`

	// AllowedCountries lists ISO country/zone codes (e.g. FR, EU) this provider serves from.
	// +optional
	AllowedCountries []string `json:"allowedCountries,omitempty"`
}

// AIProviderSpec defines the desired state of AIProvider.
type AIProviderSpec struct {
	// Type identifies the provider family.
	// +kubebuilder:validation:Enum=openai;azure-openai;mistral;anthropic;bedrock;vertex;self-hosted;custom
	Type string `json:"type"`

	// Region is the cloud/provider region (e.g. francecentral).
	// +optional
	Region string `json:"region,omitempty"`

	// DataResidency is the country/zone where data is processed (e.g. france, eu, us).
	// +optional
	DataResidency string `json:"dataResidency,omitempty"`

	// Managed distinguishes a managed API provider (true) from self-hosted GPU (false).
	// +optional
	Managed bool `json:"managed,omitempty"`

	// Pricing holds the unit economics for cost calculations.
	Pricing ProviderPricing `json:"pricing"`

	// Compliance describes sovereignty attributes.
	// +optional
	Compliance ProviderCompliance `json:"compliance,omitempty"`
}

// AIProviderStatus defines the observed state of AIProvider.
type AIProviderStatus struct {
	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the provider state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=aiprov
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
//+kubebuilder:printcolumn:name="Residency",type=string,JSONPath=`.spec.dataResidency`
//+kubebuilder:printcolumn:name="Managed",type=boolean,JSONPath=`.spec.managed`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AIProvider represents a model provider (managed API or self-hosted GPU) along
// with its pricing and sovereignty attributes.
type AIProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIProviderSpec   `json:"spec,omitempty"`
	Status AIProviderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIProviderList contains a list of AIProvider.
type AIProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AIProvider{}, &AIProviderList{})
}
