package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"stylelab/internal/card"
	"stylelab/internal/config"
	"stylelab/internal/httpapi"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/studio"
)

// The studio handlers (outline, continuity radar, branch simulator) call the
// LLM inline rather than through a job, and httpapi.New builds its own
// *llm.Client, so there is no seam to inject a stub. There is no need for one:
// the base URL travels from the user's stored BYOK key into llm.OpenAIURL, so
// pointing a key at a local server is enough to both stub the reply and observe
// the request that was sent.

type fakeLLM struct {
	srv   *httptest.Server
	mu    sync.Mutex
	calls []map[string]any
	reply string
}

func newFakeLLM(t *testing.T, reply string) *fakeLLM {
	t.Helper()
	f := &fakeLLM{reply: reply}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("fake llm: decode request: %v", err)
		}
		f.mu.Lock()
		f.calls = append(f.calls, body)
		current := f.reply
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": current}},
			},
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeLLM) setReply(reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reply = reply
}

func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// lastCall returns the most recent request body the fake received.
func (f *fakeLLM) lastCall(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("fake llm: no calls recorded")
	}
	return f.calls[len(f.calls)-1]
}

// lastUserPrompt returns the content of the last user message sent to the model.
func (f *fakeLLM) lastUserPrompt(t *testing.T) string {
	t.Helper()
	body := f.lastCall(t)
	msgs, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("fake llm: messages missing, got %T", body["messages"])
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m, ok := msgs[i].(map[string]any)
		if !ok {
			continue
		}
		if m["role"] == "user" {
			s, _ := m["content"].(string)
			return s
		}
	}
	t.Fatal("fake llm: no user message")
	return ""
}

func newStudioServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	t.Setenv("STYLELAB_DEV_INSECURE_KEY", "1")
	t.Setenv("STYLELAB_MASTER_KEY", "")
	t.Setenv("STYLELAB_DATA_DIR", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.DataDir = t.TempDir()
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	runner := job.NewRunner(st, 2)
	// The studio features run as jobs now, so the suite registers the real
	// handlers; the LLM is stubbed through the BYOK base_url instead.
	client := &llm.Client{}
	runner.Register(job.KindOutline, studio.OutlineJobHandler(st, client, cfg.MasterKey))
	runner.Register(job.KindContinuity, studio.ContinuityJobHandler(st, client, cfg.MasterKey))
	runner.Register(job.KindBranch, studio.BranchJobHandler(st, client, cfg.MasterKey))
	runner.Register(job.KindContinue, studio.ContinueJobHandler(st, client, cfg.MasterKey))
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)
	return srv, st
}

// startStudioJob posts to a studio route and returns the queued job id.
func startStudioJob(t *testing.T, srv *httptest.Server, c *http.Client, path, payload string) string {
	t.Helper()
	resp := postJSON(t, c, srv.URL+path, payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: status %d, want 202; body %s", path, resp.StatusCode, body)
	}
	id, _ := decodeJSON(t, resp)["job_id"].(string)
	if id == "" {
		t.Fatalf("%s: no job_id", path)
	}
	return id
}

// awaitJob polls until the job reaches a terminal state and returns the record.
func awaitJob(t *testing.T, srv *httptest.Server, c *http.Client, jobID string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp := mustGet(t, c, srv.URL+"/api/jobs/"+jobID)
		rec := decodeJSON(t, resp)
		resp.Body.Close()
		switch rec["status"] {
		case "succeeded", "failed", "canceled":
			return rec
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s never reached a terminal state", jobID)
	return nil
}

// runStudioJob posts and waits, asserting the job succeeded, then returns its
// result payload.
func runStudioJob(t *testing.T, srv *httptest.Server, c *http.Client, path, payload string) map[string]any {
	t.Helper()
	rec := awaitJob(t, srv, c, startStudioJob(t, srv, c, path, payload))
	if rec["status"] != "succeeded" {
		t.Fatalf("%s: job %v, error=%v", path, rec["status"], rec["error"])
	}
	result, _ := rec["result"].(map[string]any)
	return result
}

func registerUser(t *testing.T, srv *httptest.Server, email string) *http.Client {
	t.Helper()
	c := clientWithJar(t)
	resp := postJSON(t, c, srv.URL+"/api/auth/register", fmt.Sprintf(`{"email":%q,"password":"password1"}`, email))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: %d", email, resp.StatusCode)
	}
	return c
}

func createProject(t *testing.T, srv *httptest.Server, c *http.Client, name string) string {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects", fmt.Sprintf(`{"name":%q}`, name))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("create project: no id in %v", body)
	}
	return id
}

// useFakeLLM stores a BYOK key whose base URL points at the fake server, so
// every inline LLM call this user makes is served by it.
func useFakeLLM(t *testing.T, srv *httptest.Server, c *http.Client, f *fakeLLM) {
	t.Helper()
	resp := putJSON(t, c, srv.URL+"/api/me/llm-keys",
		fmt.Sprintf(`{"provider":"chat","base_url":%q,"api_key":"sk-test-abcd1234"}`, f.srv.URL))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put llm key: %d", resp.StatusCode)
	}
}

func createChapter(t *testing.T, srv *httptest.Server, c *http.Client, projectID, title, brief string) string {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/chapters",
		fmt.Sprintf(`{"title":%q,"brief":%q}`, title, brief))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create chapter: %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("create chapter: no id in %v", body)
	}
	return id
}

func listChapters(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) []any {
	t.Helper()
	resp := mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/chapters")
	defer resp.Body.Close()
	body := decodeJSON(t, resp)
	chapters, _ := body["chapters"].([]any)
	return chapters
}

func validOutlineJSON(n int) string {
	items := make([]map[string]any, 0, n)
	for i := 1; i <= n; i++ {
		items = append(items, map[string]any{
			"title": fmt.Sprintf("第%d章", i),
			"brief": fmt.Sprintf("第 %d 章的梗概内容", i),
			"hook":  "",
		})
	}
	raw, _ := json.Marshal(map[string]any{
		"synopsis": "测试梗概",
		"volumes": []map[string]any{
			{"volume_index": 1, "volume_title": "第一卷", "volume_brief": "开局", "chapters": items},
		},
	})
	return string(raw)
}

func validContinuityJSON() string {
	raw, _ := json.Marshal(map[string]any{
		"score":   82,
		"overall": "整体连贯",
		"issues": []map[string]any{
			{"severity": "warning", "category": "foreshadow", "title": "伏笔未收",
				"description": "古灯残魂未交代", "suggestion": "在后续章节收回", "location": "第1章"},
		},
		"foreshadows": []string{"第1章古灯残魂"},
	})
	return string(raw)
}

func validBranchJSON() string {
	raw, _ := json.Marshal(map[string]any{
		"current_analysis": "当前剧情分析",
		"branches": []map[string]any{
			{"id": "A", "type": "突围", "title": "正面突破", "direction": "主角正面迎敌",
				"plot_points": []string{"设伏", "反击"}, "sample_opening": "他握紧了剑。"},
		},
	})
	return string(raw)
}

// --- Defect A: an omitted model must not reach the provider as "" ------------

func TestStudioEndpointsSendDefaultModel(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-model@example.com")
	projectID := createProject(t, srv, owner, "模型默认值")

	fake := newFakeLLM(t, validOutlineJSON(2))
	useFakeLLM(t, srv, owner, fake)

	chapterID := createChapter(t, srv, owner, projectID, "第一章", "开篇梗概")

	cases := []struct {
		name    string
		path    string
		payload string
		reply   string
	}{
		{"outline", "/api/projects/" + projectID + "/outline/generate",
			`{"premise":"一个测试故事的梗概","target_chapters":2,"volume_count":1}`, validOutlineJSON(2)},
		{"continuity", "/api/projects/" + projectID + "/continuity-audit",
			`{}`, validContinuityJSON()},
		{"branch", "/api/chapters/" + chapterID + "/branch-simulate",
			`{"current_text":"正文片段"}`, validBranchJSON()},
		{"continue", "/api/chapters/" + chapterID + "/continue",
			`{"current_text":"正文片段","instruction":"继续写"}`, "续写出来的正文。"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake.setReply(tc.reply)
			runStudioJob(t, srv, owner, tc.path, tc.payload)
			model, _ := fake.lastCall(t)["model"].(string)
			if strings.TrimSpace(model) == "" {
				t.Fatalf("%s: model sent to provider was empty", tc.name)
			}
		})
	}
}

// The job result carries the feature's data, not an id pointing at a row.
func TestStudioJobResultsCarryTheirData(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-results@example.com")
	projectID := createProject(t, srv, owner, "结果载荷")
	fake := newFakeLLM(t, validOutlineJSON(3))
	useFakeLLM(t, srv, owner, fake)
	chapterID := createChapter(t, srv, owner, projectID, "第一章", "开篇梗概")

	outline := runStudioJob(t, srv, owner, "/api/projects/"+projectID+"/outline/generate",
		`{"premise":"一个测试故事的梗概","target_chapters":3,"volume_count":1}`)
	if got, _ := outline["synopsis"].(string); got != "测试梗概" {
		t.Fatalf("outline synopsis = %q", got)
	}
	volumes, _ := outline["volumes"].([]any)
	if len(volumes) != 1 {
		t.Fatalf("volumes = %d, want 1", len(volumes))
	}

	fake.setReply(validContinuityJSON())
	audit := runStudioJob(t, srv, owner, "/api/projects/"+projectID+"/continuity-audit", `{}`)
	if got := intFromJSON(audit["score"]); got != 82 {
		t.Fatalf("audit score = %d, want 82", got)
	}

	fake.setReply(validBranchJSON())
	branch := runStudioJob(t, srv, owner, "/api/chapters/"+chapterID+"/branch-simulate",
		`{"current_text":"正文"}`)
	branches, _ := branch["branches"].([]any)
	if len(branches) != 1 {
		t.Fatalf("branches = %d, want 1", len(branches))
	}

	fake.setReply("续写出来的正文。")
	cont := runStudioJob(t, srv, owner, "/api/chapters/"+chapterID+"/continue",
		`{"current_text":"正文","instruction":"继续"}`)
	if got, _ := cont["continued_text"].(string); got != "续写出来的正文。" {
		t.Fatalf("continued_text = %q", got)
	}
}

// A model reply that is not valid JSON must fail the job with a message, not
// hang or succeed empty.
func TestStudioJobSurfacesFailure(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-fail@example.com")
	projectID := createProject(t, srv, owner, "失败")
	fake := newFakeLLM(t, "这不是 JSON")
	useFakeLLM(t, srv, owner, fake)

	rec := awaitJob(t, srv, owner, startStudioJob(t, srv, owner,
		"/api/projects/"+projectID+"/outline/generate",
		`{"premise":"一个测试故事的梗概","target_chapters":2}`))
	if rec["status"] != "failed" {
		t.Fatalf("status = %v, want failed", rec["status"])
	}
	if msg, _ := rec["error"].(string); msg == "" {
		t.Fatal("failed job should carry an error message")
	}
}

func TestBranchFallsBackToChapterModel(t *testing.T) {
	srv, st := newStudioServer(t)
	owner := registerUser(t, srv, "studio-chmodel@example.com")
	projectID := createProject(t, srv, owner, "章节模型")
	fake := newFakeLLM(t, validBranchJSON())
	useFakeLLM(t, srv, owner, fake)

	chapterID := createChapter(t, srv, owner, projectID, "第一章", "开篇梗概")
	// chapters.model is written by the write job, not by PATCH, so pin it
	// directly to stand in for a chapter that has already been written once.
	if _, err := st.DB().Exec(`UPDATE chapters SET model = ? WHERE id = ?`, "gpt-4.1-mini", chapterID); err != nil {
		t.Fatalf("pin chapter model: %v", err)
	}

	runStudioJob(t, srv, owner, "/api/chapters/"+chapterID+"/branch-simulate", `{"current_text":"正文"}`)
	if got, _ := fake.lastCall(t)["model"].(string); got != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want the chapter's own model", got)
	}
}

// --- Defect G: prompts must stay bounded on a long manuscript ----------------

func TestContinuityAuditBoundsChapterContext(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-bounds@example.com")
	projectID := createProject(t, srv, owner, "长篇")
	fake := newFakeLLM(t, validContinuityJSON())
	useFakeLLM(t, srv, owner, fake)

	longBrief := strings.Repeat("长", 190)
	for i := 1; i <= 80; i++ {
		createChapter(t, srv, owner, projectID, fmt.Sprintf("第%d章", i), longBrief)
	}

	runStudioJob(t, srv, owner, "/api/projects/"+projectID+"/continuity-audit", `{}`)

	prompt := fake.lastUserPrompt(t)
	if n := strings.Count(prompt, "【第"); n > 60 {
		t.Fatalf("prompt carried %d chapters, want at most 60", n)
	}
	// Each entry is clamped to 180 runes, so 60 entries plus scaffolding stays
	// well under this ceiling. An unbounded build would blow straight past it.
	if n := len([]rune(prompt)); n > 30000 {
		t.Fatalf("prompt is %d runes, want a bounded prompt", n)
	}
	if !strings.Contains(prompt, "仅审计前") {
		t.Fatal("prompt should tell the model the audit was truncated")
	}
}

func TestBranchSimulateBoundsPriorContext(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-branchbounds@example.com")
	projectID := createProject(t, srv, owner, "推演边界")
	fake := newFakeLLM(t, validBranchJSON())
	useFakeLLM(t, srv, owner, fake)

	var lastID string
	for i := 1; i <= 10; i++ {
		id := createChapter(t, srv, owner, projectID, fmt.Sprintf("第%d章", i), "梗概")
		resp := patchJSON(t, owner, srv.URL+"/api/chapters/"+id,
			fmt.Sprintf(`{"summary":"第 %d 章的摘要"}`, i))
		resp.Body.Close()
		lastID = id
	}

	huge := strings.Repeat("字", 5000)
	runStudioJob(t, srv, owner, "/api/chapters/"+lastID+"/branch-simulate",
		fmt.Sprintf(`{"current_text":%q}`, huge))

	prompt := fake.lastUserPrompt(t)
	if n := strings.Count(prompt, "章的摘要"); n > 3 {
		t.Fatalf("prompt carried %d prior summaries, want at most 3", n)
	}
	if strings.Contains(prompt, strings.Repeat("字", 1201)) {
		t.Fatal("prompt carried more than the 1200-rune tail of the draft")
	}
}

// --- Defect E: replace must not destroy finished chapters --------------------

func TestOutlineImportAppendsChapters(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-import@example.com")
	projectID := createProject(t, srv, owner, "导入")

	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/outline/import",
		`{"chapters":[{"title":"第一章","brief":"开篇"},{"title":"第二章","brief":"承接"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	if got := intFromJSON(decodeJSON(t, resp)["inserted_count"]); got != 2 {
		t.Fatalf("inserted_count = %d, want 2", got)
	}

	chapters := listChapters(t, srv, owner, projectID)
	if len(chapters) != 2 {
		t.Fatalf("chapter count = %d, want 2", len(chapters))
	}
	first, _ := chapters[0].(map[string]any)
	id, _ := first["id"].(string)
	if !strings.HasPrefix(id, "chp_") {
		t.Fatalf("imported chapter id = %q, want the canonical chp_ prefix", id)
	}
	if got := intFromJSON(first["seq"]); got != 1 {
		t.Fatalf("first seq = %d, want 1", got)
	}
}

func TestOutlineImportReplacePreservesWrittenChapters(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-replace@example.com")
	projectID := createProject(t, srv, owner, "保护成稿")

	written := createChapter(t, srv, owner, projectID, "已完成章", "梗概")
	const prose = "这是已经写好的正文，绝不能被导入覆盖。"
	patch := patchJSON(t, owner, srv.URL+"/api/chapters/"+written, fmt.Sprintf(`{"body":%q}`, prose))
	patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("patch body: %d", patch.StatusCode)
	}
	createChapter(t, srv, owner, projectID, "草稿章", "还没写")

	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/outline/import",
		`{"replace_existing":true,"chapters":[{"title":"新一章","brief":"新梗概"},{"title":"新二章","brief":"新梗概"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import: %d", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if got := intFromJSON(body["protected_count"]); got != 1 {
		t.Fatalf("protected_count = %d, want 1", got)
	}

	// The finished chapter must survive with its prose intact.
	get := mustGet(t, owner, srv.URL+"/api/chapters/"+written)
	defer get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Fatalf("written chapter is gone: %d", get.StatusCode)
	}
	if got, _ := decodeJSON(t, get)["body"].(string); got != prose {
		t.Fatalf("body = %q, want the original prose", got)
	}

	// Draft replaced, finished chapter kept, new chapters appended after it.
	chapters := listChapters(t, srv, owner, projectID)
	if len(chapters) != 3 {
		t.Fatalf("chapter count = %d, want 3 (1 preserved + 2 imported)", len(chapters))
	}
	for _, raw := range chapters {
		ch, _ := raw.(map[string]any)
		if title, _ := ch["title"].(string); title == "草稿章" {
			t.Fatal("empty draft chapter should have been replaced")
		}
	}
}

func TestOutlineImportRejectsEmptyBrief(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-emptybrief@example.com")
	projectID := createProject(t, srv, owner, "空梗概")

	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/outline/import",
		`{"chapters":[{"title":"第一章","brief":"有梗概"},{"title":"第二章","brief":"   "}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	assertAPIError(t, decodeJSON(t, resp), "invalid")

	// Validation happens before the transaction, so nothing was written.
	if n := len(listChapters(t, srv, owner, projectID)); n != 0 {
		t.Fatalf("chapter count = %d, want 0 after a rejected import", n)
	}
}

func TestOutlineImportRejectsTooManyChapters(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-toomany@example.com")
	projectID := createProject(t, srv, owner, "超量")

	items := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		items = append(items, `{"title":"章","brief":"梗概"}`)
	}
	resp := postJSON(t, owner, srv.URL+"/api/projects/"+projectID+"/outline/import",
		`{"chapters":[`+strings.Join(items, ",")+`]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestOutlineImportRejectsForeignCard(t *testing.T) {
	srv, st := newStudioServer(t)
	owner := registerUser(t, srv, "studio-cardowner@example.com")
	ownerProject := createProject(t, srv, owner, "本项目")
	otherProject := createProject(t, srv, owner, "另一个项目")

	// A card the user owns, but that belongs to a different project.
	fix := card.ValidFixture("extracted")
	fix.ProjectID = otherProject
	insertStoreCard(t, st, fix)

	resp := postJSON(t, owner, srv.URL+"/api/projects/"+ownerProject+"/outline/import",
		fmt.Sprintf(`{"card_id":%q,"chapters":[{"title":"第一章","brief":"梗概"}]}`, fix.ID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a card from another project", resp.StatusCode)
	}
	if n := len(listChapters(t, srv, owner, ownerProject)); n != 0 {
		t.Fatalf("chapter count = %d, want 0", n)
	}
}

// --- Defect F: export must produce a parseable filename and never truncate ---

func TestExportNovelTxtAndMd(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-export@example.com")
	projectID := createProject(t, srv, owner, "导出作品")

	id := createChapter(t, srv, owner, projectID, "第一章", "梗概")
	const prose = "这是第一章的正文。"
	patch := patchJSON(t, owner, srv.URL+"/api/chapters/"+id, fmt.Sprintf(`{"body":%q}`, prose))
	patch.Body.Close()

	for _, format := range []string{"txt", "md"} {
		t.Run(format, func(t *testing.T) {
			resp := mustGet(t, owner, srv.URL+"/api/projects/"+projectID+"/export?format="+format)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("export %s: %d", format, resp.StatusCode)
			}
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !strings.Contains(string(raw), prose) {
				t.Fatalf("export %s did not contain the chapter body", format)
			}
			if _, _, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err != nil {
				t.Fatalf("Content-Disposition is unparseable: %v", err)
			}
		})
	}
}

func TestExportNovelFilenameSurvivesQuotesAndCJK(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-filename@example.com")
	// A name with both a quote and non-ASCII runes: interpolated raw, this
	// produces a header no client can parse.
	projectID := createProject(t, srv, owner, `《测"试"作品》`)
	createChapter(t, srv, owner, projectID, "第一章", "梗概")

	resp := mustGet(t, owner, srv.URL+"/api/projects/"+projectID+"/export?format=txt")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export: %d", resp.StatusCode)
	}

	cd := resp.Header.Get("Content-Disposition")
	disp, params, err := mime.ParseMediaType(cd)
	if err != nil {
		t.Fatalf("Content-Disposition %q is unparseable: %v", cd, err)
	}
	if disp != "attachment" {
		t.Fatalf("disposition = %q, want attachment", disp)
	}
	// mime.ParseMediaType decodes filename* into the filename parameter, so the
	// original name comes back intact.
	if got := params["filename"]; !strings.Contains(got, "测") {
		t.Fatalf("filename = %q, want the project name preserved via filename*", got)
	}
	if !strings.Contains(cd, "filename*=") {
		t.Fatalf("Content-Disposition %q lacks an RFC 5987 filename*", cd)
	}
}

// --- Ownership and prerequisites --------------------------------------------

func TestStudioEndpointsCrossUser404(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-owner@example.com")
	intruder := registerUser(t, srv, "studio-intruder@example.com")

	fake := newFakeLLM(t, validOutlineJSON(1))
	useFakeLLM(t, srv, intruder, fake)

	projectID := createProject(t, srv, owner, "私有作品")
	chapterID := createChapter(t, srv, owner, projectID, "第一章", "梗概")

	cases := []struct {
		name    string
		url     string
		payload string
	}{
		{"outline generate", "/api/projects/" + projectID + "/outline/generate", `{"premise":"别人的作品梗概","target_chapters":2}`},
		{"outline import", "/api/projects/" + projectID + "/outline/import", `{"chapters":[{"title":"章","brief":"梗概"}]}`},
		{"continuity audit", "/api/projects/" + projectID + "/continuity-audit", `{}`},
		{"branch simulate", "/api/chapters/" + chapterID + "/branch-simulate", `{"current_text":"正文"}`},
		{"chapter continue", "/api/chapters/" + chapterID + "/continue", `{"current_text":"正文","instruction":"继续"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, intruder, srv.URL+tc.url, tc.payload)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", resp.StatusCode)
			}
		})
	}

	export := mustGet(t, intruder, srv.URL+"/api/projects/"+projectID+"/export?format=txt")
	defer export.Body.Close()
	if export.StatusCode != http.StatusNotFound {
		t.Fatalf("export status = %d, want 404", export.StatusCode)
	}

	if fake.callCount() != 0 {
		t.Fatalf("intruder triggered %d LLM calls, want 0", fake.callCount())
	}
}

func TestStudioLLMEndpointsRequireKey(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-nokey@example.com")
	projectID := createProject(t, srv, owner, "没有密钥")
	chapterID := createChapter(t, srv, owner, projectID, "第一章", "梗概")

	cases := []struct {
		name    string
		url     string
		payload string
	}{
		{"outline generate", "/api/projects/" + projectID + "/outline/generate", `{"premise":"一个测试故事的梗概","target_chapters":2}`},
		{"continuity audit", "/api/projects/" + projectID + "/continuity-audit", `{}`},
		{"branch simulate", "/api/chapters/" + chapterID + "/branch-simulate", `{"current_text":"正文"}`},
		{"chapter continue", "/api/chapters/" + chapterID + "/continue", `{"current_text":"正文","instruction":"继续"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, owner, srv.URL+tc.url, tc.payload)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 without a stored LLM key", resp.StatusCode)
			}
			assertAPIError(t, decodeJSON(t, resp), "invalid")
		})
	}
}

// Guard against the suite silently taking a real network path if the BYOK base
// URL ever stops being honoured.
func TestFakeLLMIsActuallyUsed(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-wired@example.com")
	projectID := createProject(t, srv, owner, "连线检查")
	fake := newFakeLLM(t, validContinuityJSON())
	useFakeLLM(t, srv, owner, fake)
	createChapter(t, srv, owner, projectID, "第一章", "梗概")

	start := time.Now()
	runStudioJob(t, srv, owner, "/api/projects/"+projectID+"/continuity-audit", `{}`)
	if fake.callCount() == 0 {
		t.Fatal("handler did not call the fake LLM; BYOK base_url is not being honoured")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("request took %v, suspiciously slow for a local fake", elapsed)
	}
}

// Cancellation is the main thing the job conversion buys over the old blocking
// request: a 100-second audit can now be called off.
func TestStudioJobCanBeCanceled(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "studio-cancel@example.com")
	projectID := createProject(t, srv, owner, "可取消")
	createChapter(t, srv, owner, projectID, "第一章", "梗概")

	// A fake that blocks until released, so the job is reliably in flight.
	release := make(chan struct{})
	blocking := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": validContinuityJSON()}}},
		})
	}))
	defer blocking.Close()
	defer close(release)

	resp := putJSON(t, owner, srv.URL+"/api/me/llm-keys",
		fmt.Sprintf(`{"provider":"chat","base_url":%q,"api_key":"sk-test-abcd1234"}`, blocking.URL))
	resp.Body.Close()

	jobID := startStudioJob(t, srv, owner, "/api/projects/"+projectID+"/continuity-audit", `{}`)

	cancel := postJSON(t, owner, srv.URL+"/api/jobs/"+jobID+"/cancel", ``)
	status := cancel.StatusCode
	cancel.Body.Close()
	if status != http.StatusNoContent {
		t.Fatalf("cancel status = %d, want 204", status)
	}

	rec := awaitJob(t, srv, owner, jobID)
	if rec["status"] != "canceled" && rec["status"] != "failed" {
		t.Fatalf("status = %v, want canceled", rec["status"])
	}
}
