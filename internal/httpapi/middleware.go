package httpapi

import (
	"net/http"

	"stylelab/internal/auth"
)

const sessionCookie = "stylelab_session"
const sessionMaxAge = 14 * 24 * 60 * 60

func (s *Server) currentUserID(r *http.Request) (string, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return "", auth.ErrUnauthorized
	}
	return s.auth.UserIDFromToken(r.Context(), c.Value)
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, rawToken string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}
