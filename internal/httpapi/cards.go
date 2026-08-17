package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/fuse"
	"stylelab/internal/job"
	"stylelab/internal/llm"
)

type createVersionRequest struct {
	Name             string         `json:"name"`
	Levels           map[string]int `json:"levels"`
	RewriteSummaries bool           `json:"rewrite_summaries"`
	Model            string         `json:"model"`
}

func (s *Server) handleListCards(w http.ResponseWriter, r *http.Request) {
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

	rows, err := s.st.DB().QueryContext(
		r.Context(),
		`SELECT id, name, kind, current_version, updated_at
		 FROM style_cards WHERE project_id = ? ORDER BY updated_at DESC, id DESC`,
		projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	defer rows.Close()

	cards := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, kind, updatedAt string
		var version int
		if err := rows.Scan(&id, &name, &kind, &version, &updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "invalid", "internal error")
			return
		}
		cards = append(cards, map[string]any{
			"id":              id,
			"name":            name,
			"kind":            kind,
			"current_version": version,
			"updated_at":      updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

func (s *Server) handleGetCard(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	c, err := s.loadOwnedCard(r, userID, r.PathValue("id"), 0)
	if err != nil {
		writeCardError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleGetCardVersion(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "invalid", "invalid version")
		return
	}
	c, err := s.loadOwnedCard(r, userID, r.PathValue("id"), n)
	if err != nil {
		writeCardError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCreateCardVersion(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var in createVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}

	cur, err := s.loadOwnedCard(r, userID, r.PathValue("id"), 0)
	if err != nil {
		writeCardError(w, err)
		return
	}

	next := cur
	if name := strings.TrimSpace(in.Name); name != "" {
		next.Name = name
	}
	if next.Dimensions == nil {
		next.Dimensions = map[string]card.Dimension{}
	} else {
		copied := make(map[string]card.Dimension, len(cur.Dimensions))
		for k, d := range cur.Dimensions {
			copied[k] = d
		}
		next.Dimensions = copied
	}

	changed := make([]string, 0)
	for _, key := range card.DimensionKeys {
		level, ok := in.Levels[key]
		if !ok {
			continue
		}
		if level < 0 || level > 100 {
			writeError(w, http.StatusBadRequest, "invalid", "level must be 0-100")
			return
		}
		d := next.Dimensions[key]
		if d.Level != level {
			changed = append(changed, key)
		}
		d.Level = level
		next.Dimensions[key] = d
	}

	if in.RewriteSummaries && len(changed) > 0 {
		rewritten, err := s.rewriteChangedSummaries(r.Context(), userID, in.Model, next, changed)
		if err != nil {
			writeVersionError(w, err)
			return
		}
		for _, key := range changed {
			d := next.Dimensions[key]
			if rd, ok := rewritten[key]; ok {
				d.Summary = rd.Summary
				d.Techniques = rd.Techniques
				next.Dimensions[key] = d
			}
		}
	}

	next.Version = cur.Version + 1
	if err := card.Validate(next); err != nil {
		writeVersionError(w, err)
		return
	}
	if err := s.persistNewVersion(r.Context(), next); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, next)
}

func (s *Server) handleExportCard(w http.ResponseWriter, r *http.Request) {
	userID, err := s.currentUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	c, err := s.loadOwnedCard(r, userID, r.PathValue("id"), 0)
	if err != nil {
		writeCardError(w, err)
		return
	}
	body := card.ToSimulationProfile(c)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="simulation_profile.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleFuse(w http.ResponseWriter, r *http.Request) {
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

	var in fuse.FuseSpec
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	payload, err := json.Marshal(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	jobID, err := s.jobs.Enqueue(r.Context(), job.Record{
		UserID:    userID,
		ProjectID: projectID,
		Kind:      job.KindFuse,
		Payload:   payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid", "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) loadOwnedCard(r *http.Request, userID, cardID string, version int) (card.Card, error) {
	var (
		id, projectID, name, kind                    string
		ver                                          int
		dimsJSON, prohibJSON, factsJSON, lineageJSON sql.NullString
	)
	err := s.st.DB().QueryRowContext(
		r.Context(),
		`SELECT c.id, c.project_id, c.name, c.kind, v.version,
		        v.dimensions_json, v.prohibitions_json, v.facts_json, v.lineage_json
		 FROM style_cards c
		 JOIN projects p ON p.id = c.project_id
		 JOIN style_card_versions v ON v.card_id = c.id
		  AND v.version = CASE WHEN ? = 0 THEN c.current_version ELSE ? END
		 WHERE c.id = ? AND p.user_id = ?`,
		version, version, cardID, userID,
	).Scan(&id, &projectID, &name, &kind, &ver, &dimsJSON, &prohibJSON, &factsJSON, &lineageJSON)
	if err != nil {
		return card.Card{}, err
	}
	c := card.Card{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		Version:   ver,
		Facts:     json.RawMessage(`{}`),
	}
	if err := json.Unmarshal([]byte(dimsJSON.String), &c.Dimensions); err != nil {
		return card.Card{}, err
	}
	if err := json.Unmarshal([]byte(prohibJSON.String), &c.Prohibitions); err != nil {
		return card.Card{}, err
	}
	if factsJSON.Valid && factsJSON.String != "" {
		c.Facts = json.RawMessage(factsJSON.String)
	}
	if lineageJSON.Valid && lineageJSON.String != "" && lineageJSON.String != "null" {
		var lin card.Lineage
		if err := json.Unmarshal([]byte(lineageJSON.String), &lin); err != nil {
			return card.Card{}, err
		}
		c.Lineage = &lin
	}
	return c, nil
}

func writeCardError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "invalid", "internal error")
}

func writeVersionError(w http.ResponseWriter, err error) {
	var ce *card.Error
	if errors.As(err, &ce) && ce.Code == "invalid" {
		writeError(w, http.StatusBadRequest, "invalid", ce.Message)
		return
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "invalid:") {
		writeError(w, http.StatusBadRequest, "invalid", strings.TrimSpace(strings.TrimPrefix(msg, "invalid:")))
		return
	}
	writeError(w, http.StatusInternalServerError, "invalid", "internal error")
}

func (s *Server) persistNewVersion(ctx context.Context, c card.Card) error {
	now := time.Now().UTC().Format(time.RFC3339)
	dims, err := json.Marshal(c.Dimensions)
	if err != nil {
		return err
	}
	prohibitions, err := json.Marshal(c.Prohibitions)
	if err != nil {
		return err
	}
	facts := c.Facts
	if len(bytes.TrimSpace(facts)) == 0 {
		facts = json.RawMessage(`{}`)
	}
	var lineage any
	if c.Lineage != nil {
		raw, err := json.Marshal(c.Lineage)
		if err != nil {
			return err
		}
		lineage = string(raw)
	}
	tx, err := s.st.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(
		ctx,
		`UPDATE style_cards SET name = ?, current_version = ?, updated_at = ?
		 WHERE id = ? AND current_version = ?`,
		c.Name, c.Version, now, c.ID, c.Version-1,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("invalid: card version conflict")
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Version, string(dims), string(prohibitions), string(facts), lineage, now,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) rewriteChangedSummaries(ctx context.Context, userID, model string, c card.Card, changed []string) (map[string]card.Dimension, error) {
	key, err := s.loadUserLLMKey(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}
	subset := make(map[string]card.Dimension, len(changed))
	for _, keyName := range changed {
		subset[keyName] = c.Dimensions[keyName]
	}
	payload, err := json.Marshal(map[string]any{"dimensions": subset})
	if err != nil {
		return nil, err
	}
	raw, err := s.llm.Chat(ctx, llm.Request{
		Provider: key.provider,
		BaseURL:  key.baseURL,
		APIKey:   key.apiKey,
		Model:    model,
		Temp:     0.3,
		Messages: []llm.Message{
			{Role: "system", Content: llm.FuseSystem()},
			{Role: "user", Content: "只重写下列已改 level 的维度的 summary 与 techniques，不要改 level，不要增删键：\n" + string(payload)},
		},
	})
	if err != nil {
		return nil, err
	}
	return parseRewriteJSON(raw)
}

type llmKeyRow struct {
	provider string
	baseURL  string
	apiKey   string
}

func (s *Server) loadUserLLMKey(ctx context.Context, userID string) (llmKeyRow, error) {
	rows, err := s.st.DB().QueryContext(
		ctx,
		`SELECT provider, base_url, encrypted_key FROM user_llm_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		return llmKeyRow{}, err
	}
	defer rows.Close()

	var keys []llmKeyRow
	for rows.Next() {
		var provider, baseURL string
		var blob []byte
		if err := rows.Scan(&provider, &baseURL, &blob); err != nil {
			return llmKeyRow{}, err
		}
		plain, err := cryptokey.Open(s.cfg.MasterKey, blob)
		if err != nil {
			return llmKeyRow{}, err
		}
		keys = append(keys, llmKeyRow{provider: provider, baseURL: baseURL, apiKey: plain})
	}
	if err := rows.Err(); err != nil {
		return llmKeyRow{}, err
	}
	if len(keys) == 0 {
		return llmKeyRow{}, fmt.Errorf("invalid: missing llm key")
	}
	for _, k := range keys {
		if k.provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}

func parseRewriteJSON(raw string) (map[string]card.Dimension, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	var out struct {
		Dimensions map[string]card.Dimension `json:"dimensions"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("invalid: rewrite json: %w", err)
	}
	if len(out.Dimensions) == 0 {
		return nil, fmt.Errorf("invalid: rewrite json missing dimensions")
	}
	return out.Dimensions, nil
}
