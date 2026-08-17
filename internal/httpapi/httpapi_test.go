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
			return json.RawMessage(`{"ok":true}`), nil
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
