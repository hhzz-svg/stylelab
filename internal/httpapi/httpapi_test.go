package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"stylelab/internal/card"
	"stylelab/internal/config"
	"stylelab/internal/httpapi"
	"stylelab/internal/job"
	"stylelab/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
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
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	runner := job.NewRunner(st, 2)
	runner.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"card_id":"crd_0123456789abcdef"}`), nil
		}
	})
	runner.Register(job.KindFuse, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"card_id":"crd_fedcba9876543210","conflicts":["句式冲突"]}`), nil
		}
	})
	runner.Register(job.KindAudit, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"audit_id":"aud_0123456789abcdef"}`), nil
		}
	})
	runner.Register(job.KindSample, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"sample_id":"smp_0123456789abcdef"}`), nil
		}
	})
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)
	return srv
}

func newJobTestEnv(t *testing.T) (*httptest.Server, *job.Runner) {
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
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	started := make(chan struct{})
	runner := job.NewRunner(st, 1)
	runner.Register(job.KindExtract, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)
	return srv, runner
}

func clientWithJar(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func decodeJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json %q: %v", body, err)
	}
	return out
}

func postJSON(t *testing.T, c *http.Client, rawURL, payload string) *http.Response {
	t.Helper()
	resp, err := c.Post(rawURL, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", rawURL, err)
	}
	return resp
}

func putJSON(t *testing.T, c *http.Client, rawURL, payload string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, rawURL, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("PUT %s: %v", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", rawURL, err)
	}
	return resp
}

func deleteReq(t *testing.T, c *http.Client, rawURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func decodeJSONRaw(t *testing.T, resp *http.Response) (string, map[string]any) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json %q: %v", body, err)
	}
	return string(body), out
}

func last4Runes(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 4 {
		return s
	}
	return string([]rune(s)[n-4:])
}

func TestAuthRegisterLoginMeLogout(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)

	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"reader@example.com","password":"password1"}`)
	got := decodeJSON(t, reg)
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register status: %d body=%v", reg.StatusCode, got)
	}
	if got["user_id"] == nil || got["user_id"] == "" {
		t.Fatalf("register missing user_id: %v", got)
	}
	if !hasSessionCookie(reg) {
		t.Fatal("register missing stylelab_session cookie")
	}

	login := postJSON(t, c, srv.URL+"/api/auth/login", `{"email":"reader@example.com","password":"password1"}`)
	got = decodeJSON(t, login)
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status: %d body=%v", login.StatusCode, got)
	}
	if got["user_id"] == nil || got["user_id"] == "" {
		t.Fatalf("login missing user_id: %v", got)
	}
	if !hasSessionCookie(login) {
		t.Fatal("login missing stylelab_session cookie")
	}

	me, err := c.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	got = decodeJSON(t, me)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("me status: %d body=%v", me.StatusCode, got)
	}
	if got["email"] != "reader@example.com" {
		t.Fatalf("me email: %v", got["email"])
	}
	if got["user_id"] == nil || got["user_id"] == "" {
		t.Fatalf("me missing user_id: %v", got)
	}

	logout, err := c.Post(srv.URL+"/api/auth/logout", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	logout.Body.Close()
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status: %d", logout.StatusCode)
	}

	me, err = c.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me after logout: %v", err)
	}
	got = decodeJSON(t, me)
	if me.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after logout status: %d body=%v", me.StatusCode, got)
	}
	assertAPIError(t, got, "unauthorized")
}

func TestMeWithoutCookieUnauthorized(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "unauthorized")
}

func TestRegisterValidationErrors(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)

	resp := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"bad","password":"password1"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad email status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"ok@example.com","password":"short"}`)
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"dup@example.com","password":"password1"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first register: %d", resp.StatusCode)
	}
	resp = postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"dup@example.com","password":"password1"}`)
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "conflict")
}

func TestLLMKeysPutGetDelete(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)

	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"keys@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	const apiKey = "sk-live-secret-密钥12"
	putBody := `{"provider":"openai","base_url":"https://api.openai.com/v1","api_key":"` + apiKey + `"}`
	put := putJSON(t, c, srv.URL+"/api/me/llm-keys", putBody)
	putRaw, putGot := decodeJSONRaw(t, put)
	if put.StatusCode != http.StatusOK {
		t.Fatalf("PUT status: %d body=%s", put.StatusCode, putRaw)
	}
	if strings.Contains(putRaw, apiKey) {
		t.Fatalf("PUT response leaked api_key: %s", putRaw)
	}
	if putGot["provider"] != "openai" {
		t.Fatalf("PUT provider: %v", putGot["provider"])
	}
	wantLast4 := last4Runes(apiKey)
	if putGot["last4"] != wantLast4 {
		t.Fatalf("PUT last4: got %v want %s", putGot["last4"], wantLast4)
	}

	get, err := c.Get(srv.URL + "/api/me/llm-keys")
	if err != nil {
		t.Fatalf("GET /api/me/llm-keys: %v", err)
	}
	getRaw, getGot := decodeJSONRaw(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET status: %d body=%s", get.StatusCode, getRaw)
	}
	if strings.Contains(getRaw, apiKey) {
		t.Fatalf("GET response leaked api_key: %s", getRaw)
	}
	if strings.Contains(getRaw, `"api_key"`) {
		t.Fatalf("GET response contains api_key field: %s", getRaw)
	}
	keys, ok := getGot["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("GET keys: %v", getGot["keys"])
	}
	row, _ := keys[0].(map[string]any)
	if row["provider"] != "openai" {
		t.Fatalf("GET provider: %v", row["provider"])
	}
	if row["base_url"] != "https://api.openai.com/v1" {
		t.Fatalf("GET base_url: %v", row["base_url"])
	}
	if row["last4"] != wantLast4 {
		t.Fatalf("GET last4: got %v want %s", row["last4"], wantLast4)
	}

	del, err := deleteReq(t, c, srv.URL+"/api/me/llm-keys/openai")
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status: %d", del.StatusCode)
	}

	get, err = c.Get(srv.URL + "/api/me/llm-keys")
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	getRaw, getGot = decodeJSONRaw(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET after delete status: %d body=%s", get.StatusCode, getRaw)
	}
	keys, ok = getGot["keys"].([]any)
	if !ok || len(keys) != 0 {
		t.Fatalf("expected empty keys after delete: %v", getGot["keys"])
	}
}

func TestLLMKeysValidation(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"badkey@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	resp := putJSON(t, c, srv.URL+"/api/me/llm-keys", `{"provider":"openai","api_key":""}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty key status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = putJSON(t, c, srv.URL+"/api/me/llm-keys", `{"provider":"gemini","api_key":"abcd1234"}`)
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad provider status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")
}

func TestLLMKeysRequireAuth(t *testing.T) {
	srv := newTestServer(t)
	resp := putJSON(t, http.DefaultClient, srv.URL+"/api/me/llm-keys", `{"provider":"openai","api_key":"abcd1234"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("PUT unauth status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "unauthorized")
}

func TestProjectsCRUD(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	list, err := c.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET empty list: %v", err)
	}
	got := decodeJSON(t, list)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("empty list status: %d body=%v", list.StatusCode, got)
	}
	projects, ok := got["projects"].([]any)
	if !ok || len(projects) != 0 {
		t.Fatalf("expected empty projects: %v", got["projects"])
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"风格实验"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create status: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "prj_") {
		t.Fatalf("id prefix: %v", created["id"])
	}
	if created["name"] != "风格实验" {
		t.Fatalf("create name: %v", created["name"])
	}
	if created["created_at"] == nil || created["created_at"] == "" {
		t.Fatalf("create missing created_at: %v", created)
	}

	list, err = c.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET list: %v", err)
	}
	got = decodeJSON(t, list)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status: %d body=%v", list.StatusCode, got)
	}
	projects, ok = got["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("list projects: %v", got["projects"])
	}
	row, _ := projects[0].(map[string]any)
	if row["id"] != id || row["name"] != "风格实验" {
		t.Fatalf("list row: %v", row)
	}

	get, err := c.Get(srv.URL + "/api/projects/" + id)
	if err != nil {
		t.Fatalf("GET one: %v", err)
	}
	got = decodeJSON(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("get status: %d body=%v", get.StatusCode, got)
	}
	if got["id"] != id || got["name"] != "风格实验" {
		t.Fatalf("get body: %v", got)
	}

	del, err := deleteReq(t, c, srv.URL+"/api/projects/"+id)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status: %d", del.StatusCode)
	}

	get, err = c.Get(srv.URL + "/api/projects/" + id)
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	got = decodeJSON(t, get)
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete status: %d body=%v", get.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")
}

func TestProjectsCrossUser404(t *testing.T) {
	srv := newTestServer(t)
	owner := clientWithJar(t)
	other := clientWithJar(t)

	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"a@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"b@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"owner only"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create status: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("missing id: %v", created)
	}

	list, err := other.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatalf("other list: %v", err)
	}
	got := decodeJSON(t, list)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("other list status: %d body=%v", list.StatusCode, got)
	}
	projects, ok := got["projects"].([]any)
	if !ok || len(projects) != 0 {
		t.Fatalf("other must not see owner projects: %v", got["projects"])
	}

	get, err := other.Get(srv.URL + "/api/projects/" + id)
	if err != nil {
		t.Fatalf("other get: %v", err)
	}
	got = decodeJSON(t, get)
	if get.StatusCode != http.StatusNotFound {
		t.Fatalf("other get status: %d body=%v (must not leak)", get.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")

	del, err := deleteReq(t, other, srv.URL+"/api/projects/"+id)
	if err != nil {
		t.Fatalf("other delete: %v", err)
	}
	got = decodeJSON(t, del)
	if del.StatusCode != http.StatusNotFound {
		t.Fatalf("other delete status: %d body=%v (must not leak)", del.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")

	get, err = owner.Get(srv.URL + "/api/projects/" + id)
	if err != nil {
		t.Fatalf("owner get after other delete: %v", err)
	}
	got = decodeJSON(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("owner project should still exist: %d body=%v", get.StatusCode, got)
	}
}

func TestProjectsNameValidation(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"names@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	resp := postJSON(t, c, srv.URL+"/api/projects", `{"name":""}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty name status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = postJSON(t, c, srv.URL+"/api/projects", `{"name":"`+strings.Repeat("字", 81)+`"}`)
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("81 runes status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = postJSON(t, c, srv.URL+"/api/projects", `{"name":"`+strings.Repeat("字", 80)+`"}`)
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("80 runes status: %d body=%v", resp.StatusCode, got)
	}
	if utf8.RuneCountInString(got["name"].(string)) != 80 {
		t.Fatalf("80-rune name not stored: %v", got["name"])
	}
}

func TestAssetUploadAndList(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"assets@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"素材库"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)
	if projectID == "" {
		t.Fatalf("missing project id: %v", created)
	}

	content := "第一章\n春风过境。\n\n第二章\n夏雨初歇。"
	upload := postMultipartFile(t, c, srv.URL+"/api/projects/"+projectID+"/assets", "file", "sample.txt", content)
	got := decodeJSON(t, upload)
	if upload.StatusCode != http.StatusCreated {
		t.Fatalf("upload status: %d body=%v", upload.StatusCode, got)
	}
	id, _ := got["id"].(string)
	if !strings.HasPrefix(id, "ast_") {
		t.Fatalf("id prefix: %v", got["id"])
	}
	if got["filename"] != "sample.txt" {
		t.Fatalf("filename: %v", got["filename"])
	}
	sum := sha256.Sum256([]byte(content))
	wantSHA := hex.EncodeToString(sum[:])
	if got["sha256"] != wantSHA {
		t.Fatalf("sha256: got %v want %s", got["sha256"], wantSHA)
	}
	if intFromJSON(got["rune_count"]) != utf8.RuneCountInString(content) {
		t.Fatalf("rune_count: %v", got["rune_count"])
	}
	if intFromJSON(got["chapter_count"]) != 2 {
		t.Fatalf("chapter_count: %v", got["chapter_count"])
	}

	list, err := c.Get(srv.URL + "/api/projects/" + projectID + "/assets")
	if err != nil {
		t.Fatalf("GET assets: %v", err)
	}
	listed := decodeJSON(t, list)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status: %d body=%v", list.StatusCode, listed)
	}
	raw, _ := json.Marshal(listed)
	if strings.Contains(string(raw), content) || strings.Contains(string(raw), "春风过境") {
		t.Fatalf("list leaked file body: %s", raw)
	}
	assets, ok := listed["assets"].([]any)
	if !ok || len(assets) != 1 {
		t.Fatalf("assets: %v", listed["assets"])
	}
	row, _ := assets[0].(map[string]any)
	if row["id"] != id || row["filename"] != "sample.txt" || row["sha256"] != wantSHA {
		t.Fatalf("list row: %v", row)
	}
	if intFromJSON(row["chapter_count"]) != 2 {
		t.Fatalf("list chapter_count: %v", row["chapter_count"])
	}
}

func TestAssetUploadOtherUserProject404(t *testing.T) {
	srv := newTestServer(t)
	owner := clientWithJar(t)
	other := clientWithJar(t)

	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"owner-ast@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"other-ast@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"owner only"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)

	upload := postMultipartFile(t, other, srv.URL+"/api/projects/"+id+"/assets", "file", "x.txt", "第一章\n甲\n\n第二章\n乙")
	got := decodeJSON(t, upload)
	if upload.StatusCode != http.StatusNotFound {
		t.Fatalf("other upload status: %d body=%v", upload.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")

	list, err := other.Get(srv.URL + "/api/projects/" + id + "/assets")
	if err != nil {
		t.Fatalf("other list: %v", err)
	}
	got = decodeJSON(t, list)
	if list.StatusCode != http.StatusNotFound {
		t.Fatalf("other list status: %d body=%v", list.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")
}

func TestAssetUploadRejectsBadFile(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"badfile@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}
	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"p"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)

	resp := postMultipartFile(t, c, srv.URL+"/api/projects/"+id+"/assets", "file", "note.pdf", "not allowed")
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("pdf status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")

	resp = postMultipartFile(t, c, srv.URL+"/api/projects/"+id+"/assets", "file", "empty.txt", "")
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "invalid")
}

func TestProjectsRequireAuth(t *testing.T) {
	srv := newTestServer(t)
	resp := postJSON(t, http.DefaultClient, srv.URL+"/api/projects", `{"name":"x"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST unauth status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "unauthorized")

	resp, err := http.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET list unauth: %v", err)
	}
	got = decodeJSON(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET list unauth status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "unauthorized")
}

func TestJobsGetAndCancel(t *testing.T) {
	srv, runner := newJobTestEnv(t)
	owner := clientWithJar(t)
	other := clientWithJar(t)

	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"job-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"job-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"jobs"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	me, err := owner.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	meBody := decodeJSON(t, me)
	userID, _ := meBody["user_id"].(string)

	jobID, err := runner.Enqueue(context.Background(), job.Record{
		UserID:    userID,
		ProjectID: projectID,
		Kind:      job.KindExtract,
		Payload:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var got map[string]any
	for {
		get, err := owner.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		got = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET job status: %d body=%v", get.StatusCode, got)
		}
		if got["status"] == "running" || got["status"] == "queued" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job never visible as queued/running: %v", got)
		}
		time.Sleep(20 * time.Millisecond)
	}

	otherGet, err := other.Get(srv.URL + "/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("other GET: %v", err)
	}
	otherBody := decodeJSON(t, otherGet)
	if otherGet.StatusCode != http.StatusNotFound {
		t.Fatalf("other GET status: %d body=%v", otherGet.StatusCode, otherBody)
	}
	assertAPIError(t, otherBody, "not_found")

	cancel, err := owner.Post(srv.URL+"/api/jobs/"+jobID+"/cancel", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cancel.Body.Close()
	if cancel.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel status: %d", cancel.StatusCode)
	}

	deadline = time.Now().Add(3 * time.Second)
	for {
		get, err := owner.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET after cancel: %v", err)
		}
		got = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET after cancel status: %d body=%v", get.StatusCode, got)
		}
		status, _ := got["status"].(string)
		if status == "canceled" || status == "failed" {
			if status == "running" {
				t.Fatalf("left running: %v", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job not canceled: %v", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestExtractStartsJob(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"extract@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"抽离"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	content := "第一章\n春风过境。\n\n第二章\n夏雨初歇。"
	upload := postMultipartFile(t, c, srv.URL+"/api/projects/"+projectID+"/assets", "file", "sample.txt", content)
	asset := decodeJSON(t, upload)
	if upload.StatusCode != http.StatusCreated {
		t.Fatalf("upload: %d body=%v", upload.StatusCode, asset)
	}
	assetID, _ := asset["id"].(string)

	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/extract", `{"asset_ids":["`+assetID+`"],"name":"节奏样本","model":"gpt-4o-mini"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("extract status: %d body=%v", resp.StatusCode, got)
	}
	jobID, _ := got["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job_id: %v", got["job_id"])
	}

	deadline := time.Now().Add(3 * time.Second)
	var jobBody map[string]any
	for {
		get, err := c.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		jobBody = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET job status: %d body=%v", get.StatusCode, jobBody)
		}
		status, _ := jobBody["status"].(string)
		if status == "succeeded" {
			break
		}
		if status == "failed" || status == "canceled" {
			t.Fatalf("job ended %s: %v", status, jobBody)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not succeed: %v", jobBody)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobBody["kind"] != "extract" {
		t.Fatalf("job kind: %v", jobBody["kind"])
	}
	if jobBody["project_id"] != projectID {
		t.Fatalf("job project_id: %v", jobBody["project_id"])
	}
	result, _ := jobBody["result"].(map[string]any)
	cardID, _ := result["card_id"].(string)
	if !strings.HasPrefix(cardID, "crd_") {
		t.Fatalf("result.card_id: %v", jobBody["result"])
	}
}

func TestFuseStartsJob(t *testing.T) {
	srv := newTestServer(t)
	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"fuse@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"融合"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/fuse", `{
		"name":"融合样本",
		"model":"gpt-4o-mini",
		"parents":[
			{"card_id":"crd_aaaaaaaaaaaaaaaa","version":1,"dims":["sentence_rhythm"],"weights":{"sentence_rhythm":60}},
			{"card_id":"crd_bbbbbbbbbbbbbbbb","version":1,"dims":["sentence_rhythm"],"weights":{"sentence_rhythm":40}}
		]
	}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("fuse status: %d body=%v", resp.StatusCode, got)
	}
	jobID, _ := got["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job_id: %v", got["job_id"])
	}

	deadline := time.Now().Add(3 * time.Second)
	var jobBody map[string]any
	for {
		get, err := c.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		jobBody = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET job status: %d body=%v", get.StatusCode, jobBody)
		}
		status, _ := jobBody["status"].(string)
		if status == "succeeded" {
			break
		}
		if status == "failed" || status == "canceled" {
			t.Fatalf("job ended %s: %v", status, jobBody)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not succeed: %v", jobBody)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobBody["kind"] != "fuse" {
		t.Fatalf("job kind: %v", jobBody["kind"])
	}
	result, _ := jobBody["result"].(map[string]any)
	cardID, _ := result["card_id"].(string)
	if !strings.HasPrefix(cardID, "crd_") {
		t.Fatalf("result.card_id: %v", jobBody["result"])
	}
}

func TestFuseOtherUserProject404(t *testing.T) {
	srv := newTestServer(t)
	owner := clientWithJar(t)
	other := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"fuse-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"fuse-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}
	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"owner"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)

	resp := postJSON(t, other, srv.URL+"/api/projects/"+id+"/fuse", `{"name":"x","parents":[]}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("other fuse status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")
}

func TestCardsListGetAndVersion(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	owner := clientWithJar(t)
	other := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"cards-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"cards-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"卡片"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_0123456789abcdef"
	fix.ProjectID = projectID
	now := time.Now().UTC().Format(time.RFC3339)
	dims, err := json.Marshal(fix.Dimensions)
	if err != nil {
		t.Fatalf("dims: %v", err)
	}
	prohibitions, err := json.Marshal(fix.Prohibitions)
	if err != nil {
		t.Fatalf("prohibitions: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 2, ?, ?)`,
		fix.ID, projectID, fix.Name, fix.Kind, now, now,
	)
	if err != nil {
		t.Fatalf("insert card: %v", err)
	}
	v1Dims := map[string]card.Dimension{}
	for k, d := range fix.Dimensions {
		if k == "sentence_rhythm" {
			d.Level = 11
			d.Summary = "第一版节奏"
		}
		v1Dims[k] = d
	}
	v1JSON, _ := json.Marshal(v1Dims)
	_, err = st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, 1, ?, ?, '{}', NULL, ?)`,
		fix.ID, string(v1JSON), string(prohibitions), now,
	)
	if err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, 2, ?, ?, '{}', NULL, ?)`,
		fix.ID, string(dims), string(prohibitions), now,
	)
	if err != nil {
		t.Fatalf("insert v2: %v", err)
	}

	list, err := owner.Get(srv.URL + "/api/projects/" + projectID + "/cards")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listed := decodeJSON(t, list)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status: %d body=%v", list.StatusCode, listed)
	}
	cards, _ := listed["cards"].([]any)
	if len(cards) != 1 {
		t.Fatalf("cards: %v", listed)
	}
	row, _ := cards[0].(map[string]any)
	if row["id"] != fix.ID || row["name"] != fix.Name || row["kind"] != "extracted" {
		t.Fatalf("list row: %v", row)
	}
	if intFromJSON(row["current_version"]) != 2 {
		t.Fatalf("current_version: %v", row["current_version"])
	}
	if row["updated_at"] == nil || row["updated_at"] == "" {
		t.Fatalf("updated_at missing: %v", row)
	}

	get, err := owner.Get(srv.URL + "/api/cards/" + fix.ID)
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	current := decodeJSON(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("get status: %d body=%v", get.StatusCode, current)
	}
	if current["id"] != fix.ID || intFromJSON(current["version"]) != 2 {
		t.Fatalf("current card: %v", current)
	}

	v1, err := owner.Get(srv.URL + "/api/cards/" + fix.ID + "/versions/1")
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	old := decodeJSON(t, v1)
	if v1.StatusCode != http.StatusOK {
		t.Fatalf("v1 status: %d body=%v", v1.StatusCode, old)
	}
	if intFromJSON(old["version"]) != 1 {
		t.Fatalf("v1 version: %v", old["version"])
	}
	dimsMap, _ := old["dimensions"].(map[string]any)
	sr, _ := dimsMap["sentence_rhythm"].(map[string]any)
	if intFromJSON(sr["level"]) != 11 {
		t.Fatalf("v1 level: %v", sr)
	}

	otherGet, err := other.Get(srv.URL + "/api/cards/" + fix.ID)
	if err != nil {
		t.Fatalf("other get: %v", err)
	}
	otherBody := decodeJSON(t, otherGet)
	if otherGet.StatusCode != http.StatusNotFound {
		t.Fatalf("other get status: %d body=%v", otherGet.StatusCode, otherBody)
	}
	assertAPIError(t, otherBody, "not_found")
}

func TestCardVersionPostKeepsPreviousVersion(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	owner := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"ver@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}
	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"版本"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_1111111111111111"
	fix.ProjectID = projectID
	now := time.Now().UTC().Format(time.RFC3339)
	dims, err := json.Marshal(fix.Dimensions)
	if err != nil {
		t.Fatalf("dims: %v", err)
	}
	prohibitions, err := json.Marshal(fix.Prohibitions)
	if err != nil {
		t.Fatalf("prohibitions: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 1, ?, ?)`,
		fix.ID, projectID, fix.Name, fix.Kind, now, now,
	)
	if err != nil {
		t.Fatalf("insert card: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
			 VALUES (?, 1, ?, ?, '{}', NULL, ?)`,
		fix.ID, string(dims), string(prohibitions), now,
	)
	if err != nil {
		t.Fatalf("insert v1: %v", err)
	}

	resp := postJSON(t, owner, srv.URL+"/api/cards/"+fix.ID+"/versions", `{
			"levels":{"sentence_rhythm":70},
			"rewrite_summaries":false,
			"model":""
		}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST versions status: %d body=%v", resp.StatusCode, got)
	}
	if intFromJSON(got["version"]) != 2 {
		t.Fatalf("new version: %v", got["version"])
	}
	if got["kind"] != "extracted" {
		t.Fatalf("kind: %v", got["kind"])
	}
	dimsMap, _ := got["dimensions"].(map[string]any)
	sr, _ := dimsMap["sentence_rhythm"].(map[string]any)
	if intFromJSON(sr["level"]) != 70 {
		t.Fatalf("v2 level: %v", sr)
	}

	v1, err := owner.Get(srv.URL + "/api/cards/" + fix.ID + "/versions/1")
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	old := decodeJSON(t, v1)
	if v1.StatusCode != http.StatusOK {
		t.Fatalf("v1 status: %d body=%v", v1.StatusCode, old)
	}
	if intFromJSON(old["version"]) != 1 {
		t.Fatalf("v1 version: %v", old["version"])
	}
	oldDims, _ := old["dimensions"].(map[string]any)
	oldSR, _ := oldDims["sentence_rhythm"].(map[string]any)
	if intFromJSON(oldSR["level"]) != 50 {
		t.Fatalf("v1 level changed: %v", oldSR)
	}

	exp, err := owner.Get(srv.URL + "/api/cards/" + fix.ID + "/export")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	defer exp.Body.Close()
	if exp.StatusCode != http.StatusOK {
		t.Fatalf("export status: %d", exp.StatusCode)
	}
	if ct := exp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("export content-type: %q", ct)
	}
	cd := exp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "simulation_profile.json") {
		t.Fatalf("content-disposition: %q", cd)
	}
	body, err := io.ReadAll(exp.Body)
	if err != nil {
		t.Fatalf("export body: %v", err)
	}
	var profile map[string]any
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if profile["version"] != "simulation_profile.v1" {
		t.Fatalf("export version: %v", profile["version"])
	}
}

func TestAuditStartsJob(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Register(job.KindAudit, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"audit_id":"aud_0123456789abcdef"}`), nil
		}
	})
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"audit@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"审计"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_aaaaaaaaaaaaaaaa"
	fix.ProjectID = projectID
	insertStoreCard(t, st, fix)

	resp := postJSON(t, c, srv.URL+"/api/cards/"+fix.ID+"/audit", `{"model":""}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("audit status: %d body=%v", resp.StatusCode, got)
	}
	jobID, _ := got["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job_id: %v", got["job_id"])
	}

	deadline := time.Now().Add(3 * time.Second)
	var jobBody map[string]any
	for {
		get, err := c.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		jobBody = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET job status: %d body=%v", get.StatusCode, jobBody)
		}
		status, _ := jobBody["status"].(string)
		if status == "succeeded" {
			break
		}
		if status == "failed" || status == "canceled" {
			t.Fatalf("job ended %s: %v", status, jobBody)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not succeed: %v", jobBody)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobBody["kind"] != "audit" {
		t.Fatalf("job kind: %v", jobBody["kind"])
	}
	if jobBody["project_id"] != projectID {
		t.Fatalf("job project_id: %v", jobBody["project_id"])
	}
	result, _ := jobBody["result"].(map[string]any)
	auditID, _ := result["audit_id"].(string)
	if !strings.HasPrefix(auditID, "aud_") {
		t.Fatalf("result.audit_id: %v", jobBody["result"])
	}
}

func TestGetAuditReportAndOtherUser404(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	owner := clientWithJar(t)
	other := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"audit-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"audit-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"审计卡"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_bbbbbbbbbbbbbbbb"
	fix.ProjectID = projectID
	insertStoreCard(t, st, fix)

	report := map[string]any{
		"id":           "aud_cccccccccccccccc",
		"card_id":      fix.ID,
		"card_version": 1,
		"personas":     []string{"commercial_web", "literary_texture"},
		"by_persona": map[string]any{
			"commercial_web":   map[string]any{"strengths": []string{"钩子清楚"}, "risks": []string{"章末偏弱"}, "suggestions": []string{"加强未决"}},
			"literary_texture": map[string]any{"strengths": []string{"肌理稳"}, "risks": []string{"修辞满"}, "suggestions": []string{"减密度"}},
		},
		"conflicts":         []map[string]string{{"dimension": "tension_hook", "summary": "拉力与留白张力不同"}},
		"recommended_edits": []map[string]any{{"dimension": "tension_hook", "target_level": 70, "reason": "加强章末未决"}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	_, err = st.DB().Exec(
		`INSERT INTO audit_reports (id, project_id, card_id, card_version, report_json, created_at)
			 VALUES (?, ?, ?, 1, ?, ?)`,
		"aud_cccccccccccccccc", projectID, fix.ID, string(raw), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert audit: %v", err)
	}

	get, err := owner.Get(srv.URL + "/api/audits/aud_cccccccccccccccc")
	if err != nil {
		t.Fatalf("GET audit: %v", err)
	}
	got := decodeJSON(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET audit status: %d body=%v", get.StatusCode, got)
	}
	if got["card_id"] != fix.ID {
		t.Fatalf("card_id: %v", got["card_id"])
	}
	byPersona, _ := got["by_persona"].(map[string]any)
	if _, ok := byPersona["commercial_web"]; !ok {
		t.Fatalf("missing commercial_web: %v", got)
	}
	if _, ok := byPersona["literary_texture"]; !ok {
		t.Fatalf("missing literary_texture: %v", got)
	}

	otherGet, err := other.Get(srv.URL + "/api/audits/aud_cccccccccccccccc")
	if err != nil {
		t.Fatalf("other GET: %v", err)
	}
	otherBody := decodeJSON(t, otherGet)
	if otherGet.StatusCode != http.StatusNotFound {
		t.Fatalf("other GET status: %d body=%v", otherGet.StatusCode, otherBody)
	}
	assertAPIError(t, otherBody, "not_found")

	otherPost := postJSON(t, other, srv.URL+"/api/cards/"+fix.ID+"/audit", `{"model":""}`)
	otherPostBody := decodeJSON(t, otherPost)
	if otherPost.StatusCode != http.StatusNotFound {
		t.Fatalf("other POST status: %d body=%v", otherPost.StatusCode, otherPostBody)
	}
	assertAPIError(t, otherPostBody, "not_found")
}

func TestSampleStartsJob(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Register(job.KindSample, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"sample_id":"smp_0123456789abcdef"}`), nil
		}
	})
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"sample@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}

	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"试写"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_dddddddddddddddd"
	fix.ProjectID = projectID
	insertStoreCard(t, st, fix)

	resp := postJSON(t, c, srv.URL+"/api/cards/"+fix.ID+"/sample", `{"premise":"雨夜有人敲门","target_runes":1200,"model":""}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("sample status: %d body=%v", resp.StatusCode, got)
	}
	jobID, _ := got["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job_id: %v", got["job_id"])
	}

	deadline := time.Now().Add(3 * time.Second)
	var jobBody map[string]any
	for {
		get, err := c.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		jobBody = decodeJSON(t, get)
		if get.StatusCode != http.StatusOK {
			t.Fatalf("GET job status: %d body=%v", get.StatusCode, jobBody)
		}
		status, _ := jobBody["status"].(string)
		if status == "succeeded" {
			break
		}
		if status == "failed" || status == "canceled" {
			t.Fatalf("job ended %s: %v", status, jobBody)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not succeed: %v", jobBody)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobBody["kind"] != "sample" {
		t.Fatalf("job kind: %v", jobBody["kind"])
	}
	if jobBody["project_id"] != projectID {
		t.Fatalf("job project_id: %v", jobBody["project_id"])
	}
	result, _ := jobBody["result"].(map[string]any)
	sampleID, _ := result["sample_id"].(string)
	if !strings.HasPrefix(sampleID, "smp_") {
		t.Fatalf("result.sample_id: %v", jobBody["result"])
	}
}

func TestGetSampleAndOtherUser404(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	owner := clientWithJar(t)
	other := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"sample-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"sample-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}

	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"试写卡"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)

	fix := card.ValidFixture("extracted")
	fix.ID = "crd_eeeeeeeeeeeeeeee"
	fix.ProjectID = projectID
	insertStoreCard(t, st, fix)

	_, err = st.DB().Exec(
		`INSERT INTO samples (id, project_id, card_id, card_version, premise, body, facts_json, created_at)
			 VALUES (?, ?, ?, 1, ?, ?, ?, ?)`,
		"smp_ffffffffffffffff", projectID, fix.ID, "雨夜有人敲门", "门开了一条缝。", `{"chapters":1,"sample_too_small":true}`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert sample: %v", err)
	}

	get, err := owner.Get(srv.URL + "/api/samples/smp_ffffffffffffffff")
	if err != nil {
		t.Fatalf("GET sample: %v", err)
	}
	got := decodeJSON(t, get)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET sample status: %d body=%v", get.StatusCode, got)
	}
	if got["premise"] != "雨夜有人敲门" {
		t.Fatalf("premise: %v", got["premise"])
	}
	if got["body"] != "门开了一条缝。" {
		t.Fatalf("body: %v", got["body"])
	}
	if got["card_id"] != fix.ID {
		t.Fatalf("card_id: %v", got["card_id"])
	}
	if intFromJSON(got["card_version"]) != 1 {
		t.Fatalf("card_version: %v", got["card_version"])
	}
	facts, _ := got["facts"].(map[string]any)
	if facts == nil {
		t.Fatalf("facts missing: %v", got)
	}

	otherGet, err := other.Get(srv.URL + "/api/samples/smp_ffffffffffffffff")
	if err != nil {
		t.Fatalf("other GET: %v", err)
	}
	otherBody := decodeJSON(t, otherGet)
	if otherGet.StatusCode != http.StatusNotFound {
		t.Fatalf("other GET status: %d body=%v", otherGet.StatusCode, otherBody)
	}
	assertAPIError(t, otherBody, "not_found")

	otherPost := postJSON(t, other, srv.URL+"/api/cards/"+fix.ID+"/sample", `{"premise":"雨夜有人敲门"}`)
	otherPostBody := decodeJSON(t, otherPost)
	if otherPost.StatusCode != http.StatusNotFound {
		t.Fatalf("other POST status: %d body=%v", otherPost.StatusCode, otherPostBody)
	}
	assertAPIError(t, otherPostBody, "not_found")
}

func TestSampleRejectsBadPremise(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"sample-bad@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}
	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"试写"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	projectID, _ := created["id"].(string)
	fix := card.ValidFixture("extracted")
	fix.ID = "crd_1111111111111111"
	fix.ProjectID = projectID
	insertStoreCard(t, st, fix)

	empty := postJSON(t, c, srv.URL+"/api/cards/"+fix.ID+"/sample", `{"premise":""}`)
	emptyBody := decodeJSON(t, empty)
	if empty.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty premise status: %d body=%v", empty.StatusCode, emptyBody)
	}
	assertAPIError(t, emptyBody, "invalid")

	long := postJSON(t, c, srv.URL+"/api/cards/"+fix.ID+"/sample", `{"premise":"`+strings.Repeat("前", 81)+`"}`)
	longBody := decodeJSON(t, long)
	if long.StatusCode != http.StatusBadRequest {
		t.Fatalf("long premise status: %d body=%v", long.StatusCode, longBody)
	}
	assertAPIError(t, longBody, "invalid")
}

func TestExtractOtherUserProject404(t *testing.T) {
	srv := newTestServer(t)
	owner := clientWithJar(t)
	other := clientWithJar(t)
	reg := postJSON(t, owner, srv.URL+"/api/auth/register", `{"email":"ex-owner@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("owner register: %d", reg.StatusCode)
	}
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"ex-other@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("other register: %d", reg.StatusCode)
	}
	create := postJSON(t, owner, srv.URL+"/api/projects", `{"name":"owner"}`)
	created := decodeJSON(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", create.StatusCode, created)
	}
	id, _ := created["id"].(string)

	resp := postJSON(t, other, srv.URL+"/api/projects/"+id+"/extract", `{"asset_ids":["ast_0123456789abcdef"],"name":"x"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("other extract status: %d body=%v", resp.StatusCode, got)
	}
	assertAPIError(t, got, "not_found")
}

func TestHealthStillWorks(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status: %d", resp.StatusCode)
	}
	if got["ok"] != true {
		t.Fatalf("health body: %v", got)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	srv := newTestServer(t)
	resp := postJSON(t, http.DefaultClient, srv.URL+"/api/auth/register", `{"email":"flags@example.com","password":"password1"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "stylelab_session" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("missing stylelab_session")
	}
	if cookie.Path != "/" {
		t.Fatalf("Path: %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite: %v", cookie.SameSite)
	}
	if cookie.MaxAge != 14*24*60*60 {
		t.Fatalf("MaxAge: %d", cookie.MaxAge)
	}
	if cookie.Secure {
		t.Fatal("Secure should be false without TLS")
	}
	if len(cookie.Value) != 64 {
		t.Fatalf("cookie value length: %d", len(cookie.Value))
	}
}

func TestSessionCookieSecureBehindHTTPSProxy(t *testing.T) {
	srv := newTestServer(t)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/register", strings.NewReader(`{"email":"proxy-flags@example.com","password":"password1"}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "stylelab_session" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("missing stylelab_session")
	}
	if !cookie.Secure {
		t.Fatal("Secure should be true behind an HTTPS proxy")
	}
}

func hasSessionCookie(resp *http.Response) bool {
	for _, c := range resp.Cookies() {
		if c.Name == "stylelab_session" && c.Value != "" {
			return true
		}
	}
	return false
}

func postMultipartFile(t *testing.T, c *http.Client, rawURL, field, filename, content string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	resp, err := c.Post(rawURL, w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", rawURL, err)
	}
	return resp
}

func intFromJSON(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return -1
	}
}

func insertStoreCard(t *testing.T, st *store.Store, c card.Card) {
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

func assertAPIError(t *testing.T, body map[string]any, code string) {
	t.Helper()
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %v", body)
	}
	if errObj["code"] != code {
		t.Fatalf("error.code: got %v want %s", errObj["code"], code)
	}
	msg, _ := errObj["message"].(string)
	if msg == "" {
		t.Fatalf("error.message empty: %v", body)
	}
}

func patchJSON(t *testing.T, c *http.Client, rawURL, payload string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, rawURL, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("PATCH %s: %v", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", rawURL, err)
	}
	return resp
}

func TestBibleCRUDAndOwnership(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"bible@example.com","password":"password1"}`)
	reg.Body.Close()
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", reg.StatusCode)
	}
	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"设定集项目"}`)
	created := decodeJSON(t, create)
	projectID, _ := created["id"].(string)

	// 空列表
	list := decodeJSON(t, mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/bible"))
	if _, ok := list["entries"].([]any); !ok {
		t.Fatalf("entries should be array: %v", list["entries"])
	}

	// 创建
	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/bible", `{"kind":"character","name":"沈砚","content":"冷面剑客","status":"active"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d body=%v", resp.StatusCode, got)
	}
	entryID, _ := got["id"].(string)
	if !strings.HasPrefix(entryID, "bib_") {
		t.Fatalf("entry id: %v", got["id"])
	}
	if got["origin"] != "manual" {
		t.Fatalf("origin should be manual: %v", got["origin"])
	}

	// GET 单条返回完整 content
	getResp := mustGet(t, c, srv.URL+"/api/bible/"+entryID)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get entry: %d", getResp.StatusCode)
	}
	gotEntry := decodeJSON(t, getResp)
	if gotEntry["content"] != "冷面剑客" {
		t.Fatalf("get content: %v", gotEntry["content"])
	}

	// 校验：坏 kind
	bad := decodeJSON(t, postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/bible", `{"kind":"weapon","name":"刀","content":""}`))
	assertAPIError(t, bad, "invalid")
	// 坏 name
	bad = decodeJSON(t, postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/bible", `{"kind":"character","name":"","content":""}`))
	assertAPIError(t, bad, "invalid")

	// 列表回显
	list = decodeJSON(t, mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/bible"))
	entries, _ := list["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}

	// PATCH 更新
	patched := decodeJSON(t, patchJSON(t, c, srv.URL+"/api/bible/"+entryID, `{"content":"冷面剑客，左臂旧伤","status":"active"}`))
	if patched["content"] != "冷面剑客，左臂旧伤" {
		t.Fatalf("patch content: %v", patched["content"])
	}
	if patched["origin"] != "manual" {
		t.Fatalf("patch should keep manual origin: %v", patched["origin"])
	}

	// DELETE
	del, err := deleteReq(t, c, srv.URL+"/api/bible/"+entryID)
	if err != nil {
		t.Fatal(err)
	}
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", del.StatusCode)
	}
	del.Body.Close()
	list = decodeJSON(t, mustGet(t, c, srv.URL+"/api/projects/"+projectID+"/bible"))
	entries, _ = list["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("want 0 entries after delete, got %d", len(entries))
	}

	// 跨用户 404
	other := clientWithJar(t)
	reg = postJSON(t, other, srv.URL+"/api/auth/register", `{"email":"bible-other@example.com","password":"password1"}`)
	reg.Body.Close()
	othersList := mustGet(t, other, srv.URL+"/api/projects/"+projectID+"/bible")
	if othersList.StatusCode != http.StatusNotFound {
		t.Fatalf("other user should get 404, got %d", othersList.StatusCode)
	}
	assertAPIError(t, decodeJSON(t, othersList), "not_found")
	othersList.Body.Close()
}

func TestBibleSyncStartsJob(t *testing.T) {
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
	runner := job.NewRunner(st, 1)
	runner.Register(job.KindBibleSync, func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		prog(50, "halfway")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Millisecond):
			return json.RawMessage(`{"chapters_synced":2,"entries":3}`), nil
		}
	})
	runner.Start(context.Background())
	srv := httptest.NewServer(httpapi.New(st, cfg, runner))
	t.Cleanup(srv.Close)

	c := clientWithJar(t)
	reg := postJSON(t, c, srv.URL+"/api/auth/register", `{"email":"biblesync@example.com","password":"password1"}`)
	reg.Body.Close()
	create := postJSON(t, c, srv.URL+"/api/projects", `{"name":"回填"}`)
	created := decodeJSON(t, create)
	projectID, _ := created["id"].(string)

	resp := postJSON(t, c, srv.URL+"/api/projects/"+projectID+"/bible/sync", `{"model":"gpt-4o-mini"}`)
	got := decodeJSON(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("sync status: %d body=%v", resp.StatusCode, got)
	}
	jobID, _ := got["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("job_id: %v", got["job_id"])
	}

	deadline := time.Now().Add(3 * time.Second)
	var jobBody map[string]any
	for {
		get, err := c.Get(srv.URL + "/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		jobBody = decodeJSON(t, get)
		status, _ := jobBody["status"].(string)
		if status == "succeeded" {
			break
		}
		if status == "failed" || status == "canceled" {
			t.Fatalf("job ended %s: %v", status, jobBody)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not succeed: %v", jobBody)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobBody["kind"] != "bible_sync" {
		t.Fatalf("job kind: %v", jobBody["kind"])
	}
	result, _ := jobBody["result"].(map[string]any)
	if result["chapters_synced"] != float64(2) {
		t.Fatalf("result: %v", jobBody["result"])
	}
}

func mustGet(t *testing.T, c *http.Client, url string) *http.Response {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}
