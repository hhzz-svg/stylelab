package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/studio"
	"stylelab/internal/write"
)

// Outline sizing caps. maxOutlineBriefRunes matches write.maxBriefRunes, which
// is what handleWriteChapter later validates an imported brief against.
const (
	maxOutlineChapters   = 100
	maxOutlineBriefRunes = 200
)

type generateOutlineRequest struct {
	Premise        string `json:"premise"`
	Genre          string `json:"genre"`
	TargetChapters int    `json:"target_chapters"`
	VolumeCount    int    `json:"volume_count"`
	Model          string `json:"model"`
	CardID         string `json:"card_id"`
}

type importOutlineRequest struct {
	Chapters        []outlineChapterItem `json:"chapters"`
	ReplaceExisting bool                 `json:"replace_existing"`
	CardID          string               `json:"card_id"`
}

type outlineChapterItem struct {
	Title string `json:"title"`
	Brief string `json:"brief"`
	Hook  string `json:"hook"`
}

func (s *Server) handleGenerateOutline(w http.ResponseWriter, r *http.Request) {
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

	var req generateOutlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	in := studio.OutlineInput{
		ProjectID:      projectID,
		Premise:        req.Premise,
		Genre:          req.Genre,
		TargetChapters: req.TargetChapters,
		VolumeCount:    req.VolumeCount,
		Model:          req.Model,
	}
	// Validate now so the caller gets a 400 rather than a job that fails a
	// minute later.
	if err := studio.ValidateOutlineInput(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", trimInvalid(err))
		return
	}
	if _, err := s.loadUserLLMKey(r.Context(), userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	s.enqueueStudioJob(w, r, userID, projectID, job.KindOutline, in)
}

func (s *Server) handleImportOutline(w http.ResponseWriter, r *http.Request) {
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

	var req importOutlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	if len(req.Chapters) == 0 {
		writeError(w, http.StatusBadRequest, "invalid", "chapters list is empty")
		return
	}
	if len(req.Chapters) > maxOutlineChapters {
		writeError(w, http.StatusBadRequest, "invalid", fmt.Sprintf("一次最多导入 %d 章", maxOutlineChapters))
		return
	}

	cardID := strings.TrimSpace(req.CardID)
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

	// Validate every chapter before opening the transaction so a bad item
	// cannot leave a half-imported outline behind. An empty brief matters:
	// handleWriteChapter rejects one, so such a chapter could never be written.
	type preparedChapter struct{ title, brief string }
	prepared := make([]preparedChapter, 0, len(req.Chapters))
	for i, ch := range req.Chapters {
		title, err := write.ValidateTitle(ch.Title)
		if err != nil {
			title = fmt.Sprintf("第%d章", i+1)
		}
		brief := strings.TrimSpace(ch.Brief)
		if hook := strings.TrimSpace(ch.Hook); hook != "" && !strings.Contains(brief, hook) {
			brief = strings.TrimSpace(brief + " (伏笔: " + hook + ")")
		}
		// Clamp before validating: an over-long brief from the model is worth
		// truncating, but an empty one is a hard error.
		brief, err = write.ValidateBrief(headRunes(brief, maxOutlineBriefRunes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", fmt.Sprintf("第 %d 章缺少章节梗概", i+1))
			return
		}
		prepared = append(prepared, preparedChapter{title: title, brief: brief})
	}

	tx, err := s.st.DB().BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer tx.Rollback()

	// "Replace" must never discard finished work: drop only chapters that carry
	// no prose. Surviving chapters keep their seq, so the import appends after
	// them -- chapters has UNIQUE (project_id, seq), so renumbering around them
	// is not an option.
	protectedCount := 0
	if req.ReplaceExisting {
		if err := tx.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM chapters WHERE project_id = ? AND (status = ? OR TRIM(body) <> '')`,
			projectID, write.StatusWritten,
		).Scan(&protectedCount); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM chapters WHERE project_id = ? AND status <> ? AND TRIM(body) = ''`,
			projectID, write.StatusWritten,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}

	startSeq := 1
	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(r.Context(), `SELECT MAX(seq) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxSeq); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if maxSeq.Valid {
		startSeq = int(maxSeq.Int64) + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	insertedCount := 0

	for i, ch := range prepared {
		seq := startSeq + i
		chapterID := ids.New(write.Prefix)
		_, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO chapters (id, project_id, card_id, card_version, seq, title, brief, body, summary, status, target_runes, model, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, '', '', ?, ?, '', ?, ?)`,
			chapterID, projectID, cardID, cardVersion, seq, ch.title, ch.brief, write.StatusDraft, write.ClampTarget(0), now, now,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("插入第 %d 章失败: %v", seq, err))
			return
		}
		insertedCount++
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"inserted_count":  insertedCount,
		"protected_count": protectedCount,
	})
}
