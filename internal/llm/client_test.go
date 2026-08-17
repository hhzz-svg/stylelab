package llm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"stylelab/internal/llm"
)

func TestChatOpenAISuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization %q", got)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("content-type %q", ct)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		var body struct {
			Model       string        `json:"model"`
			Temperature float64       `json:"temperature"`
			Messages    []llm.Message `json:"messages"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode: %v", err)
		}
		if body.Model != "gpt-4o-mini" {
			t.Errorf("model %q", body.Model)
		}
		if body.Temperature != 0.3 {
			t.Errorf("temp %v", body.Temperature)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != "hi" {
			t.Errorf("messages %+v", body.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello from model"}}]}`))
	}))
	defer srv.Close()

	c := &llm.Client{HTTP: srv.Client()}
	got, err := c.Chat(context.Background(), llm.Request{
		Provider: "openai",
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "gpt-4o-mini",
		Temp:     0.3,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "hello from model" {
		t.Fatalf("got %q", got)
	}
}

func TestChatRetriesOn500ThenSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"recovered"}}]}`))
	}))
	defer srv.Close()

	c := &llm.Client{HTTP: srv.Client()}
	got, err := c.Chat(context.Background(), llm.Request{
		Provider: "compatible",
		BaseURL:  srv.URL,
		APIKey:   "k",
		Model:    "local",
		Messages: []llm.Message{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "recovered" {
		t.Fatalf("got %q", got)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits %d, want 2", hits.Load())
	}
}

func TestChatNoRetryOn400(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := &llm.Client{HTTP: srv.Client()}
	start := time.Now()
	_, err := c.Chat(context.Background(), llm.Request{
		Provider: "openai",
		BaseURL:  srv.URL,
		APIKey:   "k",
		Model:    "gpt-4o-mini",
		Messages: []llm.Message{{Role: "user", Content: "ping"}},
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits %d, want 1 (no retry)", hits.Load())
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("looks like a retry sleep: %s", elapsed)
	}
}

func TestForbiddenCopyCheck(t *testing.T) {
	banned := []string{
		"模仿作者",
		"复刻作者",
		"还原作者",
		"像某某写的",
		"仿写某某",
		"仿写",
	}
	for _, p := range banned {
		if !llm.ForbiddenCopyCheck("前缀" + p + "后缀") {
			t.Fatalf("expected true for %q", p)
		}
	}
	if llm.ForbiddenCopyCheck("只描述句式节奏、感官落点与场面推进。") {
		t.Fatal("clean text should be false")
	}
}

func TestExtractSystemHasNoBannedPhrases(t *testing.T) {
	s := llm.ExtractSystem()
	if strings.TrimSpace(s) == "" {
		t.Fatal("empty ExtractSystem")
	}
	if llm.ForbiddenCopyCheck(s) {
		t.Fatal("ExtractSystem contains banned phrases")
	}
	for _, key := range []string{
		"sentence_rhythm", "narrative_perspective", "dialogue_density",
		"sensory_description", "scene_pacing", "emotional_expression",
		"rhetoric_preference", "lexical_texture", "tension_hook",
		"prohibitions",
	} {
		if !strings.Contains(s, key) {
			t.Errorf("ExtractSystem missing %s", key)
		}
	}
}

func TestPromptConstructorsHaveNoBannedPhrases(t *testing.T) {
	prompts := []string{
		llm.FuseSystem(),
		llm.AuditSystem("commercial_web"),
		llm.AuditSystem("literary_texture"),
		llm.SampleSystem(`{"name":"节奏样本"}`),
	}
	for i, s := range prompts {
		if strings.TrimSpace(s) == "" {
			t.Errorf("prompt %d empty", i)
		}
		if llm.ForbiddenCopyCheck(s) {
			t.Errorf("prompt %d contains banned phrases", i)
		}
	}
	if !strings.Contains(llm.SampleSystem(`{"id":"crd_1"}`), `{"id":"crd_1"}`) {
		t.Fatal("SampleSystem should include card JSON")
	}
}
