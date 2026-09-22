package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"stylelab/internal/job"
	"stylelab/internal/studio"
)

type branchSimulateRequest struct {
	CurrentText string `json:"current_text"`
	Model       string `json:"model"`
}

type chapterContinueRequest struct {
	CurrentText string `json:"current_text"`
	Instruction string `json:"instruction"`
	TargetRunes int    `json:"target_runes"`
	Model       string `json:"model"`
}

// chapterProject resolves the owning project, scoped by user so another
// user's chapter is indistinguishable from a missing one.
func (s *Server) chapterProject(r *http.Request, chapterID, userID string) (string, error) {
	var projectID string
	err := s.st.DB().QueryRowContext(r.Context(),
		`SELECT c.project_id FROM chapters c
		 JOIN projects p ON p.id = c.project_id
		 WHERE c.id = ? AND p.user_id = ?`,
		chapterID, userID,
	).Scan(&projectID)
	return projectID, err
}

// Both branch features take ~90s against the model, so they run as jobs and
// return their data as the job result.
func (s *Server) handleBranchSimulate(w http.ResponseWriter, r *http.Request) {
	userID, chapterID, projectID, ok := s.studioChapterPreflight(w, r)
	if !ok {
		return
	}

	var req branchSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	s.enqueueStudioJob(w, r, userID, projectID, job.KindBranch, studio.BranchInput{
		ChapterID:   chapterID,
		CurrentText: req.CurrentText,
		Model:       req.Model,
	})
}

func (s *Server) handleChapterContinue(w http.ResponseWriter, r *http.Request) {
	userID, chapterID, projectID, ok := s.studioChapterPreflight(w, r)
	if !ok {
		return
	}

	var req chapterContinueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	s.enqueueStudioJob(w, r, userID, projectID, job.KindContinue, studio.ContinueInput{
		ChapterID:   chapterID,
		CurrentText: req.CurrentText,
		Instruction: req.Instruction,
		TargetRunes: req.TargetRunes,
		Model:       req.Model,
	})
}

// studioChapterPreflight resolves auth, chapter ownership and the presence of
// an LLM key -- everything worth answering with a status code before a job is
// queued. It writes the error response itself and reports whether to continue.
func (s *Server) studioChapterPreflight(w http.ResponseWriter, r *http.Request) (userID, chapterID, projectID string, ok bool) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return "", "", "", false
	}
	chapterID = r.PathValue("id")
	projectID, err = s.chapterProject(r, chapterID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "chapter not found")
			return "", "", "", false
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return "", "", "", false
	}
	if _, err := s.loadUserLLMKey(r.Context(), userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "未配置 LLM 模型密钥，请前往设置配置")
		return "", "", "", false
	}
	return userID, chapterID, projectID, true
}
