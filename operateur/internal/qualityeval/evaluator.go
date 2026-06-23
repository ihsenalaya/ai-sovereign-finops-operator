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

// Package qualityeval runs golden prompts against the production AI gateway and
// serializes response evidence for the pure qualityengine.
package qualityeval

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/imperium/ai-sovereign-finops-operator/internal/qualityengine"
)

const (
	defaultMaxTokens = 96
	maxActualBytes   = 700
)

// Options configures one gateway-backed evaluation run.
type Options struct {
	Endpoint       string
	PromptsDir     string
	PromptsFile    string
	Namespace      string
	Application    string
	SourceModel    string
	CandidateModel string
	MaxTokens      int
	Timeout        time.Duration
	HTTPClient     *http.Client
}

// GoldenPrompt mirrors the AIQualityGate golden dataset format.
type GoldenPrompt struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
	// Images holds optional image inputs for multimodal (vision) golden prompts.
	// When present, the gateway request is sent as an OpenAI multimodal content
	// array (text part + image_url parts). When empty, the request stays a plain
	// text string, so existing text-only datasets are unchanged.
	Images   []PromptImage `json:"images,omitempty"`
	Expected struct {
		RequiredKeywords []string          `json:"requiredKeywords,omitempty"`
		MaxTokens        int32             `json:"maxTokens,omitempty"`
		MustBeJSON       bool              `json:"mustBeJSON,omitempty"`
		Reference        string            `json:"reference,omitempty"`
		Fields           map[string]string `json:"fields,omitempty"`
	} `json:"expected,omitempty"`
}

// PromptImage references one image input for a multimodal golden prompt. Exactly
// one of URL or File should be set. File is resolved against the prompts
// directory (the mounted golden-dataset volume) and inlined as a base64 data URI,
// which keeps the image alongside the dataset without a public URL.
type PromptImage struct {
	URL  string `json:"url,omitempty"`
	File string `json:"file,omitempty"`
	MIME string `json:"mime,omitempty"`
}

// EvidenceSample is the serialized response evidence consumed by AIQualityGate.
type EvidenceSample struct {
	ID                      string            `json:"id"`
	Model                   string            `json:"model,omitempty"`
	Reference               string            `json:"reference,omitempty"`
	Actual                  string            `json:"actual,omitempty"`
	ExpectedFields          map[string]string `json:"expectedFields,omitempty"`
	ActualFields            map[string]string `json:"actualFields,omitempty"`
	SemanticScore           *float64          `json:"semanticScore,omitempty"`
	SchemaValid             *bool             `json:"schemaValid,omitempty"`
	UnexpectedRefusal       *bool             `json:"unexpectedRefusal,omitempty"`
	SensitiveDataLeak       *bool             `json:"sensitiveDataLeak,omitempty"`
	RequiredKeywordsPresent *bool             `json:"requiredKeywordsPresent,omitempty"`
}

// EvidenceDocument is written to the AIQualityGate evidence ConfigMap.
type EvidenceDocument struct {
	Samples []EvidenceSample `json:"samples"`
}

// Run evaluates source and candidate models and returns JSON evidence.
func Run(ctx context.Context, opts Options) ([]byte, error) {
	opts.Endpoint = strings.TrimSpace(opts.Endpoint)
	if opts.Endpoint == "" {
		return nil, fmt.Errorf("evaluation endpoint is empty")
	}
	if strings.TrimSpace(opts.SourceModel) == "" || strings.TrimSpace(opts.CandidateModel) == "" {
		return nil, fmt.Errorf("source and candidate model are required")
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = defaultMaxTokens
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}

	prompts, err := LoadPrompts(opts.PromptsDir, opts.PromptsFile)
	if err != nil {
		return nil, err
	}
	if len(prompts) == 0 {
		return nil, fmt.Errorf("golden dataset contains no prompts")
	}

	models := []string{opts.SourceModel}
	if opts.CandidateModel != opts.SourceModel {
		models = append(models, opts.CandidateModel)
	}

	doc := EvidenceDocument{Samples: make([]EvidenceSample, 0, len(prompts)*len(models))}
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt.ID) == "" || strings.TrimSpace(prompt.Prompt) == "" {
			return nil, fmt.Errorf("golden prompt %q is missing id or prompt text", prompt.ID)
		}
		for _, model := range models {
			actual, err := callGateway(ctx, client, opts, prompt, model)
			if err != nil {
				return nil, err
			}
			doc.Samples = append(doc.Samples, evidenceFor(prompt, model, actual))
		}
	}

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize quality evidence: %w", err)
	}
	return raw, nil
}

// LoadPrompts reads a golden dataset from a mounted ConfigMap directory or file.
func LoadPrompts(dir, file string) ([]GoldenPrompt, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(file) != "" {
		raw, err = os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read prompts file %s: %w", file, err)
		}
	} else {
		if strings.TrimSpace(dir) == "" {
			return nil, fmt.Errorf("prompts-dir or prompts-file is required")
		}
		for _, name := range []string{"prompts.yaml", "prompts.yml", "prompts.json"} {
			path := filepath.Join(dir, name)
			raw, err = os.ReadFile(path)
			if err == nil {
				break
			}
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read prompts file %s: %w", path, err)
			}
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("no prompts.yaml, prompts.yml or prompts.json found in %s", dir)
		}
	}

	var prompts []GoldenPrompt
	if err := yaml.Unmarshal(raw, &prompts); err == nil && len(prompts) > 0 {
		return prompts, nil
	}
	var wrapped struct {
		Prompts []GoldenPrompt `json:"prompts"`
	}
	if err := yaml.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("parse golden dataset: %w", err)
	}
	return wrapped.Prompts, nil
}

func callGateway(ctx context.Context, client *http.Client, opts Options, prompt GoldenPrompt, model string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		content, err := callGatewayOnce(ctx, client, opts, prompt, model)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if attempt == 5 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt*2) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}

func callGatewayOnce(ctx context.Context, client *http.Client, opts Options, prompt GoldenPrompt, model string) (string, error) {
	maxTokens := opts.MaxTokens
	if prompt.Expected.MaxTokens > 0 {
		maxTokens = int(prompt.Expected.MaxTokens)
	}
	content, err := opts.buildMessageContent(prompt)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"model":       model,
		"temperature": 0,
		"max_tokens":  maxTokens,
		"messages": []map[string]any{{
			"role":    "user",
			"content": content,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request for prompt %q model %q: %w", prompt.ID, model, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request for prompt %q model %q: %w", prompt.ID, model, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ai-eg-model", model)
	if opts.Namespace != "" {
		req.Header.Set("x-greenops-namespace", opts.Namespace)
	}
	if opts.Application != "" {
		req.Header.Set("x-greenops-app", opts.Application)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway call failed for prompt %q model %q: %w", prompt.ID, model, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gateway call for prompt %q model %q returned HTTP %d: %s", prompt.ID, model, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	responseText, err := extractChatContent(raw)
	if err != nil {
		return "", fmt.Errorf("parse gateway response for prompt %q model %q: %w", prompt.ID, model, err)
	}
	return truncateForEvidence(responseText), nil
}

// buildMessageContent returns the OpenAI chat message content for a prompt. With
// no images it is the plain prompt string (text-only, backward compatible). With
// images it is a multimodal content array: a text part plus one image_url part
// per image, each resolved to a URL or an inlined base64 data URI.
func (o Options) buildMessageContent(prompt GoldenPrompt) (any, error) {
	if len(prompt.Images) == 0 {
		return prompt.Prompt, nil
	}
	parts := make([]map[string]any, 0, len(prompt.Images)+1)
	parts = append(parts, map[string]any{"type": "text", "text": prompt.Prompt})
	for i, img := range prompt.Images {
		url, err := o.resolveImageURL(img)
		if err != nil {
			return nil, fmt.Errorf("prompt %q image %d: %w", prompt.ID, i, err)
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		})
	}
	return parts, nil
}

// resolveImageURL turns a PromptImage into a value usable in an image_url part.
// A URL is passed through; a File is read from the prompts directory (the mounted
// golden-dataset volume) and encoded as a base64 data URI. File paths are scoped
// to the prompts directory: absolute paths and parent traversal are rejected.
func (o Options) resolveImageURL(img PromptImage) (string, error) {
	if u := strings.TrimSpace(img.URL); u != "" {
		return u, nil
	}
	file := strings.TrimSpace(img.File)
	if file == "" {
		return "", fmt.Errorf("image needs url or file")
	}
	if filepath.IsAbs(file) || strings.Contains(file, "..") {
		return "", fmt.Errorf("invalid image file path %q", file)
	}
	if strings.TrimSpace(o.PromptsDir) == "" {
		return "", fmt.Errorf("image file %q requires prompts-dir", file)
	}
	data, err := os.ReadFile(filepath.Join(o.PromptsDir, file))
	if err != nil {
		return "", fmt.Errorf("read image file %s: %w", file, err)
	}
	mime := strings.TrimSpace(img.MIME)
	if mime == "" {
		mime = mimeFromExt(file)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// mimeFromExt maps common image extensions to a MIME type, defaulting to PNG.
func mimeFromExt(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}

func extractChatContent(raw []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	for _, choice := range response.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			return choice.Message.Content, nil
		}
		if strings.TrimSpace(choice.Text) != "" {
			return choice.Text, nil
		}
	}
	return "", fmt.Errorf("response contains no choice content")
}

func evidenceFor(prompt GoldenPrompt, model, actual string) EvidenceSample {
	reference := prompt.Expected.Reference
	if reference == "" && len(prompt.Expected.RequiredKeywords) > 0 {
		reference = strings.Join(prompt.Expected.RequiredKeywords, " ")
	}
	schemaValid := true
	var actualFields map[string]string
	if prompt.Expected.MustBeJSON || len(prompt.Expected.Fields) > 0 {
		schemaValid = json.Valid([]byte(actual))
		if schemaValid {
			actualFields = parseStringFields(actual)
		}
	}
	var semanticScore *float64
	if strings.TrimSpace(reference) != "" {
		semantic := qualityengine.SemanticSimilarityScore(reference, actual)
		if len(prompt.Expected.RequiredKeywords) > 0 {
			semantic = 0.75*semantic + 0.25*keywordCoverageScore(actual, prompt.Expected.RequiredKeywords)
		}
		semanticScore = &semantic
	}
	requiredKeywordsPresent := containsAll(actual, prompt.Expected.RequiredKeywords)
	unexpectedRefusal := looksLikeRefusal(actual)
	sensitiveDataLeak := false
	return EvidenceSample{
		ID:                      prompt.ID,
		Model:                   model,
		Reference:               reference,
		Actual:                  actual,
		ExpectedFields:          prompt.Expected.Fields,
		ActualFields:            actualFields,
		SemanticScore:           semanticScore,
		SchemaValid:             &schemaValid,
		UnexpectedRefusal:       &unexpectedRefusal,
		SensitiveDataLeak:       &sensitiveDataLeak,
		RequiredKeywordsPresent: &requiredKeywordsPresent,
	}
}

func parseStringFields(raw string) map[string]string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil
	}
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		switch typed := v.(type) {
		case string:
			out[k] = typed
		case float64, bool:
			out[k] = fmt.Sprint(typed)
		}
	}
	return out
}

func containsAll(text string, keywords []string) bool {
	lower := foldLatin(strings.ToLower(text))
	for _, kw := range keywords {
		kw = foldLatin(strings.ToLower(strings.TrimSpace(kw)))
		if kw == "" {
			continue
		}
		if !strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}

func keywordCoverageScore(text string, keywords []string) float64 {
	var total, present int
	lower := foldLatin(strings.ToLower(text))
	for _, kw := range keywords {
		kw = foldLatin(strings.ToLower(strings.TrimSpace(kw)))
		if kw == "" {
			continue
		}
		total++
		if strings.Contains(lower, kw) {
			present++
		}
	}
	if total == 0 {
		return 100
	}
	return 100 * float64(present) / float64(total)
}

func foldLatin(s string) string {
	replacer := strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a",
		"ç", "c",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"î", "i", "ï", "i",
		"ô", "o", "ö", "o",
		"ù", "u", "û", "u", "ü", "u",
	)
	return replacer.Replace(s)
}

func looksLikeRefusal(text string) bool {
	lower := strings.ToLower(text)
	needles := []string{
		"i can't",
		"i cannot",
		"je ne peux pas",
		"je ne peux",
		"cannot comply",
		"can't assist",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func truncateForEvidence(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxActualBytes {
		return text
	}
	return text[:maxActualBytes]
}
