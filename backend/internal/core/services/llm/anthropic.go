// Package llm holds the one thing several features need from a language model:
// send a system prompt and a message, get text back.
//
// It exists so that the shape of the Anthropic API - which header carries the
// key, which version to pin, where the text sits in the reply - is written down
// once. Two copies of that knowledge is how one of them quietly goes stale.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	defaultEndpoint  = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"
)

// DefaultModel is the small, fast one. Every use here is a short, well-defined
// job rather than anything needing a large model.
const DefaultModel = "claude-haiku-4-5-20251001"

type Client struct {
	// Endpoint is a field rather than a constant so tests can point it at a
	// stub and never reach the real service.
	Endpoint string
	Model    string
	HTTP     *http.Client
}

func NewClient(model string, timeout time.Duration) *Client {
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}

	return &Client{
		Endpoint: defaultEndpoint,
		Model:    model,
		HTTP:     &http.Client{Timeout: timeout},
	}
}

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one prompt and returns the text of the reply.
func (c *Client) Complete(ctx context.Context, apiKey, system, user string, maxTokens int) (string, error) {
	body, err := json.Marshal(request{
		Model:     c.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []message{{Role: "user", Content: user}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed response
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("llm: unreadable response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("llm: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("llm: unexpected status %d", resp.StatusCode)
	}

	var text strings.Builder
	for _, part := range parsed.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}

	return text.String(), nil
}

// ExtractJSON pulls the JSON object out of a reply. A fenced block is asked
// against but arrives often enough to be worth tolerating rather than failing
// on.
func ExtractJSON(text string) string {
	raw := strings.TrimSpace(text)

	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			return raw[start : end+1]
		}
	}

	return raw
}
