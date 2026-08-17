package fuse

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/job"
	"stylelab/internal/llm"
	"stylelab/internal/store"
)

const (
	fuseChatTemp     = 0.3
	fuseParseRetries = 2
	defaultFuseModel = "gpt-4o-mini"
	defaultFuseName  = "融合风格"
)

type FuseSpec struct {
	Name    string           `json:"name"`
	Parents []card.ParentRef `json:"parents"`
	Model   string           `json:"model"`
}

type llmKey struct {
	Provider string
	BaseURL  string
	APIKey   string
}

func JobHandler(st *store.Store, client *llm.Client, master []byte) job.Handler {
	return func(ctx context.Context, rec job.Record, prog func(int, string)) (json.RawMessage, error) {
		var in FuseSpec
		if err := json.Unmarshal(rec.Payload, &in); err != nil {
			return nil, err
		}
		c, conflicts, err := Run(ctx, st, client, master, rec.UserID, rec.ProjectID, in, prog)
		if err != nil {
			return nil, err
		}
		if conflicts == nil {
			conflicts = []string{}
		}
		return json.Marshal(map[string]any{
			"card_id":   c.ID,
			"conflicts": conflicts,
		})
	}
}

func Run(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID, projectID string, in FuseSpec, prog func(int, string)) (card.Card, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	prog(10, "load_parents")
	parents, err := loadParents(ctx, st, userID, projectID, in.Parents)
	if err != nil {
		return card.Card{}, nil, err
	}

	prog(30, "blend")
	dims, prohibitions, err := card.Blend(parents, in.Parents)
	if err != nil {
		return card.Card{}, nil, err
	}

	prog(45, "llm_key")
	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return card.Card{}, nil, err
	}

	prog(65, "fuse")
	rewritten, conflicts, err := chatFuse(ctx, client, key, in.Model, parents, in.Parents, dims, prohibitions)
	if err != nil {
		return card.Card{}, nil, err
	}
	for key, d := range rewritten {
		base := dims[key]
		base.Summary = d.Summary
		base.Techniques = d.Techniques
		dims[key] = base
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = defaultFuseName
	}
	out := card.Card{
		ID:           ids.New("crd_"),
		ProjectID:    projectID,
		Name:         name,
		Kind:         "fused",
		Version:      1,
		Dimensions:   dims,
		Prohibitions: prohibitions,
		Facts:        json.RawMessage(`{}`),
		Lineage: &card.Lineage{
			ParentCards:   in.Parents,
			PromptVersion: "fuse-v1",
		},
	}
	if err := card.Validate(out); err != nil {
		return card.Card{}, nil, err
	}

	prog(85, "persist")
	if err := persistCard(ctx, st, out); err != nil {
		return card.Card{}, nil, err
	}
	prog(100, "done")
	if conflicts == nil {
		conflicts = []string{}
	}
	return out, conflicts, nil
}

func loadParents(ctx context.Context, st *store.Store, userID, projectID string, refs []card.ParentRef) ([]card.Card, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("invalid: parents required")
	}
	out := make([]card.Card, 0, len(refs))
	for _, ref := range refs {
		c, err := loadParent(ctx, st, userID, projectID, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func loadParent(ctx context.Context, st *store.Store, userID, projectID string, ref card.ParentRef) (card.Card, error) {
	var (
		id, pid, name, kind                          string
		version                                      int
		dimsJSON, prohibJSON, factsJSON, lineageJSON sql.NullString
	)
	err := st.DB().QueryRowContext(
		ctx,
		`SELECT c.id, c.project_id, c.name, c.kind, v.version,
		        v.dimensions_json, v.prohibitions_json, v.facts_json, v.lineage_json
		 FROM style_cards c
		 JOIN projects p ON p.id = c.project_id
		 JOIN style_card_versions v ON v.card_id = c.id
		  AND v.version = CASE WHEN ? = 0 THEN c.current_version ELSE ? END
		 WHERE c.id = ? AND c.project_id = ? AND p.user_id = ?`,
		ref.Version, ref.Version, ref.CardID, projectID, userID,
	).Scan(&id, &pid, &name, &kind, &version, &dimsJSON, &prohibJSON, &factsJSON, &lineageJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return card.Card{}, fmt.Errorf("invalid: parent card not found")
		}
		return card.Card{}, err
	}
	return decodeCard(id, pid, name, kind, version, dimsJSON.String, prohibJSON.String, factsJSON.String, lineageJSON)
}

func decodeCard(id, projectID, name, kind string, version int, dimsJSON, prohibJSON, factsJSON string, lineageJSON sql.NullString) (card.Card, error) {
	c := card.Card{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		Version:   version,
		Facts:     json.RawMessage(`{}`),
	}
	if err := json.Unmarshal([]byte(dimsJSON), &c.Dimensions); err != nil {
		return card.Card{}, err
	}
	if err := json.Unmarshal([]byte(prohibJSON), &c.Prohibitions); err != nil {
		return card.Card{}, err
	}
	if strings.TrimSpace(factsJSON) != "" {
		c.Facts = json.RawMessage(factsJSON)
	}
	if lineageJSON.Valid && strings.TrimSpace(lineageJSON.String) != "" && lineageJSON.String != "null" {
		var lin card.Lineage
		if err := json.Unmarshal([]byte(lineageJSON.String), &lin); err != nil {
			return card.Card{}, err
		}
		c.Lineage = &lin
	}
	return c, nil
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
		if k.Provider == "openai" {
			return k, nil
		}
	}
	return keys[0], nil
}

func chatFuse(ctx context.Context, client *llm.Client, key llmKey, model string, parents []card.Card, spec []card.ParentRef, dims map[string]card.Dimension, prohibitions []string) (map[string]card.Dimension, []string, error) {
	if strings.TrimSpace(model) == "" {
		model = defaultFuseModel
	}
	userContent, err := buildFuseUserContent(parents, spec, dims, prohibitions)
	if err != nil {
		return nil, nil, err
	}
	req := llm.Request{
		Provider: key.Provider,
		BaseURL:  key.BaseURL,
		APIKey:   key.APIKey,
		Model:    model,
		Temp:     fuseChatTemp,
		Messages: []llm.Message{
			{Role: "system", Content: llm.FuseSystem()},
			{Role: "user", Content: userContent},
		},
	}
	var lastErr error
	for attempt := 0; attempt < 1+fuseParseRetries; attempt++ {
		raw, err := client.Chat(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		rewritten, conflicts, err := parseFuseJSON(raw)
		if err == nil {
			return rewritten, conflicts, nil
		}
		lastErr = err
	}
	return nil, nil, lastErr
}

func buildFuseUserContent(parents []card.Card, spec []card.ParentRef, dims map[string]card.Dimension, prohibitions []string) (string, error) {
	type parentView struct {
		CardID   string                    `json:"card_id"`
		Version  int                       `json:"version"`
		Selected map[string]card.Dimension `json:"selected"`
	}
	views := make([]parentView, 0, len(spec))
	for i, ref := range spec {
		selected := make(map[string]card.Dimension, len(ref.Dims))
		src := parents[i].Dimensions
		for _, key := range ref.Dims {
			if d, ok := src[key]; ok {
				selected[key] = d
			}
		}
		views = append(views, parentView{
			CardID:   ref.CardID,
			Version:  ref.Version,
			Selected: selected,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"parents":      views,
		"dimensions":   dims,
		"prohibitions": prohibitions,
	})
	if err != nil {
		return "", err
	}
	return "已按权重混合的卡片与父卡被选维：\n" + string(payload), nil
}

func parseFuseJSON(raw string) (map[string]card.Dimension, []string, error) {
	payload := extractJSONObject(raw)
	var out struct {
		Dimensions   map[string]card.Dimension `json:"dimensions"`
		Prohibitions []string                  `json:"prohibitions"`
		Conflicts    []string                  `json:"conflicts"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, nil, fmt.Errorf("invalid: fuse json: %w", err)
	}
	if len(out.Dimensions) == 0 {
		return nil, nil, fmt.Errorf("invalid: fuse json missing dimensions")
	}
	if out.Conflicts == nil {
		out.Conflicts = []string{}
	}
	return out.Dimensions, out.Conflicts, nil
}

func extractJSONObject(raw string) []byte {
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
	return bytes.TrimSpace([]byte(s))
}

func persistCard(ctx context.Context, st *store.Store, c card.Card) error {
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
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Kind, c.Version, now, now,
	)
	if err != nil {
		return err
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
