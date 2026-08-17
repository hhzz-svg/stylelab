package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/cryptokey"
)

var allowedProviders = map[string]bool{
	"openai":     true,
	"anthropic":  true,
	"compatible": true,
}

type putLLMKeyRequest struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

func (s *Server) handlePutLLMKey(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var in putLLMKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	provider := strings.TrimSpace(in.Provider)
	if !allowedProviders[provider] {
		writeError(w, http.StatusBadRequest, "invalid", "invalid provider")
		return
	}
	if strings.TrimSpace(in.APIKey) == "" {
		writeError(w, http.StatusBadRequest, "invalid", "api_key is required")
		return
	}
	blob, err := cryptokey.Seal(s.cfg.MasterKey, in.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.st.DB().ExecContext(
		r.Context(),
		`INSERT INTO user_llm_keys (user_id, provider, base_url, encrypted_key, last4, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, provider) DO UPDATE SET
		   base_url = excluded.base_url,
		   encrypted_key = excluded.encrypted_key,
		   last4 = excluded.last4,
		   updated_at = excluded.updated_at`,
		userID,
		provider,
		in.BaseURL,
		blob,
		last4Runes(in.APIKey),
		now,
		now,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"provider": provider,
		"last4":    last4Runes(in.APIKey),
	})
}

func (s *Server) handleListLLMKeys(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT provider, base_url, last4 FROM user_llm_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	keys := make([]map[string]string, 0)
	for rows.Next() {
		var provider, baseURL, last4 string
		if err := rows.Scan(&provider, &baseURL, &last4); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		keys = append(keys, map[string]string{
			"provider": provider,
			"base_url": baseURL,
			"last4":    last4,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *Server) handleDeleteLLMKey(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	provider := r.PathValue("provider")
	if !allowedProviders[provider] {
		writeError(w, http.StatusBadRequest, "invalid", "invalid provider")
		return
	}
	_, err = s.st.DB().ExecContext(
		r.Context(),
		`DELETE FROM user_llm_keys WHERE user_id = ? AND provider = ?`,
		userID,
		provider,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func last4Runes(s string) string {
	n := utf8.RuneCountInString(s)
	if n <= 4 {
		return s
	}
	return string([]rune(s)[n-4:])
}
