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
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/imperium/ai-sovereign-finops-operator/internal/gatewayreroute"
)

// This is enforcement slice 2: the operator actuates sovereignty decisions in the
// real Envoy AI Gateway data plane. An AIGatewayRoute routes by the request's
// `x-ai-eg-model` header to an AIServiceBackend. To enforce "forbidden zone", we
// reroute a forbidden model's rule to the compliant backend AND rewrite the request
// body `model` field to the compliant model (bodyMutation.set) — so the forbidden
// provider is never reached and the request still succeeds on the compliant one.
//
// The change is fully reversible: the original backend name per model is stored on
// the route in an annotation, and reverted when the policy leaves enforce mode (or
// is deleted). We use the unstructured client so the operator needs no compile-time
// dependency on the Envoy AI Gateway Go API.

const rerouteAnnotation = "aiops.imperium.io/enforced-reroutes"

func aigwRouteListGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "aigateway.envoyproxy.io", Version: "v1beta1", Kind: "AIGatewayRouteList"}
}

func listAIGatewayRoutes(ctx context.Context, c client.Client, namespace string) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(aigwRouteListGVK())
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return list, nil
}

// modelBackends maps each routed model name to the backend currently serving it,
// across all AIGatewayRoutes in the namespace. Reroute targets resolve their
// backend through this index (the compliant model's real backend).
func modelBackends(ctx context.Context, c client.Client, namespace string) (map[string]string, error) {
	routes, err := listAIGatewayRoutes(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i := range routes.Items {
		rules, _, _ := unstructured.NestedSlice(routes.Items[i].Object, "spec", "rules")
		for _, ru := range rules {
			rule, ok := ru.(map[string]any)
			if !ok {
				continue
			}
			model := gatewayreroute.RuleModel(rule)
			if model == "" {
				continue
			}
			brs, _, _ := unstructured.NestedSlice(rule, "backendRefs")
			if len(brs) == 0 {
				continue
			}
			if b0, ok := brs[0].(map[string]any); ok {
				// Ignore a backend that is itself a stored-original reroute target; we
				// want the genuine serving backend, which modelBackends sees for the
				// target model's own (untouched) rule.
				if name, _, _ := unstructured.NestedString(b0, "name"); name != "" {
					if _, exists := out[model]; !exists {
						out[model] = name
					}
				}
			}
		}
	}
	return out, nil
}

// actuateReroutes makes the gateway route each forbidden model in `desired`
// (forbiddenModel -> compliant targetModel) to the target's backend, rewriting the
// request body `model` to the target. Models previously rerouted but absent from
// `desired` are reverted to their original backend. Passing an empty/nil `desired`
// reverts everything. Returns the set of models actually actuated at the gateway.
func actuateReroutes(ctx context.Context, c client.Client, namespace string, desired map[string]string) (map[string]bool, error) {
	mb, err := modelBackends(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	routes, err := listAIGatewayRoutes(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	actuated := map[string]bool{}
	for i := range routes.Items {
		route := &routes.Items[i]
		ann := route.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		saved := map[string]string{} // model -> original backend name
		if s := ann[rerouteAnnotation]; s != "" {
			_ = json.Unmarshal([]byte(s), &saved)
		}
		rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
		// The actual rule mutation lives in the pure, unit-tested gatewayreroute
		// package; here we only handle I/O (read annotation, persist the route).
		routeActuated, changed := gatewayreroute.Apply(rules, saved, desired, mb)
		for m := range routeActuated {
			actuated[m] = true
		}
		if !changed {
			continue
		}
		_ = unstructured.SetNestedSlice(route.Object, rules, "spec", "rules")
		if len(saved) == 0 {
			delete(ann, rerouteAnnotation)
		} else {
			b, _ := json.Marshal(saved)
			ann[rerouteAnnotation] = string(b)
		}
		route.SetAnnotations(ann)
		if err := c.Update(ctx, route); err != nil {
			return actuated, err
		}
	}
	return actuated, nil
}
