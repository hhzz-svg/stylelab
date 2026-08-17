package extract_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"stylelab/internal/auth"
	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/extract"
	"stylelab/internal/ids"
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/stylestat"
)

func TestRunPersistsCardAndMarksSmallSample(t *testing.T) {
	st, dataDir, master, uid, pid := openExtractStore(t)
	text := "第一章\n春风过境。\n\n第二章\n夏雨初歇。"
	assetID := insertAsset(t, st, dataDir, pid, "sample.txt", text)

	srv := fakeLLMServer(t, validExtractJSON())
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-test-key")

	got, err := extract.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, extract.ExtractInput{
		ProjectID: pid,
		AssetIDs:  []string{assetID},
		Name:      "节奏样本",
		Model:     "gpt-4o-mini",
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ids.Valid(got.ID, "crd_") {
		t.Fatalf("card id: %s", got.ID)
	}
	if got.ProjectID != pid {
		t.Fatalf("project id: %s", got.ProjectID)
	}
	if got.Name != "节奏样本" {
		t.Fatalf("name: %s", got.Name)
	}
	if got.Kind != "extracted" {
		t.Fatalf("kind: %s", got.Kind)
	}
	if got.Version != 1 {
		t.Fatalf("version: %d", got.Version)
	}
	if got.Lineage != nil {
		t.Fatalf("extracted card must not have lineage: %+v", got.Lineage)
	}
	if err := card.Validate(got); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	var facts stylestat.Stats
	if err := json.Unmarshal(got.Facts, &facts); err != nil {
		t.Fatalf("facts json: %v", err)
	}
	if !facts.SampleTooSmall {
		t.Fatalf("expected sample_too_small for <5 chapters: %+v", facts)
	}
	if facts.Chapters != 2 {
		t.Fatalf("facts.chapters: %d", facts.Chapters)
	}

	var storedKind string
	var storedVersion int
	err = st.DB().QueryRow(`SELECT kind, current_version FROM style_cards WHERE id = ? AND project_id = ?`, got.ID, pid).
		Scan(&storedKind, &storedVersion)
	if err != nil {
		t.Fatalf("style_cards: %v", err)
	}
	if storedKind != "extracted" || storedVersion != 1 {
		t.Fatalf("stored card kind=%s version=%d", storedKind, storedVersion)
	}

	var factsJSON string
	err = st.DB().QueryRow(`SELECT facts_json FROM style_card_versions WHERE card_id = ? AND version = 1`, got.ID).
		Scan(&factsJSON)
	if err != nil {
		t.Fatalf("style_card_versions: %v", err)
	}
	if !strings.Contains(factsJSON, `"sample_too_small"`) {
		t.Fatalf("persisted facts missing sample_too_small: %s", factsJSON)
	}
}

func TestRunMissingLLMKey(t *testing.T) {
	st, dataDir, master, uid, pid := openExtractStore(t)
	assetID := insertAsset(t, st, dataDir, pid, "sample.txt", "第一章\n甲\n\n第二章\n乙")

	_, err := extract.Run(context.Background(), st, &llm.Client{}, master, uid, extract.ExtractInput{
		ProjectID: pid,
		AssetIDs:  []string{assetID},
		Name:      "无钥",
	}, nil)
	if err == nil {
		t.Fatal("expected missing llm key error")
	}
	if err.Error() != "invalid: missing llm key" {
		t.Fatalf("error: %v", err)
	}
}

func TestRunRejectsOversizeSample(t *testing.T) {
	st, dataDir, master, uid, pid := openExtractStore(t)
	text := strings.Repeat("字", 100001)
	assetID := insertAsset(t, st, dataDir, pid, "huge.txt", text)
	insertLLMKey(t, st, master, uid, "openai", "http://127.0.0.1:1", "sk-test")

	_, err := extract.Run(context.Background(), st, &llm.Client{}, master, uid, extract.ExtractInput{
		ProjectID: pid,
		AssetIDs:  []string{assetID},
		Name:      "过大",
	}, nil)
	if err == nil {
		t.Fatal("expected oversize error")
	}
	if err.Error() != "invalid: sample too large" {
		t.Fatalf("error: %v", err)
	}
}

func openExtractStore(t *testing.T) (*store.Store, string, []byte, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	master := make([]byte, 32)
	uid := ids.New("usr_")
	pid := ids.New("prj_")
	now := time.Now().UTC().Format(time.RFC3339)
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	_, err = st.DB().Exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`, uid, uid+"@example.com", hash, now)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = st.DB().Exec(`INSERT INTO projects (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`, pid, uid, "p", now)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return st, dataDir, master, uid, pid
}

func insertAsset(t *testing.T, st *store.Store, dataDir, projectID, filename, text string) string {
	t.Helper()
	data := []byte(text)
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	blobPath := filepath.Join(dataDir, "blobs", sumHex)
	if err := os.WriteFile(blobPath, data, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	id := ids.New("ast_")
	chapters := extract.SplitChapters(text)
	_, err := st.DB().Exec(
		`INSERT INTO assets (id, project_id, filename, sha256, rune_count, chapter_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, filename, sumHex, utf8.RuneCountInString(text), len(chapters),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	return id
}

func insertLLMKey(t *testing.T, st *store.Store, master []byte, userID, provider, baseURL, apiKey string) {
	t.Helper()
	blob, err := cryptokey.Seal(master, apiKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = st.DB().Exec(
		`INSERT INTO user_llm_keys (user_id, provider, base_url, encrypted_key, last4, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, provider, baseURL, blob, last4(apiKey), now, now,
	)
	if err != nil {
		t.Fatalf("insert key: %v", err)
	}
}

func fakeLLMServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func validExtractJSON() string {
	fix := card.ValidFixture("extracted")
	raw, err := json.Marshal(map[string]any{
		"dimensions":   fix.Dimensions,
		"prohibitions": fix.Prohibitions,
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func last4(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 4 {
		return s
	}
	return string([]rune(s)[n-4:])
}
