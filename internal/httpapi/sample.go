package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"stylelab/internal/job"
	"stylelab/internal/sample"
)

type sampleRequest struct {
	Premise string `json:"premise"`
	Target  int    `json:"target_runes"`
	Model   string `json:"model"`
}

func (s *Server) handleSampleCard(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	cardID := r.PathValue("id")
	c, err := s.loadOwnedCard(r, userID, cardID, 0)
	if err != nil {
		writeCardError(w, err)
		return
	}

	var in sampleRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	premise := strings.TrimSpace(in.Premise)
	if premise == "" {
		writeError(w, http.StatusBadRequest, "invalid", "premise required")
		return
	}
	if utf8.RuneCountInString(premise) > 80 {
		writeError(w, http.StatusBadRequest, "invalid", "premise too long")
		return
	}
	target := in.Target
	if target == 0 {
		target = 1200
	}
	if target < 800 || target > 2000 {
		writeError(w, http.StatusBadRequest, "invalid", "target_runes must be between 800 and 2000")
		return
	}

	payload, err := json.Marshal(sample.Input{
		CardID:  c.ID,
		Premise: premise,
		Target:  target,
		Model:   in.Model,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: c.ProjectID,
		Kind:      job.KindSample,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) handleGetSample(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var premise, body, factsJSON, cardID string
	var cardVersion int
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT s.premise, s.body, s.facts_json, s.card_id, s.card_version
		 FROM samples s
		 JOIN projects p ON p.id = s.project_id
		 WHERE s.id = ? AND p.user_id = ?`,
		r.PathValue("id"), userID,
	).Scan(&premise, &body, &factsJSON, &cardID, &cardVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	facts := json.RawMessage(factsJSON)
	if len(facts) == 0 {
		facts = json.RawMessage(`{}`)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"premise":      premise,
		"body":         body,
		"facts":        facts,
		"card_id":      cardID,
		"card_version": cardVersion,
	})
}
