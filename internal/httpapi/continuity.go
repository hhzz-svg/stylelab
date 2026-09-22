package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"stylelab/internal/job"
	"stylelab/internal/studio"
)

// The continuity audit reads the whole manuscript and takes ~100s, so it runs
// as a job. The audit report is the job's result; nothing is persisted.
func (s *Server) handleContinuityAudit(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Fail fast on the two things the caller can fix, rather than letting the
	// job start and die a minute later.
	var chapterCount int
	if err := s.st.DB().QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM chapters WHERE project_id = ?`, projectID,
	).Scan(&chapterCount); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if chapterCount == 0 {
		writeError(w, http.StatusBadRequest, "invalid", "作品尚无任何章节内容，无法进行伏笔逻辑审计")
		return
	}
	if _, err := s.loadUserLLMKey(r.Context(), userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return
	}

	s.enqueueStudioJob(w, r, userID, projectID, job.KindContinuity, studio.ContinuityInput{
		ProjectID: projectID,
		Model:     req.Model,
	})
}
