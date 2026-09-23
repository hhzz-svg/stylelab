package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"stylelab/internal/job"
	"stylelab/internal/studio"
)

// The studio results are the jobs' own results; these routes return the most
// recent successful one so the UI can show it again without paying for another
// model call. No result yet is {"latest": null}, not a 404: 404 on these routes
// means the project or chapter is missing or not yours.

func (s *Server) handleProjectStudioLatest(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	kind := job.Kind(r.URL.Query().Get("kind"))
	if !studio.ProjectScoped(kind) {
		writeError(w, http.StatusBadRequest, "invalid", "kind must be outline_generate or continuity_audit")
		return
	}
	s.writeLatest(w, r, userID, projectID, kind, "")
}

func (s *Server) handleChapterStudioLatest(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	chapterID := r.PathValue("id")
	projectID, err := s.chapterProject(r, chapterID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "chapter not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if kind := job.Kind(r.URL.Query().Get("kind")); kind != job.KindBranch {
		writeError(w, http.StatusBadRequest, "invalid", "kind must be branch_simulate")
		return
	}
	s.writeLatest(w, r, userID, projectID, job.KindBranch, chapterID)
}

func (s *Server) writeLatest(w http.ResponseWriter, r *http.Request, userID, projectID string, kind job.Kind, chapterID string) {
	latest, err := studio.LatestResult(r.Context(), s.st, userID, projectID, kind, chapterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"latest": latest})
}
