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

package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors/fake"
	promcollector "github.com/imperium/ai-sovereign-finops-operator/internal/collectors/prometheus"
	"github.com/imperium/ai-sovereign-finops-operator/internal/costengine"
	"github.com/imperium/ai-sovereign-finops-operator/internal/sovereigntyengine"
)

// catalog holds the cluster-side model/provider definitions needed by the engines.
type catalog struct {
	providers map[string]aiopsv1alpha1.AIProvider // by metadata.name
	models    []aiopsv1alpha1.AIModel
}

// loadCatalog lists AIProviders and AIModels in the given namespace.
func loadCatalog(ctx context.Context, c client.Client, namespace string) (catalog, error) {
	var provList aiopsv1alpha1.AIProviderList
	if err := c.List(ctx, &provList, client.InNamespace(namespace)); err != nil {
		return catalog{}, err
	}
	var modelList aiopsv1alpha1.AIModelList
	if err := c.List(ctx, &modelList, client.InNamespace(namespace)); err != nil {
		return catalog{}, err
	}
	cat := catalog{providers: map[string]aiopsv1alpha1.AIProvider{}, models: modelList.Items}
	for i := range provList.Items {
		cat.providers[provList.Items[i].Name] = provList.Items[i]
	}
	return cat, nil
}

// priceBook builds a costengine.PriceBook keyed by model name, resolving each
// AIModel to its provider's pricing.
func (cat catalog) priceBook() costengine.PriceBook {
	pb := costengine.PriceBook{}
	for i := range cat.models {
		m := cat.models[i]
		prov, ok := cat.providers[m.Spec.ProviderRef]
		if !ok {
			continue
		}
		pb[m.Spec.ModelName] = costengine.TokenPricing{
			Currency:         orDefault(prov.Spec.Pricing.Currency, "EUR"),
			InputPerMillion:  prov.Spec.Pricing.InputTokenPricePerMillion.AsApproximateFloat64(),
			OutputPerMillion: prov.Spec.Pricing.OutputTokenPricePerMillion.AsApproximateFloat64(),
		}
	}
	return pb
}

// modelByName returns the AIModel with the given metadata name.
func (cat catalog) modelByName(name string) (aiopsv1alpha1.AIModel, bool) {
	for i := range cat.models {
		if cat.models[i].Name == name {
			return cat.models[i], true
		}
	}
	return aiopsv1alpha1.AIModel{}, false
}

// providerFixedMonthly returns the provider's flat monthly fee (0 if unset).
func (cat catalog) providerFixedMonthly(providerRef string) float64 {
	if p, ok := cat.providers[providerRef]; ok && p.Spec.Pricing.FixedMonthlyCost != nil {
		return p.Spec.Pricing.FixedMonthlyCost.AsApproximateFloat64()
	}
	return 0
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// collectorFor returns the TelemetryCollector implied by a gateway's telemetry
// mode. The MVP defaults to the fake collector when no gateway is found or the
// mode is fake, so reports are always demonstrable without a live gateway.
func collectorFor(gw *aiopsv1alpha1.AIGateway) collectors.TelemetryCollector {
	if gw == nil {
		return fake.New()
	}
	switch gw.Spec.Telemetry.Mode {
	case aiopsv1alpha1.TelemetryModePrometheus:
		endpoint := gw.Spec.Endpoint + gw.Spec.Telemetry.MetricsEndpoint
		return promcollector.New(endpoint)
	default:
		return fake.New()
	}
}

// filterByNamespace keeps only samples for the target namespace (all if empty).
func filterByNamespace(samples []collectors.UsageSample, namespace string) []collectors.UsageSample {
	if namespace == "" {
		return samples
	}
	out := samples[:0:0]
	for _, s := range samples {
		if s.Namespace == namespace {
			out = append(out, s)
		}
	}
	return out
}

// filterByBudgetTarget keeps samples matching every non-empty target dimension.
func filterByBudgetTarget(samples []collectors.UsageSample, t aiopsv1alpha1.BudgetTarget) []collectors.UsageSample {
	out := samples[:0:0]
	for _, s := range samples {
		if t.Namespace != "" && s.Namespace != t.Namespace {
			continue
		}
		if t.Team != "" && s.Team != t.Team {
			continue
		}
		if t.Application != "" && s.Application != t.Application {
			continue
		}
		out = append(out, s)
	}
	return out
}

// providerInfos returns sovereignty ProviderInfo for each distinct provider seen
// in the samples, resolving residency/managed flags from the catalog. Providers
// absent from the catalog yield an entry with an empty zone (treated as unverified).
func (cat catalog) providerInfos(samples []collectors.UsageSample) []sovereigntyengine.ProviderInfo {
	seen := map[string]bool{}
	var out []sovereigntyengine.ProviderInfo
	for _, s := range samples {
		if s.Provider == "" || seen[s.Provider] {
			continue
		}
		seen[s.Provider] = true
		info := sovereigntyengine.ProviderInfo{Name: s.Provider}
		if p, ok := cat.providers[s.Provider]; ok {
			info.Zone = p.Spec.DataResidency
			info.Managed = p.Spec.Managed
			info.AllowedForSensitiveData = p.Spec.Compliance.AllowedForSensitiveData
		}
		out = append(out, info)
	}
	return out
}

// firstSovereigntyPolicy returns any AISovereigntyPolicy in the namespace, or nil.
func firstSovereigntyPolicy(ctx context.Context, c client.Client, namespace string) *aiopsv1alpha1.AISovereigntyPolicy {
	var list aiopsv1alpha1.AISovereigntyPolicyList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil || len(list.Items) == 0 {
		return nil
	}
	return &list.Items[0]
}

// firstGateway returns any AIGateway in the namespace, or nil if none exists.
func firstGateway(ctx context.Context, c client.Client, namespace string) *aiopsv1alpha1.AIGateway {
	var list aiopsv1alpha1.AIGatewayList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil || len(list.Items) == 0 {
		return nil
	}
	return &list.Items[0]
}
