package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"stylelab/internal/card"
	"stylelab/internal/fuse"
	"stylelab/internal/job"
)

func (s *Server) handleListCards(w http.ResponseWriter, r *http.Request) {
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

	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, name, kind, current_version, updated_at
		 FROM style_cards WHERE project_id = ? ORDER BY updated_at DESC, id DESC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	cards := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, kind, updatedAt string
		var version int
		if err := rows.Scan(&id, &name, &kind, &version, &updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		cards = append(cards, map[string]any{
			"id":              id,
			"name":            name,
			"kind":            kind,
			"current_version": version,
			"updated_at":      updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

func (s *Server) handleGetCard(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	c, err := s.loadOwnedCard(r, userID, r.PathValue("id"), 0)
	if err != nil {
		writeCardError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleGetCardVersion(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "invalid", "invalid version")
		return
	}
	c, err := s.loadOwnedCard(r, userID, r.PathValue("id"), n)
	if err != nil {
		writeCardError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleFuse(w http.ResponseWriter, r *http.Request) {
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

	var in fuse.FuseSpec
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	payload, err := json.Marshal(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: projectID,
		Kind:      job.KindFuse,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) loadOwnedCard(r *http.Request, userID, cardID string, version int) (card.Card, error) {
	var (
		id, projectID, name, kind                    string
		ver                                          int
		dimsJSON, prohibJSON, factsJSON, lineageJSON sql.NullString
	)
	err := s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT c.id, c.project_id, c.name, c.kind, v.version,
		        v.dimensions_json, v.prohibitions_json, v.facts_json, v.lineage_json
		 FROM style_cards c
		 JOIN projects p ON p.id = c.project_id
		 JOIN style_card_versions v ON v.card_id = c.id
		  AND v.version = CASE WHEN ? = 0 THEN c.current_version ELSE ? END
		 WHERE c.id = ? AND p.user_id = ?`,
		version, version, cardID, userID,
	).Scan(&id, &projectID, &name, &kind, &ver, &dimsJSON, &prohibJSON, &factsJSON, &lineageJSON)
	if err != nil {
		return card.Card{}, err
	}
	c := card.Card{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		Version:   ver,
		Facts:     json.RawMessage(`{}`),
	}
	if err := json.Unmarshal([]byte(dimsJSON.String), &c.Dimensions); err != nil {
		return card.Card{}, err
	}
	if err := json.Unmarshal([]byte(prohibJSON.String), &c.Prohibitions); err != nil {
		return card.Card{}, err
	}
	if factsJSON.Valid && factsJSON.String != "" {
		c.Facts = json.RawMessage(factsJSON.String)
	}
	if lineageJSON.Valid && lineageJSON.String != "" && lineageJSON.String != "null" {
		var lin card.Lineage
		if err := json.Unmarshal([]byte(lineageJSON.String), &lin); err != nil {
			return card.Card{}, err
		}
		c.Lineage = &lin
	}
	return c, nil
}

func writeCardError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "invalid", "internal error")
}
