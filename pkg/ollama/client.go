package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 60 * time.Second

// Client communicates with a local Ollama instance via its REST API.
type Client struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates an Ollama API client for the given endpoint and model.
// Functional options may override the default HTTP client (60s timeout).
func NewClient(endpoint, model string, opts ...Option) *Client {
	c := &Client{
		endpoint:   endpoint,
		model:      model,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Generate sends a prompt to the Ollama generate endpoint and returns
// the model's response text. The request disables streaming and
// requests JSON-formatted output.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := ollamaGenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama: generate: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("ollama: generate: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ollama: generate: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var genResp ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("ollama: generate: %w", err)
	}

	return genResp.Response, nil
}

// Ping checks whether the Ollama server is reachable.
// Returns nil on HTTP 200, or a wrapped error otherwise.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/", nil)
	if err != nil {
		return fmt.Errorf("ollama: ping: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: ping: %w", err)
	}
	defer resp.Body.Close()
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: ping: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// ModelAvailable checks whether the configured model is available on
// the Ollama server by querying GET /api/tags. It strips the ":latest"
// tag suffix when comparing model names.
func (c *Client) ModelAvailable(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/api/tags", nil)
	if err != nil {
		return false, fmt.Errorf("ollama: model available: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("ollama: model available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ollama: model available: unexpected status %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return false, fmt.Errorf("ollama: model available: %w", err)
	}

	want := stripLatest(c.model)
	for _, m := range tagsResp.Models {
		if stripLatest(m.Name) == want {
			return true, nil
		}
	}

	return false, nil
}

// stripLatest removes a trailing ":latest" suffix from a model name
// so that "granite-guardian:8b" matches "granite-guardian:8b:latest".
func stripLatest(name string) string {
	return strings.TrimSuffix(name, ":latest")
}

// --- unexported request/response types ---

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name string `json:"name"`
}
