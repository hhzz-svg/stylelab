package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"stylelab/internal/extract"
	"stylelab/internal/job"
)

type extractRequest struct {
	AssetIDs []string `json:"asset_ids"`
	Name     string   `json:"name"`
	Model    string   `json:"model"`
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
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

	var in extractRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	payload, err := json.Marshal(extract.ExtractInput{
		ProjectID: projectID,
		AssetIDs:  in.AssetIDs,
		Name:      in.Name,
		Model:     in.Model,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: projectID,
		Kind:      job.KindExtract,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}
