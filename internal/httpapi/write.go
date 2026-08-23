package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/write"
)

type createChapterRequest struct {
	Title  string `json:"title"`
	Brief  string `json:"brief"`
	CardID string `json:"card_id"`
}

type patchChapterRequest struct {
	Title       *string `json:"title"`
	Brief       *string `json:"brief"`
	Body        *string `json:"body"`
	Summary     *string `json:"summary"`
	CardID      *string `json:"card_id"`
	TargetRunes *int    `json:"target_runes"`
}

type writeChapterRequest struct {
	Model  string `json:"model"`
	Target int    `json:"target_runes"`
	Note   string `json:"note"`
}

func (s *Server) handleListChapters(w http.ResponseWriter, r *http.Request) {
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
	list, err := write.ListByProject(r.Context(), s.st, userID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if list == nil {
		list = []write.ChapterSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"chapters": list})
}

func (s *Server) handleCreateChapter(w http.ResponseWriter, r *http.Request) {
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
	var in createChapterRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	title, err := write.ValidateTitle(in.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	brief, err := write.ValidateBrief(in.Brief)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	cardID := strings.TrimSpace(in.CardID)
	cardVersion := 0
	if cardID != "" {
		c, err := s.loadOwnedCard(r, userID, cardID, 0)
		if err != nil {
			writeCardError(w, err)
			return
		}
		if c.ProjectID != projectID {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		cardID = c.ID
		cardVersion = c.Version
	}
	seq, err := write.NextSeq(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	ch := write.Chapter{
		ID:          ids.New(write.Prefix),
		ProjectID:   projectID,
		CardID:      cardID,
		CardVersion: cardVersion,
		Seq:         seq,
		Title:       title,
		Brief:       brief,
		Status:      write.StatusDraft,
		TargetRunes: write.ClampTarget(0),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := write.Insert(r.Context(), s.st, ch); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleGetChapter(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	ch, err := write.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handlePatchChapter(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	ch, err := write.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	var in patchChapterRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if in.Title != nil {
		title, err := write.ValidateTitle(*in.Title)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		ch.Title = title
	}
	if in.Brief != nil {
		brief, err := write.ValidateBrief(*in.Brief)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		ch.Brief = brief
	}
	if in.Body != nil {
		ch.Body = *in.Body
		if strings.TrimSpace(ch.Body) != "" && ch.Status != write.StatusWriting {
			ch.Status = write.StatusWritten
		}
	}
	if in.Summary != nil {
		ch.Summary = strings.TrimSpace(*in.Summary)
	}
	if in.TargetRunes != nil {
		ch.TargetRunes = write.ClampTarget(*in.TargetRunes)
	}
	if in.CardID != nil {
		cardID := strings.TrimSpace(*in.CardID)
		if cardID == "" {
			ch.CardID = ""
			ch.CardVersion = 0
		} else {
			c, err := s.loadOwnedCard(r, userID, cardID, 0)
			if err != nil {
				writeCardError(w, err)
				return
			}
			if c.ProjectID != ch.ProjectID {
				writeError(w, http.StatusNotFound, "not_found", "not found")
				return
			}
			ch.CardID = c.ID
			ch.CardVersion = c.Version
		}
	}
	ch.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := write.UpdateFields(r.Context(), s.st, ch); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleDeleteChapter(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	if err := write.DeleteAndRenumber(r.Context(), s.st, userID, r.PathValue("id")); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWriteChapter(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	ch, err := write.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	var in writeChapterRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "invalid json")
			return
		}
	}
	note, err := write.ValidateNote(in.Note)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if strings.TrimSpace(ch.CardID) == "" {
		writeError(w, http.StatusBadRequest, "invalid", "先选一张风格卡再写")
		return
	}
	if _, err := write.ValidateBrief(ch.Brief); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if err := write.MarkWriting(r.Context(), s.st, ch.ID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	payload, err := json.Marshal(write.Input{
		ChapterID: ch.ID,
		Model:     in.Model,
		Target:    in.Target,
		Note:      note,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: ch.ProjectID,
		Kind:      job.KindWrite,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) handleManuscript(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	projectID := r.PathValue("id")
	var name string
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT name FROM projects WHERE id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	md, err := write.ManuscriptMarkdown(r.Context(), s.st, userID, projectID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="manuscript.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(md))
}
