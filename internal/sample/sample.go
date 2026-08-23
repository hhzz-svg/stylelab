package sample

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/stylestat"
)

const (
	defaultTargetRunes = 1200
	minTargetRunes     = 800
	maxTargetRunes     = 2000
	maxPremiseRunes    = 80
	defaultSampleModel = "gpt-4o-mini"
	sampleChatTemp     = 0.7
)

type Input struct {
	CardID  string `json:"card_id"`
	Premise string `json:"premise"`
	Target  int    `json:"target_runes"` // default 1200, min 800, max 2000
	Model   string `json:"model"`
}

type Sample struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"project_id"`
	CardID      string          `json:"card_id"`
	CardVersion int             `json:"card_version"`
	Premise     string          `json:"premise"`
	Body        string          `json:"body"`
	Facts       json.RawMessage `json:"facts"`
	Target      int             `json:"target_runes"`
}

type llmKey struct {
	Provider string
	BaseURL  string
	APIKey   string
}

func JobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in Input
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		if in.CardID == "" {
			return nil, fmt.Errorf("invalid: card_id required")
		}
		out, err := Run(ctx, st, client, master, rec.UserID, in, prog)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"sample_id": out.ID})
	}
}

func Run(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID string, in Input, prog func(int, string)) (Sample, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	premise := strings.TrimSpace(in.Premise)
	if premise == "" {
		return Sample{}, fmt.Errorf("invalid: premise required")
	}
	if utf8.RuneCountInString(premise) > maxPremiseRunes {
		return Sample{}, fmt.Errorf("invalid: premise too long")
	}
	target := in.Target
	if target == 0 {
		target = defaultTargetRunes
	}
	if target < minTargetRunes || target > maxTargetRunes {
		return Sample{}, fmt.Errorf("invalid: target_runes must be between %d and %d", minTargetRunes, maxTargetRunes)
	}

	prog(10, "load_card")
	c, err := loadOwnedCard(ctx, st, userID, in.CardID)
	if err != nil {
		return Sample{}, err
	}

	prog(25, "llm_key")
	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return Sample{}, err
	}

	prog(50, "generate")
	body, err := chatSample(ctx, client, key, in.Model, c, premise, target)
	if err != nil {
		return Sample{}, err
	}

	prog(75, "stylestat")
	stats := stylestat.Compute(stylestat.Input{Chapters: []string{body}})
	facts, err := json.Marshal(stats)
	if err != nil {
		return Sample{}, err
	}

	out := Sample{
		ID:          ids.New("smp_"),
		ProjectID:   c.ProjectID,
		CardID:      c.ID,
		CardVersion: c.Version,
		Premise:     premise,
		Body:        body,
		Facts:       facts,
		Target:      target,
	}

	prog(90, "persist")
	if err := persistSample(ctx, st, out); err != nil {
		return Sample{}, err
	}
	prog(100, "done")
	return out, nil
}

func loadOwnedCard(ctx context.Context, st *store.Store, userID, cardID string) (card.Card, error) {
	var (
		id, projectID, name, kind                    string
		version                                      int
		dimsJSON, prohibJSON, factsJSON, lineageJSON sql.NullString
	)
	err := st.DB().QueryRowContext(
		ctx,
		`SELECT c.id, c.project_id, c.name, c.kind, v.version,
		        v.dimensions_json, v.prohibitions_json, v.facts_json, v.lineage_json
		 FROM style_cards c
		 JOIN projects p ON p.id = c.project_id
		 JOIN style_card_versions v ON v.card_id = c.id AND v.version = c.current_version
		 WHERE c.id = ? AND p.user_id = ?`,
		cardID, userID,
	).Scan(&id, &projectID, &name, &kind, &version, &dimsJSON, &prohibJSON, &factsJSON, &lineageJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return card.Card{}, fmt.Errorf("invalid: card not found")
		}
		return card.Card{}, err
	}
	out := card.Card{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		Version:   version,
		Facts:     json.RawMessage(`{}`),
	}
	if err := json.Unmarshal([]byte(dimsJSON.String), &out.Dimensions); err != nil {
		return card.Card{}, err
	}
	if err := json.Unmarshal([]byte(prohibJSON.String), &out.Prohibitions); err != nil {
		return card.Card{}, err
	}
	if factsJSON.Valid && strings.TrimSpace(factsJSON.String) != "" {
		out.Facts = json.RawMessage(factsJSON.String)
	}
	if lineageJSON.Valid && strings.TrimSpace(lineageJSON.String) != "" && lineageJSON.String != "null" {
		var lin card.Lineage
		if err := json.Unmarshal([]byte(lineageJSON.String), &lin); err != nil {
			return card.Card{}, err
		}
		out.Lineage = &lin
	}
	return out, nil
}

func loadUserKey(ctx context.Context, st *store.Store, master []byte, userID string) (llmKey, error) {
	rows, err := st.DB().QueryContext(
		ctx,
		`SELECT provider, base_url, encrypted_key FROM user_llm_keys WHERE user_id = ? ORDER BY provider`,
		userID,
	)
	if err != nil {
		return llmKey{}, err
	}
	defer rows.Close()

	var keys []llmKey
	for rows.Next() {
		var provider, baseURL string
		var blob []byte
		if err := rows.Scan(&provider, &baseURL, &blob); err != nil {
			return llmKey{}, err
		}
		plain, err := cryptokey.Open(master, blob)
		if err != nil {
			return llmKey{}, err
		}
		keys = append(keys, llmKey{Provider: provider, BaseURL: baseURL, APIKey: plain})
	}
	if err := rows.Err(); err != nil {
		return llmKey{}, err
	}
	if len(keys) == 0 {
		return llmKey{}, fmt.Errorf("invalid: missing llm key")
	}
	for _, k := range keys {
		if k.Provider == "chat" || k.Provider == "response" || k.Provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}

func chatSample(ctx context.Context, client *llm.Client, key llmKey, model string, c card.Card, premise string, target int) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = defaultSampleModel
	}
	cardJSON, err := json.Marshal(map[string]any{
		"id":           c.ID,
		"name":         c.Name,
		"kind":         c.Kind,
		"version":      c.Version,
		"dimensions":   c.Dimensions,
		"prohibitions": c.Prohibitions,
	})
	if err != nil {
		return "", err
	}
	userContent := fmt.Sprintf("前提：%s\n目标字数：%d", premise, target)
	req := llm.Request{
		Provider: key.Provider,
		BaseURL:  key.BaseURL,
		APIKey:   key.APIKey,
		Model:    model,
		Temp:     sampleChatTemp,
		Messages: []llm.Message{
			{Role: "system", Content: llm.SampleSystem(string(cardJSON))},
			{Role: "user", Content: userContent},
		},
	}
	body, err := client.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("invalid: empty sample body")
	}
	if llm.ForbiddenCopyCheck(body) {
		return "", fmt.Errorf("invalid: sample contains banned copy")
	}
	return body, nil
}

func persistSample(ctx context.Context, st *store.Store, s Sample) error {
	_, err := st.DB().ExecContext(
		ctx,
		`INSERT INTO samples (id, project_id, card_id, card_version, premise, body, facts_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.CardID, s.CardVersion, s.Premise, s.Body, string(s.Facts),
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
