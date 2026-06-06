// Package llm provides a minimal, provider-agnostic chat client used by the
// experiments to issue real LLM calls. The OpenAI implementation reads its API
// key from a file or environment variable and never logs the secret.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage holds token accounting returned by the provider.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is a chat completion result.
type Response struct {
	Text      string
	Usage     Usage
	LatencyMS int64
	Model     string
	Provider  string
}

// Client issues chat completions.
type Client interface {
	Chat(ctx context.Context, model string, msgs []Message, maxTokens int, temperature float64) (Response, error)
	Provider() string
}

var skRe = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)

// LoadKey reads an API key from a file (supporting "sk-..." or
// "OPENAI_API_KEY=sk-..." content) and falls back to the OPENAI_API_KEY env var.
// The returned key is never logged by this package.
func LoadKey(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if m := skRe.FindString(string(b)); m != "" {
			return m, nil
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	}
	if env := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("no API key found at %s or in OPENAI_API_KEY", path)
}

// OpenAI is a Client backed by the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey   string
	baseURL  string
	provider string
	http     *http.Client
}

// NewOpenAI builds an OpenAI client. baseURL defaults to the public API;
// provider is a label used in telemetry (e.g. "openai").
func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{
		apiKey:   apiKey,
		baseURL:  "https://api.openai.com/v1",
		provider: "openai",
		http:     &http.Client{Timeout: 90 * time.Second},
	}
}

// NewOpenAICompatible builds a client for any OpenAI-compatible Chat Completions
// endpoint (e.g. a self-hosted vLLM server, Azure OpenAI proxy). baseURL is the
// API root ending in /v1; provider is a telemetry label. apiKey may be empty for
// unauthenticated local servers.
func NewOpenAICompatible(baseURL, apiKey, provider string) *OpenAI {
	if provider == "" {
		provider = "openai-compatible"
	}
	return &OpenAI{
		apiKey:   apiKey,
		baseURL:  baseURL,
		provider: provider,
		http:     &http.Client{Timeout: 120 * time.Second},
	}
}

// Provider implements Client.
func (c *OpenAI) Provider() string { return c.provider }

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat implements Client with one retry on transient (5xx/429) errors.
func (c *OpenAI) Chat(ctx context.Context, model string, msgs []Message, maxTokens int, temperature float64) (Response, error) {
	reqBody, _ := json.Marshal(chatRequest{Model: model, Messages: msgs, MaxTokens: maxTokens, Temperature: temperature})

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(reqBody))
		if err != nil {
			return Response{}, err
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		latency := time.Since(start).Milliseconds()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("openai %s: status %d", model, resp.StatusCode)
			continue
		}
		var cr chatResponse
		if err := json.Unmarshal(body, &cr); err != nil {
			return Response{}, fmt.Errorf("decode openai response: %w", err)
		}
		if cr.Error != nil {
			return Response{}, fmt.Errorf("openai %s: %s (%s)", model, cr.Error.Message, cr.Error.Type)
		}
		if len(cr.Choices) == 0 {
			return Response{}, fmt.Errorf("openai %s: empty choices", model)
		}
		return Response{
			Text:      strings.TrimSpace(cr.Choices[0].Message.Content),
			Usage:     Usage{InputTokens: cr.Usage.PromptTokens, OutputTokens: cr.Usage.CompletionTokens},
			LatencyMS: latency,
			Model:     model,
			Provider:  c.provider,
		}, nil
	}
	return Response{}, fmt.Errorf("openai %s failed after retries: %w", model, lastErr)
}
