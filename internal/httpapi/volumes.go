package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"stylelab/internal/lore"
)

// handleStructure returns the book as volume -> chapter -> scene.
func (s *Server) handleStructure(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	out, err := lore.LoadStructure(r.Context(), s.st, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeVolumeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, lore.ErrVolumeNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case strings.HasPrefix(err.Error(), "invalid:"):
		writeError(w, http.StatusBadRequest, "invalid", trimInvalid(err))
	default:
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
	}
}

func (s *Server) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.ownedProjectForInsight(w, r)
	if !ok {
		return
	}
	var in lore.VolumeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	v, err := lore.CreateVolume(r.Context(), s.st, projectID, in)
	if err != nil {
		writeVolumeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handleUpdateVolume(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var in lore.VolumeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	v, err := lore.UpdateVolume(r.Context(), s.st, userID, r.PathValue("id"), in)
	if err != nil {
		writeVolumeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	if err := lore.DeleteVolume(r.Context(), s.st, userID, r.PathValue("id")); err != nil {
		writeVolumeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
