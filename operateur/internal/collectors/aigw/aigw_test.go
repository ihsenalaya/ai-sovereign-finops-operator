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

package aigw

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
)

// A realistic-enough Envoy AI Gateway exposition: the gen_ai_client_token_usage
// histogram, one series per (model, token_type, namespace, app). The collector
// reads each series' _sum (token total) and _count (request count).
const sampleExposition = `# HELP gen_ai_client_token_usage Measured token usage.
# TYPE gen_ai_client_token_usage histogram
# mistral-large, marketing/content-writer carried by request headers
gen_ai_client_token_usage_bucket{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="marketing",k8s_app="content-writer",le="+Inf"} 10
gen_ai_client_token_usage_sum{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="marketing",k8s_app="content-writer"} 5000
gen_ai_client_token_usage_count{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="marketing",k8s_app="content-writer"} 10
gen_ai_client_token_usage_bucket{gen_ai_request_model="mistral-large",gen_ai_token_type="output",k8s_namespace="marketing",k8s_app="content-writer",le="+Inf"} 10
gen_ai_client_token_usage_sum{gen_ai_request_model="mistral-large",gen_ai_token_type="output",k8s_namespace="marketing",k8s_app="content-writer"} 2000
gen_ai_client_token_usage_count{gen_ai_request_model="mistral-large",gen_ai_token_type="output",k8s_namespace="marketing",k8s_app="content-writer"} 10
# same model, DIFFERENT namespace via headers — must stay separated
gen_ai_client_token_usage_bucket{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="sales",k8s_app="lead-bot",le="+Inf"} 3
gen_ai_client_token_usage_sum{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="sales",k8s_app="lead-bot"} 600
gen_ai_client_token_usage_count{gen_ai_request_model="mistral-large",gen_ai_token_type="input",k8s_namespace="sales",k8s_app="lead-bot"} 3
# gpt-4o, NO headers — attribution falls back to the AIModel catalog (serves*)
gen_ai_client_token_usage_bucket{gen_ai_request_model="gpt-4o",gen_ai_token_type="input",le="+Inf"} 4
gen_ai_client_token_usage_sum{gen_ai_request_model="gpt-4o",gen_ai_token_type="input"} 800
gen_ai_client_token_usage_count{gen_ai_request_model="gpt-4o",gen_ai_token_type="input"} 4
# cached token type must be ignored (not input/output)
gen_ai_client_token_usage_bucket{gen_ai_request_model="gpt-4o",gen_ai_token_type="cache_read_input",le="+Inf"} 4
gen_ai_client_token_usage_sum{gen_ai_request_model="gpt-4o",gen_ai_token_type="cache_read_input"} 9999
gen_ai_client_token_usage_count{gen_ai_request_model="gpt-4o",gen_ai_token_type="cache_read_input"} 4
# Real Envoy AI Gateway duration histogram, seconds.
# TYPE gen_ai_server_request_duration_seconds histogram
gen_ai_server_request_duration_seconds_bucket{gen_ai_request_model="mistral-large",k8s_namespace="marketing",k8s_app="content-writer",le="+Inf"} 10
gen_ai_server_request_duration_seconds_sum{gen_ai_request_model="mistral-large",k8s_namespace="marketing",k8s_app="content-writer"} 12.5
gen_ai_server_request_duration_seconds_count{gen_ai_request_model="mistral-large",k8s_namespace="marketing",k8s_app="content-writer"} 10
gen_ai_server_request_duration_seconds_bucket{gen_ai_request_model="gpt-4o",le="+Inf"} 4
gen_ai_server_request_duration_seconds_sum{gen_ai_request_model="gpt-4o"} 3.2
gen_ai_server_request_duration_seconds_count{gen_ai_request_model="gpt-4o"} 4
`

func newFakeClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := aiopsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	models := []client.Object{
		&aiopsv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{Name: "mistral-large", Namespace: "ns"},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef: "mistral-eu", ModelName: "mistral-large", Type: "llm",
			},
		},
		&aiopsv1alpha1.AIModel{
			ObjectMeta: metav1.ObjectMeta{Name: "gpt-4o", Namespace: "ns"},
			Spec: aiopsv1alpha1.AIModelSpec{
				ProviderRef: "openai", ModelName: "gpt-4o", Type: "llm",
				ServesNamespace: "finance", ServesApplication: "risk", ServesTeam: "finance-team",
			},
		},
	}
	return fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(models...).Build()
}

func TestCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleExposition))
	}))
	defer srv.Close()

	c := New(newFakeClient(t), "ns", srv.URL)
	samples, err := c.Collect(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	byKey := map[string]collectors.UsageSample{}
	for _, s := range samples {
		byKey[s.Namespace+"/"+s.Application+"/"+s.Model] = s
	}
	if len(samples) != 3 {
		t.Fatalf("got %d samples, want 3: %+v", len(samples), samples)
	}

	// Header-attributed: real namespace/app, provider+from catalog, team follows ns.
	mk := byKey["marketing/content-writer/mistral-large"]
	if mk.InputTokens != 5000 || mk.OutputTokens != 2000 {
		t.Errorf("mistral-large tokens = in %d/out %d, want 5000/2000", mk.InputTokens, mk.OutputTokens)
	}
	if mk.Requests != 10 {
		t.Errorf("mistral-large requests = %d, want 10 (histogram _count)", mk.Requests)
	}
	if mk.Provider != "mistral-eu" {
		t.Errorf("mistral-large provider = %q, want mistral-eu (from catalog)", mk.Provider)
	}
	if mk.Team != "marketing" {
		t.Errorf("mistral-large team = %q, want marketing (follows header namespace)", mk.Team)
	}
	if math.Abs(mk.LatencyMillis-1250) > 0.001 {
		t.Errorf("mistral-large latency = %.0fms, want 1250ms", mk.LatencyMillis)
	}

	// Same model, second namespace stays separated (shared-model attribution).
	if s := byKey["sales/lead-bot/mistral-large"]; s.InputTokens != 600 || s.Requests != 3 {
		t.Errorf("sales mistral-large = in %d req %d, want 600/3", s.InputTokens, s.Requests)
	}

	// No-header traffic falls back to the catalog serves* defaults.
	gk := byKey["finance/risk/gpt-4o"]
	if gk.InputTokens != 800 || gk.Requests != 4 {
		t.Errorf("gpt-4o = in %d req %d, want 800/4", gk.InputTokens, gk.Requests)
	}
	if gk.Team != "finance-team" {
		t.Errorf("gpt-4o team = %q, want finance-team (catalog ServesTeam)", gk.Team)
	}
	if math.Abs(gk.LatencyMillis-800) > 0.001 {
		t.Errorf("gpt-4o latency = %.0fms, want 800ms", gk.LatencyMillis)
	}
	// The cached token type must not leak into output tokens.
	if gk.OutputTokens != 0 {
		t.Errorf("gpt-4o output = %d, want 0 (cache_read_input ignored)", gk.OutputTokens)
	}
}

func TestCollectNoMetricFamily(t *testing.T) {
	// A gateway with no traffic yet exposes no gen_ai_client_token_usage family.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# nothing here\n"))
	}))
	defer srv.Close()

	c := New(newFakeClient(t), "ns", srv.URL)
	if _, err := c.Collect(context.Background(), time.Hour); err == nil {
		t.Fatal("expected an error when the token metric is absent, got nil")
	}
}

func TestCollectHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(newFakeClient(t), "ns", srv.URL)
	if _, err := c.Collect(context.Background(), time.Hour); err == nil {
		t.Fatal("expected an error on HTTP 500, got nil")
	}
}

// errorSplitExposition reproduces the measured E1 case (run
// aks-live-20260708T160246Z): finance/risk-assistant on gpt-us-mini had 85
// successful responses (carrying tokens) and 660 error responses (error_type
// "_OTHER", response_model "unknown", no tokens). The collector must report
// Requests=85 (successful, matching the billed tokens) and Errors=660, never
// letting the larger error count inflate the request count.
const errorSplitExposition = `# TYPE gen_ai_client_token_usage histogram
gen_ai_client_token_usage_bucket{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="input",k8s_namespace="finance",k8s_app="risk-assistant",le="+Inf"} 85
gen_ai_client_token_usage_sum{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="input",k8s_namespace="finance",k8s_app="risk-assistant"} 1299
gen_ai_client_token_usage_count{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="input",k8s_namespace="finance",k8s_app="risk-assistant"} 85
gen_ai_client_token_usage_bucket{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="output",k8s_namespace="finance",k8s_app="risk-assistant",le="+Inf"} 85
gen_ai_client_token_usage_sum{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="output",k8s_namespace="finance",k8s_app="risk-assistant"} 10882
gen_ai_client_token_usage_count{gen_ai_request_model="gpt-us-mini",gen_ai_token_type="output",k8s_namespace="finance",k8s_app="risk-assistant"} 85
# TYPE gen_ai_server_request_duration_seconds histogram
gen_ai_server_request_duration_seconds_bucket{gen_ai_request_model="gpt-us-mini",k8s_namespace="finance",k8s_app="risk-assistant",le="+Inf"} 85
gen_ai_server_request_duration_seconds_sum{gen_ai_request_model="gpt-us-mini",k8s_namespace="finance",k8s_app="risk-assistant"} 17.0
gen_ai_server_request_duration_seconds_count{gen_ai_request_model="gpt-us-mini",k8s_namespace="finance",k8s_app="risk-assistant"} 85
gen_ai_server_request_duration_seconds_bucket{error_type="_OTHER",gen_ai_request_model="gpt-us-mini",gen_ai_response_model="unknown",k8s_namespace="finance",k8s_app="risk-assistant",le="+Inf"} 660
gen_ai_server_request_duration_seconds_sum{error_type="_OTHER",gen_ai_request_model="gpt-us-mini",gen_ai_response_model="unknown",k8s_namespace="finance",k8s_app="risk-assistant"} 66.0
gen_ai_server_request_duration_seconds_count{error_type="_OTHER",gen_ai_request_model="gpt-us-mini",gen_ai_response_model="unknown",k8s_namespace="finance",k8s_app="risk-assistant"} 660
`

func TestCollectSplitsSuccessAndError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(errorSplitExposition))
	}))
	defer srv.Close()

	scheme := runtime.NewScheme()
	if err := aiopsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	model := &aiopsv1alpha1.AIModel{
		ObjectMeta: metav1.ObjectMeta{Name: "gpt-us-mini", Namespace: "ns"},
		Spec:       aiopsv1alpha1.AIModelSpec{ProviderRef: "openai-us", ModelName: "gpt-us-mini", Type: "llm"},
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(model).Build()

	samples, err := New(cl, "ns", srv.URL).Collect(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1: %+v", len(samples), samples)
	}
	s := samples[0]
	if s.Requests != 85 {
		t.Errorf("Requests = %d, want 85 (successful only; the 660 error count must not inflate it)", s.Requests)
	}
	if s.Errors != 660 {
		t.Errorf("Errors = %d, want 660 (error_type series counted separately)", s.Errors)
	}
	if s.InputTokens != 1299 || s.OutputTokens != 10882 {
		t.Errorf("tokens = in %d/out %d, want 1299/10882 (successful responses only)", s.InputTokens, s.OutputTokens)
	}
	// Latency must reflect the 85 successful calls (17.0s/85 = 200ms), not the error series.
	if math.Abs(s.LatencyMillis-200) > 0.001 {
		t.Errorf("LatencyMillis = %.3f, want 200 (successful series only)", s.LatencyMillis)
	}
}
