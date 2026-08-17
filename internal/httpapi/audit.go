package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"stylelab/internal/job"
)

type auditRequest struct {
	Model string `json:"model"`
}

func (s *Server) handleAuditCard(w http.ResponseWriter, r *http.Request) {
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

	var in auditRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	payload, err := json.Marshal(map[string]string{
		"card_id": c.ID,
		"model":   in.Model,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: c.ProjectID,
		Kind:      job.KindAudit,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) handleGetAudit(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var reportJSON string
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT a.report_json
		 FROM audit_reports a
		 JOIN projects p ON p.id = a.project_id
		 WHERE a.id = ? AND p.user_id = ?`,
		r.PathValue("id"), userID,
	).Scan(&reportJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(reportJSON))
	if len(reportJSON) == 0 || reportJSON[len(reportJSON)-1] != '\n' {
		_, _ = w.Write([]byte("\n"))
	}
}
