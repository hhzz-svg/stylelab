package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/ids"
)

type createProjectRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var in createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	name := strings.TrimSpace(in.Name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 80 {
		writeError(w, http.StatusBadRequest, "invalid", "name must be 1-80 characters")
		return
	}
	id := ids.New("prj_")
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = s.st.DB().ExecContext(
		r.Context(),
		`INSERT INTO projects (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`,
		id, userID, name, createdAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         id,
		"name":       name,
		"created_at": createdAt,
	})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, name, created_at FROM projects WHERE user_id = ? ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	projects := make([]map[string]string, 0)
	for rows.Next() {
		var id, name, createdAt string
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		projects = append(projects, map[string]string{
			"id":         id,
			"name":       name,
			"created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id := r.PathValue("id")
	var name, createdAt string
	err = s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT name, created_at FROM projects WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(&name, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":         id,
		"name":       name,
		"created_at": createdAt,
	})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	id := r.PathValue("id")
	res, err := s.st.DB().ExecContext(
		r.Context(),
		`DELETE FROM projects WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	n, err := res.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
