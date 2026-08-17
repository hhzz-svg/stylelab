package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"stylelab/internal/job"
)

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	rec, err := s.jobs.Get(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	if err := s.jobs.Cancel(r.Context(), r.PathValue("id"), userID); err != nil {
		writeJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id := r.PathValue("id")
	rec, err := s.jobs.Get(r.Context(), id, userID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "invalid", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeProgressEvent(w, flusher, rec)
	if isTerminal(rec.Status) {
		return
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	last := rec
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			rec, err := s.jobs.Get(r.Context(), id, userID)
			if err != nil {
				return
			}
			if rec.Progress != last.Progress || rec.Stage != last.Stage || rec.Status != last.Status {
				writeProgressEvent(w, flusher, rec)
				last = rec
			}
			if isTerminal(rec.Status) {
				return
			}
		}
	}
}

func writeProgressEvent(w http.ResponseWriter, flusher http.Flusher, rec job.Record) {
	payload, _ := json.Marshal(map[string]any{
		"progress": rec.Progress,
		"stage":    rec.Stage,
		"status":   rec.Status,
	})
	_, _ = w.Write([]byte("event: progress\ndata: "))
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n\n"))
	flusher.Flush()
}

func isTerminal(st job.Status) bool {
	return st == job.StatusSucceeded || st == job.StatusFailed || st == job.StatusCanceled
}

func writeJobError(w http.ResponseWriter, err error) {
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "invalid", "internal error")
}
