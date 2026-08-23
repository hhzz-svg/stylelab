package llm

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

const (
	defaultOpenAIBase    = "https://api.openai.com"
	anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion     = "2023-06-01"
	httpTimeout          = 180 * time.Second
	defaultAnthropicMax  = 4096
	retryWait1           = 500 * time.Millisecond
	retryWait2           = 1500 * time.Millisecond
	maxAttempts          = 3
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Provider  string // openai | anthropic | compatible
	BaseURL   string
	APIKey    string
	Model     string
	Temp      float64
	MaxTokens int // 0 = provider default (Anthropic 4096)
	Messages  []Message
}

type Client struct {
	HTTP *http.Client // 180s timeout
}

var defaultHTTP = &http.Client{Timeout: httpTimeout}

func (c *Client) Chat(ctx context.Context, req Request) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			wait := retryWait1
			if attempt == 2 {
				wait = retryWait2
			}
			if err := sleepCtx(ctx, wait); err != nil {
				return "", err
			}
		}
		text, status, err := c.do(ctx, req)
		if err != nil {
			return "", err
		}
		if status >= 200 && status < 300 {
			return text, nil
		}
		lastErr = fmt.Errorf("llm: http %d: %s", status, text)
		if status == http.StatusTooManyRequests || status >= 500 {
			continue
		}
		return "", lastErr
	}
	return "", lastErr
}

func (c *Client) do(ctx context.Context, req Request) (string, int, error) {
	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return "", 0, err
	}
	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return strings.TrimSpace(string(body)), resp.StatusCode, nil
	}
	text, err := parseResponse(req.Provider, body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return text, resp.StatusCode, nil
}

func (c *Client) buildRequest(ctx context.Context, req Request) (*http.Request, error) {
	switch req.Provider {
	case "chat", "openai", "compatible", "":
		return buildOpenAIRequest(ctx, req)
	case "response", "responses":
		return buildResponseRequest(ctx, req)
	case "anthropic":
		return buildAnthropicRequest(ctx, req)
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", req.Provider)
	}
}

func buildOpenAIRequest(ctx context.Context, req Request) (*http.Request, error) {
	body := map[string]any{
		"model":       req.Model,
		"temperature": req.Temp,
		"messages":    req.Messages,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, OpenAIURL(req.BaseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return httpReq, nil
}

func buildResponseRequest(ctx context.Context, req Request) (*http.Request, error) {
	body := map[string]any{
		"model":       req.Model,
		"temperature": req.Temp,
		"input":       req.Messages,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ResponseURL(req.BaseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return httpReq, nil
}

func buildAnthropicRequest(ctx context.Context, req Request) (*http.Request, error) {
	var systemParts []string
	var msgs []Message
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
			continue
		}
		msgs = append(msgs, m)
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultAnthropicMax
	}
	payload, err := json.Marshal(struct {
		Model       string    `json:"model"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		System      string    `json:"system,omitempty"`
		Messages    []Message `json:"messages"`
	}{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Temperature: req.Temp,
		System:      strings.Join(systemParts, "\n\n"),
		Messages:    msgs,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, AnthropicURL(req.BaseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	return httpReq, nil
}

func parseResponse(provider string, body []byte) (string, error) {
	switch provider {
	case "anthropic":
		var parsed struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("llm: decode anthropic: %w", err)
		}
		if len(parsed.Content) == 0 {
			return "", fmt.Errorf("llm: empty anthropic content")
		}
		return parsed.Content[0].Text, nil
	case "response", "responses":
		var parsed struct {
			OutputText string `json:"output_text"`
			Output     []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &parsed); err == nil {
			if parsed.OutputText != "" {
				return parsed.OutputText, nil
			}
			if len(parsed.Output) > 0 && len(parsed.Output[0].Content) > 0 {
				return parsed.Output[0].Content[0].Text, nil
			}
			if len(parsed.Choices) > 0 && parsed.Choices[0].Message.Content != "" {
				return parsed.Choices[0].Message.Content, nil
			}
		}
		return "", fmt.Errorf("llm: empty response output")
	default:
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("llm: decode chat: %w", err)
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("llm: empty chat choices")
		}
		return parsed.Choices[0].Message.Content, nil
	}
}

func AnthropicURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return anthropicMessagesURL
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages"
	}
	return base + "/v1/messages"
}

func ResponseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultOpenAIBase + "/v1/responses"
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1/responses") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/responses"
	}
	return base + "/v1/responses"
}

func OpenAIURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultOpenAIBase + "/v1/chat/completions"
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return defaultHTTP
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
