package httpapi

import (
	"encoding/json"
	"net/http"

	"stylelab/internal/auth"
	"stylelab/internal/config"
	"stylelab/internal/store"
)

type Server struct {
	st   *store.Store
	cfg  config.Config
	auth *auth.Service
	mux  *http.ServeMux
}

func New(st *store.Store, cfg config.Config) http.Handler {
	s := &Server{
		st:   st,
		cfg:  cfg,
		auth: auth.NewService(st),
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/me", s.handleMe)
	s.mux.HandleFunc("PUT /api/me/llm-keys", s.handlePutLLMKey)
	s.mux.HandleFunc("GET /api/me/llm-keys", s.handleListLLMKeys)
	s.mux.HandleFunc("DELETE /api/me/llm-keys/{provider}", s.handleDeleteLLMKey)
	s.mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
