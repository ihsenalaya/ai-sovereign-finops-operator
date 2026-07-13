package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundedEndpointDoesNotExposeTenantOrRequestIdentity(t *testing.T) {
	tests := map[string]string{
		"/v1/admit":                       "admit",
		"/v1/liability/private-tenant-id": "liability",
		"/anything/request-123":           "unknown",
	}
	for path, expected := range tests {
		if got := boundedEndpoint(path); got != expected {
			t.Fatalf("boundedEndpoint(%q)=%q want %q", path, got, expected)
		}
	}
}

func TestInstrumentHTTPPreservesStatus(t *testing.T) {
	handler := instrumentHTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic", http.StatusTeapot)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/admit", nil))
	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusTeapot)
	}
}
