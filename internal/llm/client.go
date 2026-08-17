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
	defaultOpenAIBase   = "https://api.openai.com"
	anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion    = "2023-06-01"
	httpTimeout         = 120 * time.Second
	retryWait1          = 500 * time.Millisecond
	retryWait2          = 1500 * time.Millisecond
	maxAttempts         = 3
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Provider string // openai | anthropic | compatible
	BaseURL  string
	APIKey   string
	Model    string
	Temp     float64
	Messages []Message
}

type Client struct {
	HTTP *http.Client // 120s timeout
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
	case "openai", "compatible":
		return buildOpenAIRequest(ctx, req)
	case "anthropic":
		return buildAnthropicRequest(ctx, req)
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", req.Provider)
	}
}

func buildOpenAIRequest(ctx context.Context, req Request) (*http.Request, error) {
	payload, err := json.Marshal(struct {
		Model       string    `json:"model"`
		Temperature float64   `json:"temperature"`
		Messages    []Message `json:"messages"`
	}{
		Model:       req.Model,
		Temperature: req.Temp,
		Messages:    req.Messages,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIURL(req.BaseURL), bytes.NewReader(payload))
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
	payload, err := json.Marshal(struct {
		Model       string    `json:"model"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		System      string    `json:"system,omitempty"`
		Messages    []Message `json:"messages"`
	}{
		Model:       req.Model,
		MaxTokens:   4096,
		Temperature: req.Temp,
		System:      strings.Join(systemParts, "\n\n"),
		Messages:    msgs,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(payload))
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
	default:
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("llm: decode openai: %w", err)
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("llm: empty openai choices")
		}
		return parsed.Choices[0].Message.Content, nil
	}
}

func openAIURL(base string) string {
	if strings.TrimSpace(base) == "" {
		base = defaultOpenAIBase
	}
	return strings.TrimRight(base, "/") + "/v1/chat/completions"
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
