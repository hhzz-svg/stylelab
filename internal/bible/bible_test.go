package bible_test

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
	"stylelab/internal/bible"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/write"
)

func TestValidateNameAndKind(t *testing.T) {
	if _, err := bible.ValidateName("   "); err == nil {
		t.Fatal("empty name should fail")
	}
	if _, err := bible.ValidateName(strings.Repeat("沈", 41)); err == nil {
		t.Fatal("long name should fail")
	}
	if _, err := bible.ValidateKind("weapon"); err == nil {
		t.Fatal("unknown kind should fail")
	}
	if _, err := bible.ValidateKind(" character "); err != nil {
		t.Fatalf("trimmed kind: %v", err)
	}
	if _, err := bible.ValidateContent(strings.Repeat("字", 2001)); err == nil {
		t.Fatal("long content should fail")
	}
	if _, err := bible.ValidateStatus("done"); err == nil {
		t.Fatal("unknown status should fail")
	}
}

func TestRenderForPrompt(t *testing.T) {
	if got := bible.RenderForPrompt(nil); got != "" {
		t.Fatalf("empty entries should render empty, got %q", got)
	}
	entries := []bible.Entry{
		{Kind: bible.KindThread, Name: "门后的信", Content: "第 3 章已烧毁", Status: bible.StatusResolved},
		{Kind: bible.KindCharacter, Name: "沈砚", Content: "冷面剑客，左臂有旧伤"},
		{Kind: bible.KindSetting, Name: "旧宅", Content: "城西，雨天积水"},
	}
	got := bible.RenderForPrompt(entries)
	for _, need := range []string{"【人物】", "沈砚：冷面剑客", "【设定】", "旧宅", "【伏笔】", "门后的信（已收线）"} {
		if !strings.Contains(got, need) {
			t.Fatalf("render missing %q:\n%s", need, got)
		}
	}
	if strings.Index(got, "【人物】") > strings.Index(got, "【伏笔】") {
		t.Fatal("characters should come before threads")
	}

	// 每条截断：400 字 content 只保留 240 字（header 文案"复述"自带 1 个"述"，故 241）
	long := []bible.Entry{{Kind: bible.KindCharacter, Name: "长卷", Content: strings.Repeat("述", 400)}}
	rendered := bible.RenderForPrompt(long)
	if got := strings.Count(rendered, "述"); got != 241 {
		t.Fatalf("per-entry clamp: want 241 述 (240 content + 1 header), got %d", got)
	}

	// 整块上限：大量条目时提前截止
	many := make([]bible.Entry, 60)
	for i := range many {
		many[i] = bible.Entry{Kind: bible.KindCharacter, Name: "人物" + strings.Repeat("〇", 30) + string(rune('A'+i%26)), Content: strings.Repeat("事", 240)}
	}
	if n := utf8.RuneCountInString(bible.RenderForPrompt(many)); n > 4100 {
		t.Fatalf("total clamp exceeded: %d runes", n)
	}
}

func TestParseOps(t *testing.T) {
	raw := "```json\n{\"ops\":[" +
		"{\"op\":\"create\",\"kind\":\"character\",\"name\":\"沈砚\",\"content\":\"剑客\"}," +
		"{\"op\":\"create\",\"kind\":\"weapon\",\"name\":\"刀\",\"content\":\"\"}," +
		"{\"op\":\"create\",\"kind\":\"setting\",\"name\":\"  \"}," +
		"{\"op\":\"update\",\"id\":\"bib_missing001\",\"content\":\"x\"}," +
		"{\"op\":\"update\",\"id\":\"not-a-bib-id\",\"content\":\"x\"}," +
		"{\"op\":\"update\",\"id\":\"bib_" + strings.Repeat("0", 16) + "\",\"content\":\"y\",\"status\":\"bogus\"}," +
		"{\"op\":\"delete\",\"id\":\"bib_" + strings.Repeat("1", 16) + "\"}" +
		"]}\n```"
	ops, err := bible.ParseOps(raw)
	if err != nil {
		t.Fatalf("ParseOps: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("want 2 valid ops, got %d: %+v", len(ops), ops)
	}
	if ops[0].Op != "create" || ops[0].Name != "沈砚" {
		t.Fatalf("op0: %+v", ops[0])
	}
	if ops[1].Op != "update" || ops[1].Status != "" {
		t.Fatalf("invalid status should be cleared: %+v", ops[1])
	}

	if _, err := bible.ParseOps("这不是 JSON"); err == nil {
		t.Fatal("garbage should fail")
	}
	ops, err = bible.ParseOps(`{"ops":[]}`)
	if err != nil || len(ops) != 0 {
		t.Fatalf("empty ops: %v %+v", err, ops)
	}
}

func TestSyncChapterAndApplyOps(t *testing.T) {
	st, master, uid, pid := openStore(t)
	existing := insertEntry(t, st, pid, bible.KindCharacter, "沈砚", "冷面剑客")
	_ = existing
	var captured []string
	opsJSON := `{"ops":[` +
		`{"op":"create","kind":"character","name":"阿九","content":"沈砚的师妹"},` +
		`{"op":"create","kind":"character","name":"沈砚","content":"冷面剑客，第 2 章受重伤"},` +
		`{"op":"update","id":"bib_` + strings.Repeat("9", 16) + `","content":"幽灵条目"}` +
		`]}`
	srv := fakeLLM(t, &captured, opsJSON)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-bible-key")

	entries, err := bible.ListByProject(context.Background(), st, uid, pid)
	if err != nil {
		t.Fatal(err)
	}
	ops, err := bible.SyncChapter(context.Background(), &llm.Client{HTTP: srv.Client()}, bible.Key{
		Provider: "openai", BaseURL: srv.URL, APIKey: "sk-bible-key",
	}, "", entries, 2, "灯下", "沈砚进门，阿九递伞。")
	if err != nil {
		t.Fatalf("SyncChapter: %v", err)
	}
	if len(ops) != 3 {
		t.Fatalf("want 3 parsed ops, got %d: %+v", len(ops), ops)
	}
	if !strings.Contains(captured[0], "冷面剑客") || !strings.Contains(captured[0], "灯下") {
		t.Fatalf("sync prompt missing entries or chapter: %s", captured[0])
	}

	created, updated, err := bible.ApplyOps(context.Background(), st, uid, pid, 2, ops)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	if created != 1 || updated != 1 {
		t.Fatalf("want created=1 updated=1, got %d/%d", created, updated)
	}
	after, err := bible.ListByProject(context.Background(), st, uid, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("want 2 entries after apply, got %d", len(after))
	}
	for _, e := range after {
		if e.Origin != bible.OriginAuto || e.SourceSeq != 2 {
			t.Fatalf("entry %s should be auto/seq2: %+v", e.Name, e)
		}
		switch e.Name {
		case "阿九":
			if e.Content != "沈砚的师妹" {
				t.Fatalf("阿九 content: %q", e.Content)
			}
		case "沈砚":
			if !strings.Contains(e.Content, "受重伤") {
				t.Fatalf("沈砚 should be updated in place: %q", e.Content)
			}
		default:
			t.Fatalf("unexpected entry %q", e.Name)
		}
	}
}

func TestRunSyncBackfill(t *testing.T) {
	st, master, uid, pid := openStore(t)
	c := insertCard(t, st, pid)
	ch1 := insertChapter(t, st, pid, c, 1, "旧宅", "雨夜到门")
	ch2 := insertChapter(t, st, pid, c, 2, "灯下", "老人开口")
	markWritten(t, st, ch1.ID, "他推门进了旧宅。", "他进了旧宅。")
	markWritten(t, st, ch2.ID, "灯很暗。老人坐着。", "老人开口了。")

	// 第二章的回答里 update 指向不存在的 id（被忽略），只应新建"老人"。
	// 第二章的 prompt 应带上第一章建立的"旧宅"条目（渐进式状态）。
	var captured []string
	srv := fakeLLM(t, &captured,
		`{"ops":[{"op":"create","kind":"setting","name":"旧宅","content":"城西老宅"}]}`,
		`{"ops":[{"op":"create","kind":"character","name":"老人","content":"守宅人"},{"op":"update","id":"bib_`+strings.Repeat("9", 16)+`","content":"幽灵条目"}]}`,
	)
	insertLLMKey(t, st, master, uid, "openai", srv.URL, "sk-bible-key")
	res, err := bible.RunSync(context.Background(), st, &llm.Client{HTTP: srv.Client()}, master, uid, pid, bible.SyncInput{}, func(int, string) {})
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if res.ChaptersSynced != 2 {
		t.Fatalf("chapters synced: %d", res.ChaptersSynced)
	}
	if res.Entries != 2 {
		t.Fatalf("entries: %d", res.Entries)
	}
	if len(captured) != 2 {
		t.Fatalf("want 2 llm calls, got %d", len(captured))
	}
	if !strings.Contains(captured[1], "旧宅") {
		t.Fatalf("second call should see current entries: %s", captured[1])
	}
}

func TestRunSyncNoWrittenChapters(t *testing.T) {
	st, master, uid, pid := openStore(t)
	insertLLMKey(t, st, master, uid, "openai", "http://127.0.0.1:1", "sk-bible-key")
	_, err := bible.RunSync(context.Background(), st, &llm.Client{}, master, uid, pid, bible.SyncInput{}, nil)
	if err == nil || !strings.Contains(err.Error(), "no written chapters") {
		t.Fatalf("want no written chapters error, got %v", err)
	}
}

func openStore(t *testing.T) (*store.Store, []byte, string, string) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
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

func insertEntry(t *testing.T, st *store.Store, projectID, kind, name, content string) bible.Entry {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	e := bible.Entry{
		ID:        ids.New(bible.Prefix),
		ProjectID: projectID,
		Kind:      kind,
		Name:      name,
		Content:   content,
		Status:    bible.StatusActive,
		Origin:    bible.OriginManual,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := bible.Insert(context.Background(), st, e); err != nil {
		t.Fatal(err)
	}
	return e
}

func insertCard(t *testing.T, st *store.Store, projectID string) string {
	t.Helper()
	id := ids.New("crd_")
	now := time.Now().UTC().Format(time.RFC3339)
	dims := `{"sentence_rhythm":{"level":50,"summary":"s","techniques":["a","b","c"]}}`
	if _, err := st.DB().Exec(
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, 'card', 'extracted', 1, ?, ?)`, id, projectID, now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, 1, ?, '[]', '{}', NULL, ?)`, id, dims, now,
	); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertChapter(t *testing.T, st *store.Store, projectID, cardID string, seq int, title, brief string) write.Chapter {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	ch := write.Chapter{
		ID:        ids.New(write.Prefix),
		ProjectID: projectID,
		CardID:    cardID,
		Seq:       seq,
		Title:     title,
		Brief:     brief,
		Status:    write.StatusDraft,
		TargetRunes: 2500,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := write.Insert(context.Background(), st, ch); err != nil {
		t.Fatal(err)
	}
	return ch
}

func markWritten(t *testing.T, st *store.Store, chapterID, body, summary string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.DB().Exec(
		`UPDATE chapters SET body=?, summary=?, status=?, updated_at=? WHERE id=?`,
		body, summary, write.StatusWritten, now, chapterID,
	); err != nil {
		t.Fatal(err)
	}
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

func fakeLLM(t *testing.T, captured *[]string, answers ...string) *httptest.Server {
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
