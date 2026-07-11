package sidecarproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProxyInjectsHeadersViaHTTPProxy(t *testing.T) {
	got := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(New(Config{
		Namespace:   "finance",
		Application: "risk-assistant",
	}))
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	req, err := http.NewRequest(http.MethodGet, upstream.URL+"/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("x-original", "keep-me")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client do: %v", err)
	}
	defer resp.Body.Close()

	headers := <-got
	if headers.Get(HeaderNamespace) != "finance" {
		t.Fatalf("%s = %q, want finance", HeaderNamespace, headers.Get(HeaderNamespace))
	}
	if headers.Get(HeaderApp) != "risk-assistant" {
		t.Fatalf("%s = %q, want risk-assistant", HeaderApp, headers.Get(HeaderApp))
	}
	if headers.Get("x-original") != "keep-me" {
		t.Fatalf("x-original = %q, want keep-me", headers.Get("x-original"))
	}
}

func TestProxyCallsGOVARAdmitAndSettle(t *testing.T) {
	var gotAdmit map[string]any
	var gotSettle map[string]any
	govar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/admit":
			_ = json.NewDecoder(r.Body).Decode(&gotAdmit)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"decision":            "ADMIT",
				"reason_code":         "highest_utility_feasible",
				"selected_deployment": "gpt-fr",
			})
		case "/v1/settle":
			_ = json.NewDecoder(r.Body).Decode(&gotSettle)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/v1/cancel":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer govar.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-ai-eg-model"); got != "gpt-fr" {
			t.Fatalf("x-ai-eg-model = %q, want gpt-fr", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 20,
			},
		})
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(New(Config{
		Namespace:         "finance",
		Application:       "risk-assistant",
		TenantID:          "tenant-finance",
		BudgetPolicyName:  "budget-finance",
		RoutingPolicyName: "routing-finance",
		GOVAREndpoint:     govar.URL,
	}))
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	reqBody := `{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}],"max_tokens":64}`
	req, err := http.NewRequest(http.MethodPost, upstream.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotAdmit["tenant_id"] != "tenant-finance" {
		t.Fatalf("admit tenant_id = %#v", gotAdmit["tenant_id"])
	}
	if gotSettle["actual_input_tokens"] != float64(10) || gotSettle["actual_output_tokens"] != float64(20) {
		t.Fatalf("settle payload = %#v", gotSettle)
	}
}

func TestProxyCancelsOnUpstreamFailure(t *testing.T) {
	cancelCalled := false
	govar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/admit":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"decision":            "ADMIT",
				"reason_code":         "highest_utility_feasible",
				"selected_deployment": "gpt-fr",
			})
		case "/v1/cancel":
			cancelCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/v1/settle":
			t.Fatal("settle should not be called on upstream failure")
		default:
			http.NotFound(w, r)
		}
	}))
	defer govar.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "backend failure", http.StatusBadGateway)
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(New(Config{
		Namespace:         "finance",
		Application:       "risk-assistant",
		TenantID:          "tenant-finance",
		BudgetPolicyName:  "budget-finance",
		RoutingPolicyName: "routing-finance",
		GOVAREndpoint:     govar.URL,
	}))
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	req, err := http.NewRequest(http.MethodPost, upstream.URL+"/v1/chat/completions", strings.NewReader(`{"max_tokens":32}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client do: %v", err)
	}
	defer resp.Body.Close()
	if !cancelCalled {
		t.Fatal("expected cancel to be called")
	}
}

func TestProxyHonorsTargetFilter(t *testing.T) {
	got := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}

	proxy := httptest.NewServer(New(Config{
		Namespace:   "legal",
		Application: "contract-review",
		Targets:     []string{"not-" + upstreamURL.Hostname()},
	}))
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(upstream.URL)
	if err != nil {
		t.Fatalf("client get: %v", err)
	}
	defer resp.Body.Close()

	headers := <-got
	if headers.Get(HeaderNamespace) != "" {
		t.Fatalf("%s = %q, want empty", HeaderNamespace, headers.Get(HeaderNamespace))
	}
	if headers.Get(HeaderApp) != "" {
		t.Fatalf("%s = %q, want empty", HeaderApp, headers.Get(HeaderApp))
	}
}
