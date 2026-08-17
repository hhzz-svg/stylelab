package fuse_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"unicode/utf8"

	"stylelab/internal/auth"
	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/fuse"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

func TestRunPersistsFusedCardAndKeepsBlendLevels(t *testing.T) {
	st, master, uid, pid := openFuseStore(t)
	a := parentCard(pid, "crd_aaaaaaaaaaaaaaaa", 80, "A 节奏")
	b := parentCard(pid, "crd_bbbbbbbbbbbbbbbb", 20, "B 节奏")
	insertCard(t, st, a)
	insertCard(t, st, b)

	rewritten := card.ValidFixture("extracted")
	setDim(&rewritten, "sentence_rhythm", func(d *card.Dimension) {
		d.Level = 1
		d.Summary = "融合后的节奏摘要"
		d.Techniques = []string{"短句收束", "停顿对照", "信息后置"}
	})
	content, err := json.Marshal(map[string]any{
		"dimensions":   rewritten.Dimensions,
		"prohibitions": []string{"LLM 不该覆盖这条"},
		"conflicts":    []string{"句式密度与留白冲突"},
	})
	if err != nil {
		t.Fatalf("marshal llm: %v", err)
	}
	srv := fakeLLMServer(t, string(content))
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-fuse-key")

	spec := fuse.FuseSpec{
		Name:  "融合样本",
		Model: "gpt-4o-mini",
		Parents: []card.ParentRef{
			{CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 60}},
			{CardID: b.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 40}},
		},
	}
	got, conflicts, err := fuse.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, pid, spec, func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ids.Valid(got.ID, "crd_") {
		t.Fatalf("card id: %s", got.ID)
	}
	if got.Kind != "fused" || got.Version != 1 {
		t.Fatalf("kind/version: %s %d", got.Kind, got.Version)
	}
	if got.Name != "融合样本" {
		t.Fatalf("name: %s", got.Name)
	}
	if got.Lineage == nil || got.Lineage.PromptVersion != "fuse-v1" {
		t.Fatalf("lineage: %+v", got.Lineage)
	}
	if len(got.Lineage.ParentCards) != 2 {
		t.Fatalf("parents: %+v", got.Lineage.ParentCards)
	}
	if err := card.Validate(got); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.Dimensions["sentence_rhythm"].Level != 56 {
		t.Fatalf("level should stay blended 56, got %d", got.Dimensions["sentence_rhythm"].Level)
	}
	if got.Dimensions["sentence_rhythm"].Summary != "融合后的节奏摘要" {
		t.Fatalf("summary: %s", got.Dimensions["sentence_rhythm"].Summary)
	}
	if len(conflicts) != 1 || conflicts[0] != "句式密度与留白冲突" {
		t.Fatalf("conflicts: %v", conflicts)
	}
	if string(got.Facts) != "{}" && string(got.Facts) != "null" {
		// fused facts are an empty object
		var obj map[string]any
		if err := json.Unmarshal(got.Facts, &obj); err != nil || len(obj) != 0 {
			t.Fatalf("facts: %s", got.Facts)
		}
	}

	var storedKind string
	var storedVersion int
	err = st.DB().QueryRow(`SELECT kind, current_version FROM style_cards WHERE id = ? AND project_id = ?`, got.ID, pid).
		Scan(&storedKind, &storedVersion)
	if err != nil {
		t.Fatalf("style_cards: %v", err)
	}
	if storedKind != "fused" || storedVersion != 1 {
		t.Fatalf("stored card kind=%s version=%d", storedKind, storedVersion)
	}
	var lineageJSON string
	err = st.DB().QueryRow(`SELECT lineage_json FROM style_card_versions WHERE card_id = ? AND version = 1`, got.ID).
		Scan(&lineageJSON)
	if err != nil {
		t.Fatalf("lineage: %v", err)
	}
	if lineageJSON == "" || lineageJSON == "null" {
		t.Fatalf("expected lineage json, got %q", lineageJSON)
	}
}

func TestJobHandlerResultHasCardIDAndConflicts(t *testing.T) {
	st, master, uid, pid := openFuseStore(t)
	a := parentCard(pid, "crd_aaaaaaaaaaaaaaaa", 80, "A 节奏")
	b := parentCard(pid, "crd_bbbbbbbbbbbbbbbb", 20, "B 节奏")
	insertCard(t, st, a)
	insertCard(t, st, b)

	fix := card.ValidFixture("extracted")
	content, err := json.Marshal(map[string]any{
		"dimensions":   fix.Dimensions,
		"prohibitions": fix.Prohibitions,
		"conflicts":    []string{"冲突一条"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	srv := fakeLLMServer(t, string(content))
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-fuse-key")

	payload, err := json.Marshal(fuse.FuseSpec{
		Name: "融合样本",
		Parents: []card.ParentRef{
			{CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 60}},
			{CardID: b.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 40}},
		},
		Model: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	h := fuse.JobHandler(st, &llm.Client{HTTP: srv.Client()}, master)
	result, err := h(context.Background(), job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindFuse,
		Payload:   payload,
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("JobHandler: %v", err)
	}
	var out struct {
		CardID    string   `json:"card_id"`
		Conflicts []string `json:"conflicts"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("result json %s: %v", result, err)
	}
	if !ids.Valid(out.CardID, "crd_") {
		t.Fatalf("card_id: %q", out.CardID)
	}
	if len(out.Conflicts) != 1 || out.Conflicts[0] != "冲突一条" {
		t.Fatalf("conflicts: %v", out.Conflicts)
	}
}

func TestRunMissingLLMKey(t *testing.T) {
	st, master, uid, pid := openFuseStore(t)
	a := parentCard(pid, "crd_aaaaaaaaaaaaaaaa", 80, "A")
	b := parentCard(pid, "crd_bbbbbbbbbbbbbbbb", 20, "B")
	insertCard(t, st, a)
	insertCard(t, st, b)

	_, _, err := fuse.Run(context.Background(), st, &llm.Client{}, master, uid, pid, fuse.FuseSpec{
		Name: "无钥",
		Parents: []card.ParentRef{
			{CardID: a.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 60}},
			{CardID: b.ID, Version: 1, Dims: []string{"sentence_rhythm"}, Weights: map[string]int{"sentence_rhythm": 40}},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected missing llm key")
	}
	if err.Error() != "invalid: missing llm key" {
		t.Fatalf("error: %v", err)
	}
}

func parentCard(projectID, id string, rhythmLevel int, rhythmSummary string) card.Card {
	c := card.ValidFixture("extracted")
	c.ID = id
	c.ProjectID = projectID
	c.Name = id
	setDim(&c, "sentence_rhythm", func(d *card.Dimension) {
		d.Level = rhythmLevel
		d.Summary = rhythmSummary
	})
	return c
}

func setDim(c *card.Card, key string, mut func(*card.Dimension)) {
	d := c.Dimensions[key]
	mut(&d)
	c.Dimensions[key] = d
}

func openFuseStore(t *testing.T) (*store.Store, []byte, string, string) {
	t.Helper()
	st, err := store.Open(t.TempDir())
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
	return st, master, uid, pid
}

func insertCard(t *testing.T, st *store.Store, c card.Card) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	dims, err := json.Marshal(c.Dimensions)
	if err != nil {
		t.Fatalf("dims: %v", err)
	}
	prohibitions, err := json.Marshal(c.Prohibitions)
	if err != nil {
		t.Fatalf("prohibitions: %v", err)
	}
	facts := c.Facts
	if len(facts) == 0 {
		facts = json.RawMessage(`{}`)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Kind, c.Version, now, now,
	)
	if err != nil {
		t.Fatalf("insert style_cards: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, ?)`,
		c.ID, c.Version, string(dims), string(prohibitions), string(facts), now,
	)
	if err != nil {
		t.Fatalf("insert versions: %v", err)
	}
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

func last4(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 4 {
		return s
	}
	return string([]rune(s)[n-4:])
}
