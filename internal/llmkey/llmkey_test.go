package llmkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"stylelab/internal/cryptokey"
	"stylelab/internal/llmkey"
	"stylelab/internal/store"
)

// The selection rule used to be implicit in eight copies of this loader. These
// tests pin it down so a change is a deliberate one.

var master = make([]byte, 32)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func addUser(t *testing.T, st *store.Store, id string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.DB().Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, 'x', ?)`,
		id, id+"@example.com", now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func addKey(t *testing.T, st *store.Store, userID, provider, baseURL, secret string) {
	t.Helper()
	blob, err := cryptokey.Seal(master, secret)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.DB().Exec(
		`INSERT INTO user_llm_keys (user_id, provider, base_url, encrypted_key, last4, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, provider, baseURL, blob, secret[len(secret)-4:], now, now,
	); err != nil {
		t.Fatalf("insert key: %v", err)
	}
}

func TestLoadNoKey(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	_, err := llmkey.Load(context.Background(), st, master, "usr_a")
	if !errors.Is(err, llmkey.ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
	// The HTTP layer strips this prefix for display; keep it.
	if err.Error() != "invalid: missing llm key" {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestLoadDecryptsSingleKey(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	addKey(t, st, "usr_a", "anthropic", "https://example.test", "sk-ant-secret-1234")
	k, err := llmkey.Load(context.Background(), st, master, "usr_a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if k.Provider != "anthropic" || k.BaseURL != "https://example.test" || k.APIKey != "sk-ant-secret-1234" {
		t.Fatalf("got %+v", k)
	}
}

// With several keys, an OpenAI-shaped provider wins even when another sorts
// first alphabetically.
func TestLoadPrefersOpenAIShapedProvider(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	addKey(t, st, "usr_a", "anthropic", "", "sk-ant-aaaa1111")
	addKey(t, st, "usr_a", "compatible", "", "sk-cmp-bbbb2222")
	addKey(t, st, "usr_a", "chat", "", "sk-chat-cccc3333")
	k, err := llmkey.Load(context.Background(), st, master, "usr_a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if k.Provider != "chat" {
		t.Fatalf("provider = %q, want chat", k.Provider)
	}
}

// Without a preferred provider the first in provider order is used.
func TestLoadFallsBackToFirstByProvider(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	addKey(t, st, "usr_a", "compatible", "", "sk-cmp-bbbb2222")
	addKey(t, st, "usr_a", "anthropic", "", "sk-ant-aaaa1111")
	k, err := llmkey.Load(context.Background(), st, master, "usr_a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if k.Provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", k.Provider)
	}
}

func TestLoadIsScopedToUser(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	addUser(t, st, "usr_b")
	addKey(t, st, "usr_a", "chat", "", "sk-chat-aaaa1111")
	if _, err := llmkey.Load(context.Background(), st, master, "usr_b"); !errors.Is(err, llmkey.ErrMissing) {
		t.Fatalf("usr_b err = %v, want ErrMissing", err)
	}
}

func TestLoadWrongMasterFails(t *testing.T) {
	st := newStore(t)
	addUser(t, st, "usr_a")
	addKey(t, st, "usr_a", "chat", "", "sk-chat-aaaa1111")
	other := make([]byte, 32)
	other[0] = 1
	if _, err := llmkey.Load(context.Background(), st, other, "usr_a"); err == nil {
		t.Fatal("expected a decrypt error with the wrong master key")
	}
}
