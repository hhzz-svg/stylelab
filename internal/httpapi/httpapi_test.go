package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"stylelab/internal/config"
	"stylelab/internal/httpapi"
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
	srv := httptest.NewServer(httpapi.New(st, cfg))
	t.Cleanup(srv.Close)
	return srv
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
