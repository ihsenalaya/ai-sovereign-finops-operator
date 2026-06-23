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

package qualityeval

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunCallsGatewayForSourceAndCandidate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(`
- id: risk
  prompt: "Summarize risk."
  expected:
    reference: "Risk includes liquidity exposure."
    requiredKeywords: ["risk"]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-greenops-namespace"); got != "finance" {
			t.Fatalf("namespace header = %q, want finance", got)
		}
		if got := r.Header.Get("x-greenops-app"); got != "risk-assistant" {
			t.Fatalf("app header = %q, want risk-assistant", got)
		}
		model := r.Header.Get("x-ai-eg-model")
		calls = append(calls, model)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Risk includes liquidity exposure."}}]}`))
	}))
	defer srv.Close()

	raw, err := Run(context.Background(), Options{
		Endpoint:       srv.URL,
		PromptsDir:     dir,
		Namespace:      "finance",
		Application:    "risk-assistant",
		SourceModel:    "gpt-us-mini",
		CandidateModel: "gpt-france-mini",
		Timeout:        time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "gpt-us-mini" || calls[1] != "gpt-france-mini" {
		t.Fatalf("gateway calls = %#v, want source then candidate", calls)
	}
	var doc EvidenceDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Samples) != 2 {
		t.Fatalf("samples = %d, want 2", len(doc.Samples))
	}
	if doc.Samples[0].SemanticScore == nil || *doc.Samples[0].SemanticScore <= 0 {
		t.Fatalf("semantic score was not populated: %#v", doc.Samples[0])
	}
}

func TestRunSendsMultimodalContentForImagePrompts(t *testing.T) {
	dir := t.TempDir()
	// A 1x1 PNG written to the mounted golden-dataset volume.
	pngBytes := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")
	if err := os.WriteFile(filepath.Join(dir, "invoice.png"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(`
- id: ocr-total
  prompt: "Extract the invoice total."
  images:
    - file: invoice.png
    - url: https://example.test/remote.jpg
  expected:
    requiredKeywords: ["total"]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	type imagePart struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	var lastContent []imagePart
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content []imagePart `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Messages) == 1 {
			lastContent = body.Messages[0].Content
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Total is 1240 EUR."}}]}`))
	}))
	defer srv.Close()

	_, err := Run(context.Background(), Options{
		Endpoint:       srv.URL,
		PromptsDir:     dir,
		SourceModel:    "vision-a",
		CandidateModel: "vision-b",
		Timeout:        time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(lastContent) != 3 {
		t.Fatalf("content parts = %d, want 3 (text + 2 images): %#v", len(lastContent), lastContent)
	}
	if lastContent[0].Type != "text" || lastContent[0].Text != "Extract the invoice total." {
		t.Fatalf("first part is not the text prompt: %#v", lastContent[0])
	}
	wantDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	if lastContent[1].Type != "image_url" || lastContent[1].ImageURL.URL != wantDataURI {
		t.Fatalf("file image not inlined as data URI: %#v", lastContent[1])
	}
	if lastContent[2].ImageURL.URL != "https://example.test/remote.jpg" {
		t.Fatalf("url image not passed through: %#v", lastContent[2])
	}
}

func TestRunRejectsImageFileOutsidePromptsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(`
- id: bad
  prompt: "Read it."
  images:
    - file: ../etc/passwd
`), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	_, err := Run(context.Background(), Options{
		Endpoint:       srv.URL,
		PromptsDir:     dir,
		SourceModel:    "a",
		CandidateModel: "b",
		Timeout:        time.Second,
	})
	if err == nil {
		t.Fatal("expected error for image path traversal, got nil")
	}
}

func TestRunFailsOnGatewayError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(`
- id: one
  prompt: "Hello"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "backend unavailable", http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := Run(context.Background(), Options{
		Endpoint:       srv.URL,
		PromptsDir:     dir,
		SourceModel:    "source",
		CandidateModel: "candidate",
		Timeout:        time.Second,
	})
	if err == nil {
		t.Fatal("expected gateway error")
	}
}
