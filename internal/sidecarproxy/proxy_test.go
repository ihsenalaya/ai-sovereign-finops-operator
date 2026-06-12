package sidecarproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
