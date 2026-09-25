package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"stylelab/internal/lore"
)

// handleGraphAnalysis ranks the project's graph nodes. The analyses are
// deterministic and take milliseconds on a graph of a few hundred nodes, so
// unlike the model-backed features this answers synchronously.
func (s *Server) handleGraphAnalysis(w http.ResponseWriter, r *http.Request) {
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
	out, err := lore.Analyze(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
