package sample_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"stylelab/internal/auth"
	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/sample"
	"stylelab/internal/store"
	"stylelab/internal/stylestat"
)

func TestRunRejectsEmptyPremise(t *testing.T) {
	st, master, uid, pid := openSampleStore(t)
	c := persistCard(t, st, pid)
	_, err := sample.Run(context.Background(), st, &llm.Client{}, master, uid, sample.Input{
		CardID:  c.ID,
		Premise: "   ",
		Target:  1200,
	}, nil)
	if err == nil {
		t.Fatal("expected empty premise error")
	}
	if !strings.Contains(err.Error(), "premise") {
		t.Fatalf("error: %v", err)
	}
}

func TestRunRejectsLongPremise(t *testing.T) {
	st, master, uid, pid := openSampleStore(t)
	c := persistCard(t, st, pid)
	_, err := sample.Run(context.Background(), st, &llm.Client{}, master, uid, sample.Input{
		CardID:  c.ID,
		Premise: strings.Repeat("前", 81),
		Target:  1200,
	}, nil)
	if err == nil {
		t.Fatal("expected long premise error")
	}
	if !strings.Contains(err.Error(), "premise") {
		t.Fatalf("error: %v", err)
	}
}

func TestRunDefaultsTargetRunes(t *testing.T) {
	st, master, uid, pid := openSampleStore(t)
	c := persistCard(t, st, pid)
	var captured string
	srv := fakeSampleLLM(t, "雨停了。门口的灯还亮着。", &captured)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-sample-key")

	got, err := sample.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, sample.Input{
		CardID:  c.ID,
		Premise: "雨夜有人敲门",
		Target:  0,
		Model:   "gpt-4o-mini",
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Target != 1200 {
		t.Fatalf("default target: %d", got.Target)
	}
	if !strings.Contains(captured, "1200") {
		t.Fatalf("llm request missing default target: %s", captured)
	}
}

func TestRunPersistsSample(t *testing.T) {
	st, master, uid, pid := openSampleStore(t)
	c := persistCard(t, st, pid)
	body := "门开了一条缝。风带着湿气进来，灯火在墙上晃了一下。"
	var captured string
	srv := fakeSampleLLM(t, body, &captured)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-sample-key")

	got, err := sample.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, sample.Input{
		CardID:  c.ID,
		Premise: "雨夜有人敲门",
		Target:  800,
		Model:   "gpt-4o-mini",
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ids.Valid(got.ID, "smp_") {
		t.Fatalf("sample id: %s", got.ID)
	}
	if got.CardID != c.ID {
		t.Fatalf("card_id: %s", got.CardID)
	}
	if got.CardVersion != c.Version {
		t.Fatalf("card_version: %d", got.CardVersion)
	}
	if got.Premise != "雨夜有人敲门" {
		t.Fatalf("premise: %s", got.Premise)
	}
	if got.Body != body {
		t.Fatalf("body: %s", got.Body)
	}
	if got.Target != 800 {
		t.Fatalf("target: %d", got.Target)
	}

	var facts stylestat.Stats
	if err := json.Unmarshal(got.Facts, &facts); err != nil {
		t.Fatalf("facts: %v", err)
	}
	if facts.Chapters != 1 {
		t.Fatalf("facts.chapters: %d", facts.Chapters)
	}
	if !facts.SampleTooSmall {
		t.Fatalf("expected sample_too_small for one chapter: %+v", facts)
	}

	if !strings.Contains(captured, "风格卡片") {
		t.Fatalf("SampleSystem not used: %s", captured)
	}
	if !strings.Contains(captured, c.Name) && !strings.Contains(captured, "sentence_rhythm") {
		t.Fatalf("card json missing from prompt: %s", captured)
	}

	var storedID, storedProject, storedCard, storedPremise, storedBody, factsJSON string
	var storedVersion int
	err = st.DB().QueryRow(
		`SELECT id, project_id, card_id, card_version, premise, body, facts_json FROM samples WHERE id = ?`,
		got.ID,
	).Scan(&storedID, &storedProject, &storedCard, &storedVersion, &storedPremise, &storedBody, &factsJSON)
	if err != nil {
		t.Fatalf("samples: %v", err)
	}
	if storedProject != pid || storedCard != c.ID || storedVersion != c.Version {
		t.Fatalf("stored row project=%s card=%s ver=%d", storedProject, storedCard, storedVersion)
	}
	if storedPremise != "雨夜有人敲门" || storedBody != body {
		t.Fatalf("stored premise/body: %s / %s", storedPremise, storedBody)
	}
	if !strings.Contains(factsJSON, `"sample_too_small"`) {
		t.Fatalf("persisted facts missing sample_too_small: %s", factsJSON)
	}
}

func TestJobHandlerResultHasSampleID(t *testing.T) {
	st, master, uid, pid := openSampleStore(t)
	c := persistCard(t, st, pid)
	srv := fakeSampleLLM(t, "灯还亮着。", nil)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-sample-key")

	payload, err := json.Marshal(sample.Input{
		CardID:  c.ID,
		Premise: "雨夜有人敲门",
		Target:  900,
		Model:   "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	h := sample.JobHandler(st, &llm.Client{HTTP: srv.Client()}, master)
	result, err := h(context.Background(), job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindSample,
		Payload:   payload,
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("JobHandler: %v", err)
	}
	var out struct {
		SampleID string `json:"sample_id"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("result json %s: %v", result, err)
	}
	if !ids.Valid(out.SampleID, "smp_") {
		t.Fatalf("sample_id: %q", out.SampleID)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM samples WHERE id = ?`, out.SampleID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("samples count: %d", n)
	}
}

func openSampleStore(t *testing.T) (*store.Store, []byte, string, string) {
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

func persistCard(t *testing.T, st *store.Store, projectID string) card.Card {
	t.Helper()
	c := card.ValidFixture("extracted")
	c.ID = ids.New("crd_")
	c.ProjectID = projectID
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
	return c
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

func fakeSampleLLM(t *testing.T, body string, captured *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if captured != nil {
			*captured = string(raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": body}},
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
