package httpapi_test

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stylelab/internal/store"
)

func createManualCard(t *testing.T, srv *httptest.Server, c *http.Client, projectID string) string {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/cards",
		`{"name":"冷峻","kind":"manual","dimensions":{"sentence_rhythm":{"level":60,"summary":"短句","techniques":["断句"]}}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create card: %d", resp.StatusCode)
	}
	id, _ := decodeJSON(t, resp)["id"].(string)
	if id == "" {
		t.Fatal("create card: no id")
	}
	return id
}

func createChapterWithCard(t *testing.T, srv *httptest.Server, c *http.Client, projectID, cardID, title, brief string) string {
	t.Helper()
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/chapters",
		fmt.Sprintf(`{"title":%q,"brief":%q,"card_id":%q}`, title, brief, cardID))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create chapter: %d", resp.StatusCode)
	}
	id, _ := decodeJSON(t, resp)["id"].(string)
	return id
}

func postStatus(t *testing.T, c *http.Client, rawURL, payload string) (int, string) {
	t.Helper()
	resp := postJSON(t, c, rawURL, payload)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestWriteChapterJobAndManuscript(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "write-chapter@example.com")
	projectID := createProject(t, srv, owner, "写章")
	cardID := createManualCard(t, srv, owner, projectID)
	chapterID := createChapterWithCard(t, srv, owner, projectID, cardID, "雪夜", "主角在雪夜里赶路")

	const body = "雪下了一整夜。他没有停下。"
	fake := newFakeLLM(t, body)
	useFakeLLM(t, srv, owner, fake)

	jobID := startStudioJob(t, srv, owner, "/api/chapters/"+chapterID+"/write", `{"note":"多写风声"}`)
	rec := awaitJob(t, srv, owner, jobID)
	if rec["kind"] != "write" || rec["status"] != "succeeded" {
		t.Fatalf("job = kind %v status %v error %v", rec["kind"], rec["status"], rec["error"])
	}
	if !strings.Contains(fake.lastUserPromptContaining(t, "本章序号"), "多写风声") {
		t.Fatal("the note did not reach the chapter prompt")
	}

	resp := mustGet(t, owner, srv.URL+"/api/chapters/"+chapterID)
	ch := decodeJSON(t, resp)
	resp.Body.Close()
	if ch["status"] != "written" || ch["body"] != body {
		t.Fatalf("chapter after write = status %v body %q", ch["status"], ch["body"])
	}

	// The manuscript export carries the written chapter.
	resp = mustGet(t, owner, srv.URL+"/api/projects/"+projectID+"/manuscript.md")
	md, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manuscript: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Fatalf("manuscript content-type = %q", ct)
	}
	for _, want := range []string{"# 写章", "## 第 1 章 雪夜", body} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("manuscript lacks %q:\n%s", want, md)
		}
	}
}

func TestWriteChapterRejections(t *testing.T) {
	srv, st := newStudioServer(t)
	owner := registerUser(t, srv, "write-reject@example.com")
	intruder := registerUser(t, srv, "write-intruder@example.com")
	projectID := createProject(t, srv, owner, "写章校验")
	otherProject := createProject(t, srv, owner, "另一本书")
	cardID := createManualCard(t, srv, owner, projectID)
	foreignCard := createManualCard(t, srv, owner, otherProject)

	noCard := createChapter(t, srv, owner, projectID, "无卡", "有梗概")
	if code, raw := postStatus(t, owner, srv.URL+"/api/chapters/"+noCard+"/write", `{}`); code != http.StatusBadRequest {
		t.Fatalf("no card: %d %s", code, raw)
	}

	// A card from another of the owner's projects cannot be attached.
	resp := patchJSON(t, owner, srv.URL+"/api/chapters/"+noCard, fmt.Sprintf(`{"card_id":%q}`, foreignCard))
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("attach foreign card: %d, want 404", resp.StatusCode)
	}

	chapterID := createChapterWithCard(t, srv, owner, projectID, cardID, "第二章", "梗概")
	setBrief(t, st, chapterID, "   ")
	if code, raw := postStatus(t, owner, srv.URL+"/api/chapters/"+chapterID+"/write", `{}`); code != http.StatusBadRequest {
		t.Fatalf("empty brief: %d %s", code, raw)
	}
	setBrief(t, st, chapterID, "梗概")

	if code, _ := postStatus(t, owner, srv.URL+"/api/chapters/"+chapterID+"/write",
		fmt.Sprintf(`{"note":%q}`, strings.Repeat("长", 5000))); code != http.StatusBadRequest {
		t.Fatalf("oversized note: %d, want 400", code)
	}

	if code, _ := postStatus(t, intruder, srv.URL+"/api/chapters/"+chapterID+"/write", `{}`); code != http.StatusNotFound {
		t.Fatalf("other user write: %d, want 404", code)
	}
	r := mustGet(t, intruder, srv.URL+"/api/projects/"+projectID+"/manuscript.md")
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("other user manuscript: %d, want 404", r.StatusCode)
	}

	// None of the rejected calls may leave the chapter marked as writing.
	resp = mustGet(t, owner, srv.URL+"/api/chapters/"+chapterID)
	ch := decodeJSON(t, resp)
	resp.Body.Close()
	if ch["status"] != "draft" {
		t.Fatalf("status after rejected writes = %v, want draft", ch["status"])
	}
}

func setBrief(t *testing.T, st *store.Store, chapterID, brief string) {
	t.Helper()
	if _, err := st.DB().Exec(`UPDATE chapters SET brief = ? WHERE id = ?`, brief, chapterID); err != nil {
		t.Fatalf("set brief: %v", err)
	}
}

// The events stream reports progress while a job runs and ends on its
// terminal state.
func TestJobEventsStreamUntilTerminal(t *testing.T) {
	srv, _ := newStudioServer(t)
	owner := registerUser(t, srv, "job-events@example.com")
	intruder := registerUser(t, srv, "job-events-intruder@example.com")
	projectID := createProject(t, srv, owner, "事件流")
	createChapter(t, srv, owner, projectID, "第一章", "开篇")

	fake := newFakeLLM(t, graphReply([][3]string{{"甲", "", ""}}, nil))
	release := make(chan struct{})
	fake.mu.Lock()
	fake.hold = release
	fake.mu.Unlock()
	released := false
	t.Cleanup(func() {
		if !released {
			close(release)
		}
	})
	useFakeLLM(t, srv, owner, fake)

	jobID := startStudioJob(t, srv, owner, "/api/projects/"+projectID+"/graph/extract", `{}`)

	r := mustGet(t, intruder, srv.URL+"/api/jobs/"+jobID+"/events")
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("other user events: %d, want 404", r.StatusCode)
	}

	resp := mustGet(t, owner, srv.URL+"/api/jobs/"+jobID+"/events")
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}

	events := make(chan string)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				events <- data
			}
		}
	}()

	next := func() string {
		t.Helper()
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("stream closed before a terminal event")
			}
			return ev
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for an event")
		}
		return ""
	}

	// The job is parked in the model call, so the stream is open and not done.
	for ev := next(); !strings.Contains(ev, `"status":"running"`); ev = next() {
	}
	close(release)
	released = true
	var last string
	for ev := range events {
		last = ev
	}
	if !strings.Contains(last, `"status":"succeeded"`) || !strings.Contains(last, `"progress":100`) {
		t.Fatalf("last event = %s, want succeeded at 100", last)
	}

	// A finished job answers with its one terminal event and closes.
	resp2 := mustGet(t, owner, srv.URL+"/api/jobs/"+jobID+"/events")
	raw, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if strings.Count(string(raw), "event: progress") != 1 || !strings.Contains(string(raw), `"status":"succeeded"`) {
		t.Fatalf("finished job stream = %q", raw)
	}
}
