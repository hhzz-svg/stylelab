package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var Dist embed.FS

// Handler serves the embedded Vite SPA for non-/api GET requests.
// Existing files under dist are returned as-is; everything else falls back to index.html.
func Handler(next http.Handler) http.Handler {
	root, err := fs.Sub(Dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if serveStatic(root, r.URL.Path) {
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, root)
	})
}

func serveStatic(root fs.FS, urlPath string) bool {
	path := strings.TrimPrefix(urlPath, "/")
	if path == "" || path == "." {
		return false
	}
	if !fs.ValidPath(path) {
		return false
	}
	f, err := root.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		return false
	}
	return true
}

func serveIndex(w http.ResponseWriter, r *http.Request, root fs.FS) {
	f, err := root.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", st.ModTime(), rs)
}
