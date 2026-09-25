package httpapi

import (
	"encoding/json"
	"net/http"

	"stylelab/internal/auth"
	"stylelab/internal/config"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

type Server struct {
	st   *store.Store
	cfg  config.Config
	auth *auth.Service
	jobs *job.Runner
	llm  *llm.Client
	mux  *http.ServeMux
}

func New(st *store.Store, cfg config.Config, runner *job.Runner) http.Handler {
	s := &Server{
		st:   st,
		cfg:  cfg,
		auth: auth.NewService(st),
		jobs: runner,
		llm:  &llm.Client{},
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
	s.mux.HandleFunc("POST /api/projects/{id}/assets", s.handleUploadAsset)
	s.mux.HandleFunc("GET /api/projects/{id}/assets", s.handleListAssets)
	s.mux.HandleFunc("POST /api/projects/{id}/extract", s.handleExtract)
	s.mux.HandleFunc("POST /api/projects/{id}/fuse", s.handleFuse)
	s.mux.HandleFunc("GET /api/projects/{id}/cards", s.handleListCards)
	s.mux.HandleFunc("POST /api/projects/{id}/cards", s.handleCreateCard)
	s.mux.HandleFunc("GET /api/cards/{id}", s.handleGetCard)
	s.mux.HandleFunc("POST /api/cards/{id}/versions", s.handleCreateCardVersion)
	s.mux.HandleFunc("GET /api/cards/{id}/versions/{n}", s.handleGetCardVersion)
	s.mux.HandleFunc("GET /api/cards/{id}/export", s.handleExportCard)
	s.mux.HandleFunc("POST /api/cards/{id}/audit", s.handleAuditCard)
	s.mux.HandleFunc("GET /api/audits/{id}", s.handleGetAudit)
	s.mux.HandleFunc("POST /api/cards/{id}/sample", s.handleSampleCard)
	s.mux.HandleFunc("GET /api/samples/{id}", s.handleGetSample)
	s.mux.HandleFunc("GET /api/projects/{id}/chapters", s.handleListChapters)
	s.mux.HandleFunc("POST /api/projects/{id}/chapters", s.handleCreateChapter)
	s.mux.HandleFunc("GET /api/projects/{id}/manuscript.md", s.handleManuscript)
	s.mux.HandleFunc("GET /api/projects/{id}/bible", s.handleListBible)
	s.mux.HandleFunc("POST /api/projects/{id}/bible", s.handleCreateBible)
	s.mux.HandleFunc("POST /api/projects/{id}/bible/sync", s.handleSyncBible)
	s.mux.HandleFunc("GET /api/bible/{id}", s.handleGetBible)
	s.mux.HandleFunc("GET /api/chapters/{id}", s.handleGetChapter)
	s.mux.HandleFunc("PATCH /api/chapters/{id}", s.handlePatchChapter)
	s.mux.HandleFunc("DELETE /api/chapters/{id}", s.handleDeleteChapter)
	s.mux.HandleFunc("POST /api/chapters/{id}/write", s.handleWriteChapter)
	s.mux.HandleFunc("PATCH /api/bible/{id}", s.handlePatchBible)
	s.mux.HandleFunc("DELETE /api/bible/{id}", s.handleDeleteBible)
	s.mux.HandleFunc("GET /api/projects/{id}/graph", s.handleGetGraph)
	s.mux.HandleFunc("POST /api/projects/{id}/graph/nodes", s.handleSaveGraphNode)
	s.mux.HandleFunc("DELETE /api/projects/{id}/graph/nodes/{nodeId}", s.handleDeleteGraphNode)
	s.mux.HandleFunc("POST /api/projects/{id}/graph/edges", s.handleSaveGraphEdge)
	s.mux.HandleFunc("DELETE /api/projects/{id}/graph/edges/{edgeId}", s.handleDeleteGraphEdge)
	s.mux.HandleFunc("POST /api/projects/{id}/graph/extract", s.handleExtractGraph)
	s.mux.HandleFunc("GET /api/projects/{id}/graph/analysis", s.handleGraphAnalysis)
	s.mux.HandleFunc("GET /api/projects/{id}/graph/lineage", s.handleGraphLineage)
	s.mux.HandleFunc("GET /api/projects/{id}/graph/places", s.handleGraphPlaces)
	s.mux.HandleFunc("GET /api/projects/{id}/graph/cooccurrence", s.handleGraphCooccurrence)
	s.mux.HandleFunc("GET /api/projects/{id}/graph/alias-suggestions", s.handleAliasSuggestions)
	s.mux.HandleFunc("GET /api/chapters/{id}/scenes", s.handleChapterScenes)
	s.mux.HandleFunc("POST /api/chapters/{id}/scenes/split", s.handleSplitChapterScenes)
	s.mux.HandleFunc("PATCH /api/scenes/{id}", s.handleUpdateScene)
	s.mux.HandleFunc("POST /api/projects/{id}/scenes/split-all", s.handleSplitAllScenes)
	s.mux.HandleFunc("GET /api/projects/{id}/structure", s.handleStructure)
	s.mux.HandleFunc("POST /api/projects/{id}/volumes", s.handleCreateVolume)
	s.mux.HandleFunc("PATCH /api/volumes/{id}", s.handleUpdateVolume)
	s.mux.HandleFunc("DELETE /api/volumes/{id}", s.handleDeleteVolume)
	s.mux.HandleFunc("POST /api/projects/{id}/outline/generate", s.handleGenerateOutline)
	s.mux.HandleFunc("POST /api/projects/{id}/outline/import", s.handleImportOutline)
	s.mux.HandleFunc("POST /api/chapters/{id}/branch-simulate", s.handleBranchSimulate)
	s.mux.HandleFunc("POST /api/chapters/{id}/continue", s.handleChapterContinue)
	s.mux.HandleFunc("POST /api/projects/{id}/continuity-audit", s.handleContinuityAudit)
	s.mux.HandleFunc("GET /api/projects/{id}/export", s.handleExportNovelMulti)
	s.mux.HandleFunc("GET /api/projects/{id}/studio/latest", s.handleProjectStudioLatest)
	s.mux.HandleFunc("GET /api/chapters/{id}/studio/latest", s.handleChapterStudioLatest)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleCancelJob)
	s.mux.HandleFunc("GET /api/jobs/{id}/events", s.handleJobEvents)
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
