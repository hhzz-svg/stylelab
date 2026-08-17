package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"stylelab/internal/auth"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	userID, err := s.auth.Register(r.Context(), in.Email, in.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	raw, _, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookie(w, r, raw, sessionMaxAge)
	writeJSON(w, http.StatusCreated, map[string]string{"user_id": userID})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	raw, userID, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookie(w, r, raw, sessionMaxAge)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.auth.Logout(r.Context(), c.Value)
	}
	setSessionCookie(w, r, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var email string
	if err := s.st.DB().QueryRowContext(r.Context(), `SELECT email FROM users WHERE id = ?`, userID).Scan(&email); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"user_id": userID,
		"email":   email,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid", err.Error())
	case errors.Is(err, auth.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "email already registered")
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
	default:
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
	}
}
