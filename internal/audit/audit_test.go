package audit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"stylelab/internal/audit"
	"stylelab/internal/auth"
	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

func TestRunPersistsDualPersonaReport(t *testing.T) {
	st, master, uid, pid := openAuditStore(t)
	c := persistCard(t, st, pid)

	commercial := personaJSON(t, "commercial_web", "钩子密度够用", "章末拉力偏弱", "tension_hook", 72, "加强章末未决")
	literary := personaJSON(t, "literary_texture", "感官落点克制", "修辞略满", "rhetoric_preference", 38, "减修辞密度")
	srv := fakePersonaLLM(t, commercial, literary)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-audit-key")

	got, err := audit.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, c.ID, "gpt-4o-mini", func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ids.Valid(got.ID, "aud_") {
		t.Fatalf("audit id: %s", got.ID)
	}
	if got.CardID != c.ID {
		t.Fatalf("card_id: %s", got.CardID)
	}
	if got.CardVersion != c.Version {
		t.Fatalf("card_version: %d", got.CardVersion)
	}
	if len(got.Personas) != 2 {
		t.Fatalf("personas: %v", got.Personas)
	}
	if got.Personas[0] != "commercial_web" || got.Personas[1] != "literary_texture" {
		t.Fatalf("persona order: %v", got.Personas)
	}
	if _, ok := got.ByPersona["commercial_web"]; !ok {
		t.Fatalf("missing commercial_web: %+v", got.ByPersona)
	}
	if _, ok := got.ByPersona["literary_texture"]; !ok {
		t.Fatalf("missing literary_texture: %+v", got.ByPersona)
	}
	if got.ByPersona["commercial_web"].Strengths[0] != "钩子密度够用" {
		t.Fatalf("commercial strengths: %v", got.ByPersona["commercial_web"].Strengths)
	}
	if got.ByPersona["literary_texture"].Risks[0] != "修辞略满" {
		t.Fatalf("literary risks: %v", got.ByPersona["literary_texture"].Risks)
	}
	if len(got.Conflicts) == 0 {
		t.Fatal("expected conflicts")
	}
	if len(got.RecommendedEdits) == 0 {
		t.Fatal("expected recommended_edits")
	}

	var storedID, storedProject, storedCard, reportJSON string
	var storedVersion int
	err = st.DB().QueryRow(`SELECT id, project_id, card_id, card_version, report_json FROM audit_reports WHERE id = ?`, got.ID).
		Scan(&storedID, &storedProject, &storedCard, &storedVersion, &reportJSON)
	if err != nil {
		t.Fatalf("audit_reports: %v", err)
	}
	if storedProject != pid || storedCard != c.ID || storedVersion != c.Version {
		t.Fatalf("stored row project=%s card=%s ver=%d", storedProject, storedCard, storedVersion)
	}
	if !strings.Contains(reportJSON, `"commercial_web"`) || !strings.Contains(reportJSON, `"literary_texture"`) {
		t.Fatalf("persisted report missing persona keys: %s", reportJSON)
	}
	if llm.ForbiddenCopyCheck(reportJSON) {
		t.Fatalf("report contains banned copy: %s", reportJSON)
	}
}

func TestJobHandlerResultHasAuditID(t *testing.T) {
	st, master, uid, pid := openAuditStore(t)
	c := persistCard(t, st, pid)
	commercial := personaJSON(t, "commercial_web", "信息投放清楚", "对白推进偏慢", "dialogue_density", 60, "加快对白")
	literary := personaJSON(t, "literary_texture", "句层肌理稳", "留白不足", "scene_pacing", 45, "加留白")
	srv := fakePersonaLLM(t, commercial, literary)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-audit-key")

	payload, err := json.Marshal(map[string]string{
		"card_id": c.ID,
		"model":   "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	h := audit.JobHandler(st, &llm.Client{HTTP: srv.Client()}, master)
	result, err := h(context.Background(), job.Record{
		UserID:    uid,
		ProjectID: pid,
		Kind:      job.KindAudit,
		Payload:   payload,
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("JobHandler: %v", err)
	}
	var out struct {
		AuditID string `json:"audit_id"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("result json %s: %v", result, err)
	}
	if !ids.Valid(out.AuditID, "aud_") {
		t.Fatalf("audit_id: %q", out.AuditID)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM audit_reports WHERE id = ?`, out.AuditID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit_reports count: %d", n)
	}
}

func TestRunMissingLLMKey(t *testing.T) {
	st, master, uid, pid := openAuditStore(t)
	c := persistCard(t, st, pid)
	_, err := audit.Run(context.Background(), st, &llm.Client{}, master, uid, c.ID, "", nil)
	if err == nil {
		t.Fatal("expected missing llm key")
	}
	if err.Error() != "invalid: missing llm key" {
		t.Fatalf("error: %v", err)
	}
}

func openAuditStore(t *testing.T) (*store.Store, []byte, string, string) {
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

func personaJSON(t *testing.T, persona, strength, risk, dim string, level int, reason string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"strengths":   []string{strength},
		"risks":       []string{risk},
		"suggestions": []string{reason},
		"conflicts": []map[string]string{
			{"dimension": dim, "summary": persona + " 与另一视角在 " + dim + " 上张力不同"},
		},
		"recommended_edits": []map[string]any{
			{"dimension": dim, "target_level": level, "reason": reason},
		},
	})
	if err != nil {
		t.Fatalf("persona json: %v", err)
	}
	return string(raw)
}

func fakePersonaLLM(t *testing.T, commercial, literary string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		payload := string(raw)
		content := commercial
		switch {
		case strings.Contains(payload, "literary_texture"):
			content = literary
			mu.Lock()
			seen["literary_texture"]++
			mu.Unlock()
		case strings.Contains(payload, "commercial_web"):
			content = commercial
			mu.Lock()
			seen["commercial_web"]++
			mu.Unlock()
		default:
			http.Error(w, "unknown persona", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(func() {
		srv.Close()
		mu.Lock()
		defer mu.Unlock()
		if seen["commercial_web"] == 0 || seen["literary_texture"] == 0 {
			t.Errorf("expected both persona chats, got %v", seen)
		}
	})
	return srv
}

func last4(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 4 {
		return s
	}
	return string([]rune(s)[n-4:])
}
