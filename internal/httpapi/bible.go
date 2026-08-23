package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/bible"
	"stylelab/internal/ids"
	"stylelab/internal/job"
)

type createBibleRequest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

type patchBibleRequest struct {
	Kind    *string `json:"kind"`
	Name    *string `json:"name"`
	Content *string `json:"content"`
	Status  *string `json:"status"`
}

type syncBibleRequest struct {
	Model string `json:"model"`
}

func (s *Server) handleListBible(w http.ResponseWriter, r *http.Request) {
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
	list, err := bible.ListSummaries(r.Context(), s.st, userID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if list == nil {
		list = []bible.EntrySummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": list})
}

func (s *Server) handleCreateBible(w http.ResponseWriter, r *http.Request) {
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
	var in createBibleRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	kind, err := bible.ValidateKind(in.Kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	name, err := bible.ValidateName(in.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	content, err := bible.ValidateContent(in.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	status := bible.StatusActive
	if strings.TrimSpace(in.Status) != "" {
		status, err = bible.ValidateStatus(in.Status)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e := bible.Entry{
		ID:        ids.New(bible.Prefix),
		ProjectID: projectID,
		Kind:      kind,
		Name:      name,
		Content:   content,
		Status:    status,
		Origin:    bible.OriginManual,
		SourceSeq: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := bible.Insert(r.Context(), s.st, e); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleGetBible(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	e, err := bible.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handlePatchBible(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	e, err := bible.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	var in patchBibleRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if in.Kind != nil {
		kind, err := bible.ValidateKind(*in.Kind)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		e.Kind = kind
	}
	if in.Name != nil {
		name, err := bible.ValidateName(*in.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		e.Name = name
	}
	if in.Content != nil {
		content, err := bible.ValidateContent(*in.Content)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		e.Content = content
	}
	if in.Status != nil {
		status, err := bible.ValidateStatus(*in.Status)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		e.Status = status
	}
	e.Origin = bible.OriginManual
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := bible.UpdateFields(r.Context(), s.st, e); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleDeleteBible(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	if err := bible.Delete(r.Context(), s.st, userID, r.PathValue("id")); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSyncBible(w http.ResponseWriter, r *http.Request) {
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
	var in syncBibleRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "invalid json")
			return
		}
	}
	payload, err := json.Marshal(bible.SyncInput{Model: in.Model})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: projectID,
		Kind:      job.KindBibleSync,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}
