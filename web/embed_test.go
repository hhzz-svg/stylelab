package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stylelab/web"
)

func TestHandlerServesIndexAndFallsBack(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(web.Handler(api))
	t.Cleanup(srv.Close)

	index := getBody(t, srv.URL+"/")
	if !strings.Contains(index, `id="root"`) {
		t.Fatalf("GET / missing root mount: %s", index)
	}

	fallback := getBody(t, srv.URL+"/p/prj_demo/lab/crd_demo")
	if !strings.Contains(fallback, `id="root"`) {
		t.Fatalf("SPA fallback missing root mount: %s", fallback)
	}

	health := getRaw(t, srv.URL+"/api/health")
	if health.status != http.StatusOK {
		t.Fatalf("GET /api/health status=%d", health.status)
	}
	if !strings.Contains(health.body, `"ok":true`) {
		t.Fatalf("GET /api/health body=%s", health.body)
	}

	missingAPI := getRaw(t, srv.URL+"/api/missing")
	if missingAPI.status != http.StatusNotFound {
		t.Fatalf("GET /api/missing status=%d", missingAPI.status)
	}
	if strings.Contains(missingAPI.body, `id="root"`) {
		t.Fatal("GET /api/missing must not fall back to SPA")
	}
}

func TestHandlerServesStaticAssetsWhenPresent(t *testing.T) {
	entries, err := web.Dist.ReadDir("dist/assets")
	if err != nil {
		t.Skip("no embedded assets; stub index.html only")
	}
	var jsName string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			jsName = e.Name()
			break
		}
	}
	if jsName == "" {
		t.Skip("no embedded js assets")
	}

	srv := httptest.NewServer(web.Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	got := getRaw(t, srv.URL+"/assets/"+jsName)
	if got.status != http.StatusOK {
		t.Fatalf("GET /assets/%s status=%d body=%s", jsName, got.status, got.body)
	}
	ct := got.contentType
	if !strings.Contains(ct, "javascript") && !strings.Contains(ct, "ecmascript") && !strings.Contains(ct, "text/plain") {
		// FileServer may omit a precise JS type; body must still be the asset, not the SPA shell.
		if strings.Contains(got.body, `id="root"`) {
			t.Fatalf("asset served as SPA fallback, content-type=%s", ct)
		}
	}
}

type rawResp struct {
	status      int
	body        string
	contentType string
}

func getRaw(t *testing.T, rawURL string) rawResp {
	t.Helper()
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", rawURL, err)
	}
	return rawResp{status: resp.StatusCode, body: string(b), contentType: resp.Header.Get("Content-Type")}
}

func getBody(t *testing.T, rawURL string) string {
	t.Helper()
	got := getRaw(t, rawURL)
	if got.status != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", rawURL, got.status, got.body)
	}
	return got.body
}
