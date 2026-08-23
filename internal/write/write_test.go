package write_test

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
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/write"
)

func TestValidateBrief(t *testing.T) {
	if _, err := write.ValidateBrief("   "); err == nil {
		t.Fatal("empty brief should fail")
	}
	if _, err := write.ValidateBrief(strings.Repeat("概", 201)); err == nil {
		t.Fatal("long brief should fail")
	}
	got, err := write.ValidateBrief("  雨夜归宅  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "雨夜归宅" {
		t.Fatalf("got %q", got)
	}
}

func TestClampTarget(t *testing.T) {
	if write.ClampTarget(0) != 2500 {
		t.Fatalf("default: %d", write.ClampTarget(0))
	}
	if write.ClampTarget(1999) != 2000 {
		t.Fatalf("min: %d", write.ClampTarget(1999))
	}
	if write.ClampTarget(4000) != 3500 {
		t.Fatalf("max: %d", write.ClampTarget(4000))
	}
	if write.ClampTarget(2800) != 2800 {
		t.Fatalf("passthrough: %d", write.ClampTarget(2800))
	}
}

func TestChapterUserPromptIncludesPreviousSummary(t *testing.T) {
	ch := write.Chapter{ID: "chp_this", Seq: 2, Title: "灯下", Brief: "老人开口"}
	siblings := []write.ChapterSummary{
		{ID: "chp_prev", Seq: 1, Title: "旧宅"},
		{ID: "chp_this", Seq: 2, Title: "灯下"},
	}
	prev := []write.Chapter{{
		Seq:     1,
		Title:   "旧宅",
		Summary: "他在雨里站了很久，终于推门。",
		Body:    strings.Repeat("尾", 40) + "门开了。",
	}}
	prompt := write.ChapterUserPrompt(ch, siblings, prev, "", "少写对白", 2500)
	for _, need := range []string{"第 2 章", "灯下", "老人开口", "少写对白", "2500", "旧宅", "他在雨里站了很久", "门开了。", "← 本章"} {
		if !strings.Contains(prompt, need) {
			t.Fatalf("prompt missing %q:\n%s", need, prompt)
		}
	}
}

func TestRunRejectsMissingKey(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	ch := insertChapter(t, st, pid, c.ID, 1, "旧宅", "雨夜到门")
	_, err := write.Run(context.Background(), st, &llm.Client{}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    2500,
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing llm key") {
		t.Fatalf("want missing llm key, got %v", err)
	}
}

func TestRunWritesBodyAndSummary(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	ch := insertChapter(t, st, pid, c.ID, 1, "旧宅", "雨夜到门")
	var captured []string
	srv := fakeWriteLLM(t, &captured, "雨还在下。他推开门。", "他推门进了旧宅。")
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-write-key")

	got, err := write.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    0,
		Note:      "克制",
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Body != "雨还在下。他推开门。" {
		t.Fatalf("body: %q", got.Body)
	}
	if got.Summary != "他推门进了旧宅。" {
		t.Fatalf("summary: %q", got.Summary)
	}
	if got.Status != write.StatusWritten {
		t.Fatalf("status: %s", got.Status)
	}
	if got.TargetRunes != 2500 {
		t.Fatalf("target: %d", got.TargetRunes)
	}
	if len(captured) < 2 {
		t.Fatalf("expected body+summary calls, got %d", len(captured))
	}
	if !strings.Contains(captured[0], "克制") || !strings.Contains(captured[0], "2500") {
		t.Fatalf("first call missing note/target: %s", captured[0])
	}
}

func TestRunUsesPreviousChapterInPrompt(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	prev := insertChapter(t, st, pid, c.ID, 1, "旧宅", "到门")
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := st.DB().Exec(
		`UPDATE chapters SET body=?, summary=?, status=? WHERE id=?`,
		"院子里全是水。门开了。", "他进了旧宅。", write.StatusWritten, prev.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	ch := insertChapter(t, st, pid, c.ID, 2, "灯下", "老人开口")
	_ = now
	var captured []string
	srv := fakeWriteLLM(t, &captured, "灯很暗。老人坐着。", "老人开口了。")
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-write-key")

	if _, err := write.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    2200,
	}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(captured[0], "他进了旧宅。") {
		t.Fatalf("prompt missing prev summary: %s", captured[0])
	}
	if !strings.Contains(captured[0], "门开了。") {
		t.Fatalf("prompt missing prev tail: %s", captured[0])
	}
}

func TestRunInjectsBibleIntoPrompt(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	insertBibleEntry(t, st, pid, "character", "沈砚", "冷面剑客，左臂有旧伤")
	insertBibleEntry(t, st, pid, "setting", "旧宅", "城西，雨天积水")
	ch := insertChapter(t, st, pid, c.ID, 1, "旧宅", "雨夜到门")
	var captured []string
	srv := fakeWriteLLM(t, &captured, "他推门进了旧宅。", "他进了旧宅。", `{"ops":[]}`)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-write-key")

	if _, err := write.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    2500,
	}, func(int, string) {}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(captured) < 1 {
		t.Fatalf("no llm calls captured")
	}
	// 章节生成的第一个请求应含设定集条目文本
	if !strings.Contains(captured[0], "沈砚") || !strings.Contains(captured[0], "冷面剑客") {
		t.Fatalf("chapter prompt missing bible entries: %s", captured[0])
	}
}

func TestRunSyncsBibleAfterWrite(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	ch := insertChapter(t, st, pid, c.ID, 1, "旧宅", "雨夜到门")
	// 第三个请求（bible 同步）返回一条新建人物
	srv := fakeWriteLLM(t, nil, "他推门进了旧宅。", "他进了旧宅。", `{"ops":[{"op":"create","kind":"character","name":"沈砚","content":"冷面剑客"}]}`)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-write-key")

	if _, err := write.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    2500,
	}, func(int, string) {}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	rows, err := st.DB().Query(`SELECT name, origin, source_seq FROM bible_entries WHERE project_id=?`, pid)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n, origin string
		var seq int
		if err := rows.Scan(&n, &origin, &seq); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
		if origin != "auto" || seq != 1 {
			t.Fatalf("bible entry %s should be auto/seq1: origin=%s seq=%d", n, origin, seq)
		}
	}
	if len(names) != 1 || names[0] != "沈砚" {
		t.Fatalf("want 1 auto bible entry 沈砚, got %v", names)
	}
}

func TestRunBibleSyncFailureDoesNotBreakWrite(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := persistCard(t, st, pid)
	ch := insertChapter(t, st, pid, c.ID, 1, "旧宅", "雨夜到门")
	// 第三个请求返回垃圾：ParseOps 失败重试三次都失败，syncBible 静默跳过
	srv := fakeWriteLLM(t, nil, "他推门进了旧宅。", "他进了旧宅。", "这根本不是 JSON")
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-write-key")

	got, err := write.Run(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, write.Input{
		ChapterID: ch.ID,
		Target:    2500,
	}, func(int, string) {})
	if err != nil {
		t.Fatalf("Run should succeed despite bible sync failure: %v", err)
	}
	if got.Status != write.StatusWritten {
		t.Fatalf("status should be written, got %s", got.Status)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM bible_entries WHERE project_id=?`, pid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("no bible entries should exist after sync failure, got %d", n)
	}
}

func insertBibleEntry(t *testing.T, st *store.Store, projectID, kind, name, content string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	id := ids.New("bib_")
	if _, err := st.DB().Exec(
		`INSERT INTO bible_entries (id, project_id, kind, name, content, status, origin, source_seq, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'active', 'manual', 0, ?, ?)`,
		id, projectID, kind, name, content, now, now,
	); err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T) (*store.Store, []byte, string, string) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	master := make([]byte, 32)
	uid := ids.New("usr_")
	pid := ids.New("prj_")
	now := time.Now().UTC().Format(time.RFC3339)
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`, uid, uid+"@example.com", hash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`INSERT INTO projects (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`, pid, uid, "p", now); err != nil {
		t.Fatal(err)
	}
	return st, master, uid, pid
}

func persistCard(t *testing.T, st *store.Store, projectID string) card.Card {
	t.Helper()
	c := card.ValidFixture("extracted")
	c.ID = ids.New("crd_")
	c.ProjectID = projectID
	now := time.Now().UTC().Format(time.RFC3339)
	dims, _ := json.Marshal(c.Dimensions)
	prohibitions, _ := json.Marshal(c.Prohibitions)
	facts := c.Facts
	if len(facts) == 0 {
		facts = json.RawMessage(`{}`)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Kind, c.Version, now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, ?)`,
		c.ID, c.Version, string(dims), string(prohibitions), string(facts), now,
	); err != nil {
		t.Fatal(err)
	}
	return c
}

func insertChapter(t *testing.T, st *store.Store, projectID, cardID string, seq int, title, brief string) write.Chapter {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	ch := write.Chapter{
		ID:          ids.New(write.Prefix),
		ProjectID:   projectID,
		CardID:      cardID,
		Seq:         seq,
		Title:       title,
		Brief:       brief,
		Status:      write.StatusDraft,
		TargetRunes: 2500,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := write.Insert(context.Background(), st, ch); err != nil {
		t.Fatal(err)
	}
	return ch
}

func insertLLMKey(t *testing.T, st *store.Store, master []byte, userID, provider, baseURL, apiKey string) {
	t.Helper()
	blob, err := cryptokey.Seal(master, apiKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.DB().Exec(
		`INSERT INTO user_llm_keys (user_id, provider, base_url, encrypted_key, last4, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, provider, baseURL, blob, last4(apiKey), now, now,
	); err != nil {
		t.Fatal(err)
	}
}

func fakeWriteLLM(t *testing.T, captured *[]string, answers ...string) *httptest.Server {
	t.Helper()
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if captured != nil {
			*captured = append(*captured, string(raw))
		}
		text := ""
		if i < len(answers) {
			text = answers[i]
		}
		i++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": text}},
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
