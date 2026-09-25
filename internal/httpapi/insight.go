package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"stylelab/internal/insight"
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
	weights, err := lore.ValidateWeights(r.URL.Query().Get("weights"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", trimInvalid(err))
		return
	}
	out, err := lore.Analyze(r.Context(), s.st, projectID, weights)
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

// handleGraphCooccurrence reports who appears where in the written
// chapters, who shares paragraphs with whom, and who has been gone a while.
func (s *Server) handleGraphCooccurrence(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	absentAfter := insight.DefaultAbsentAfter
	if raw := r.URL.Query().Get("absent_after"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 1000 {
			writeError(w, http.StatusBadRequest, "invalid", "absent_after must be a whole number of chapters, 1-1000")
			return
		}
		absentAfter = n
	}
	out, err := lore.Cooccurrence(r.Context(), s.st, projectID, absentAfter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGraphPlaces returns the geography with the scenes set at each place.
func (s *Server) handleGraphPlaces(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	out, err := lore.LoadPlaces(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAliasSuggestions proposes aliases found in the prose; the author
// adopts them by saving the node's details.aliases.
func (s *Server) handleAliasSuggestions(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	out, err := lore.AliasSuggestions(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}
