package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"stylelab/internal/lore"
	"stylelab/internal/write"
)

// Scene segmentation is deterministic and fast (no model), so these answer
// synchronously.

func (s *Server) ownedChapterForScenes(w http.ResponseWriter, r *http.Request) (write.Chapter, bool) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return write.Chapter{}, false
	}
	ch, err := write.LoadOwned(r.Context(), s.st, userID, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return write.Chapter{}, false
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return write.Chapter{}, false
	}
	return ch, true
}

func (s *Server) handleChapterScenes(w http.ResponseWriter, r *http.Request) {
	ch, ok := s.ownedChapterForScenes(w, r)
	if !ok {
		return
	}
	out, err := lore.LoadScenes(r.Context(), s.st, ch.ID, ch.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSplitChapterScenes(w http.ResponseWriter, r *http.Request) {
	ch, ok := s.ownedChapterForScenes(w, r)
	if !ok {
		return
	}
	scenes, err := lore.SplitChapter(r.Context(), s.st, ch.ProjectID, ch.ID, ch.Body)
	if err != nil {
		if errors.Is(err, lore.ErrNoBody) {
			writeError(w, http.StatusBadRequest, "invalid", trimInvalid(err))
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, lore.ChapterScenes{Scenes: scenes})
}

func (s *Server) handleUpdateScene(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var edit lore.SceneEdit
	if err := json.NewDecoder(r.Body).Decode(&edit); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	scene, err := lore.UpdateScene(r.Context(), s.st, userID, r.PathValue("id"), edit)
	switch {
	case errors.Is(err, lore.ErrSceneNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case err != nil && strings.HasPrefix(err.Error(), "invalid:"):
		writeError(w, http.StatusBadRequest, "invalid", trimInvalid(err))
	case err != nil:
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
	default:
		writeJSON(w, http.StatusOK, scene)
	}
}

func (s *Server) handleSplitAllScenes(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	chapters, scenes, err := lore.SplitAll(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"chapters": chapters, "scenes": scenes})
}
