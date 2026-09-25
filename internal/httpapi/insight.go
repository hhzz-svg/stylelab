package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"stylelab/internal/lore"
)

// ownedProjectForInsight resolves the caller and checks the project, writing
// the error response itself when either fails.
func (s *Server) ownedProjectForInsight(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return "", false
	}
	projectID := r.PathValue("id")
	if err := s.requireOwnedProject(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return "", false
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return "", false
	}
	return projectID, true
}

// handleGraphAnalysis ranks the project's graph nodes. The analyses are
// deterministic and take milliseconds on a graph of a few hundred nodes, so
// unlike the model-backed features this answers synchronously.
func (s *Server) handleGraphAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	out, err := lore.Analyze(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGraphLineage returns the laid-out lineage forest.
func (s *Server) handleGraphLineage(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	out, err := lore.Lineage(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
