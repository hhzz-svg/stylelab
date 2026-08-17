package extract

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"stylelab/internal/card"
	"stylelab/internal/cryptokey"
	"stylelab/internal/ids"
	"stylelab/internal/llm"
	"stylelab/internal/store"
	"stylelab/internal/stylestat"
)

const (
	maxSampleRunes      = 100000
	chapterSampleRunes  = 400
	maxLLMSampleRunes   = 6000
	extractChatTemp     = 0.3
	extractParseRetries = 2
	defaultExtractModel = "gpt-4o-mini"
	defaultExtractName  = "抽离风格"
	patternPrefix       = "避免过度使用："
)

type ExtractInput struct {
	ProjectID string   `json:"project_id"`
	AssetIDs  []string `json:"asset_ids"`
	Name      string   `json:"name"`
	Model     string   `json:"model"`
}

type llmKey struct {
	Provider string
	BaseURL  string
	APIKey   string
}

func Run(ctx context.Context, st *store.Store, client *llm.Client, master []byte, userID string, in ExtractInput, prog func(int, string)) (card.Card, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if prog == nil {
		prog = func(int, string) {}
	}

	prog(5, "load_assets")
	text, err := loadConcatText(ctx, st, userID, in)
	if err != nil {
		return card.Card{}, err
	}
	if utf8.RuneCountInString(text) > maxSampleRunes {
		return card.Card{}, fmt.Errorf("invalid: sample too large")
	}

	prog(20, "stylestat")
	chapters := SplitChapters(text)
	stats := stylestat.Compute(stylestat.Input{Chapters: chapters})
	facts, err := json.Marshal(stats)
	if err != nil {
		return card.Card{}, err
	}

	prog(35, "sample")
	sample := buildLLMSample(chapters)

	prog(45, "llm_key")
	key, err := loadUserKey(ctx, st, master, userID)
	if err != nil {
		return card.Card{}, err
	}

	prog(60, "extract")
	userContent := "统计事实：\n" + string(facts) + "\n\n章节样本：\n" + sample
	dims, prohibitions, err := chatExtract(ctx, client, key, in.Model, userContent)
	if err != nil {
		return card.Card{}, err
	}

	for _, p := range stats.Patterns {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		prohibitions = append(prohibitions, patternPrefix+p.Name)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = defaultExtractName
	}
	out := card.Card{
		ID:           ids.New("crd_"),
		ProjectID:    in.ProjectID,
		Name:         name,
		Kind:         "extracted",
		Version:      1,
		Dimensions:   dims,
		Prohibitions: prohibitions,
		Facts:        facts,
	}
	if err := card.Validate(out); err != nil {
		return card.Card{}, err
	}

	prog(85, "persist")
	if err := persistCard(ctx, st, out); err != nil {
		return card.Card{}, err
	}
	prog(100, "done")
	return out, nil
}

func loadConcatText(ctx context.Context, st *store.Store, userID string, in ExtractInput) (string, error) {
	if in.ProjectID == "" || len(in.AssetIDs) == 0 {
		return "", fmt.Errorf("invalid: assets required")
	}
	var b strings.Builder
	for _, assetID := range in.AssetIDs {
		var sha string
		err := st.DB().QueryRowContext(
			ctx,
			`SELECT a.sha256
			 FROM assets a
			 JOIN projects p ON p.id = a.project_id
			 WHERE a.id = ? AND a.project_id = ? AND p.user_id = ?`,
			assetID, in.ProjectID, userID,
		).Scan(&sha)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", fmt.Errorf("invalid: asset not found")
			}
			return "", err
		}
		data, err := os.ReadFile(filepath.Join(st.DataDir(), "blobs", sha))
		if err != nil {
			return "", err
		}
		b.Write(data)
	}
	return b.String(), nil
}

func buildLLMSample(chapters []string) string {
	var b strings.Builder
	total := 0
	for _, ch := range chapters {
		runes := []rune(ch)
		if len(runes) > chapterSampleRunes {
			runes = runes[:chapterSampleRunes]
		}
		remain := maxLLMSampleRunes - total
		if remain <= 0 {
			break
		}
		if len(runes) > remain {
			runes = runes[:remain]
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(string(runes))
		total += len(runes)
	}
	return b.String()
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

func chatExtract(ctx context.Context, client *llm.Client, key llmKey, model, userContent string) (map[string]card.Dimension, []string, error) {
	if strings.TrimSpace(model) == "" {
		model = defaultExtractModel
	}
	req := llm.Request{
		Provider: key.Provider,
		BaseURL:  key.BaseURL,
		APIKey:   key.APIKey,
		Model:    model,
		Temp:     extractChatTemp,
		Messages: []llm.Message{
			{Role: "system", Content: llm.ExtractSystem()},
			{Role: "user", Content: userContent},
		},
	}
	var lastErr error
	for attempt := 0; attempt < 1+extractParseRetries; attempt++ {
		raw, err := client.Chat(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		dims, prohibitions, err := parseExtractJSON(raw)
		if err == nil {
			return dims, prohibitions, nil
		}
		lastErr = err
	}
	return nil, nil, lastErr
}

func parseExtractJSON(raw string) (map[string]card.Dimension, []string, error) {
	payload := extractJSONObject(raw)
	var out struct {
		Dimensions   map[string]card.Dimension `json:"dimensions"`
		Prohibitions []string                  `json:"prohibitions"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, nil, fmt.Errorf("invalid: extract json: %w", err)
	}
	if len(out.Dimensions) == 0 {
		return nil, nil, fmt.Errorf("invalid: extract json missing dimensions")
	}
	if out.Prohibitions == nil {
		out.Prohibitions = []string{}
	}
	return out.Dimensions, out.Prohibitions, nil
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
	_, err = st.DB().ExecContext(
		ctx,
		`INSERT INTO style_cards (id, project_id, name, kind, current_version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Kind, c.Version, now, now,
	)
	if err != nil {
		return err
	}
	_, err = st.DB().ExecContext(
		ctx,
		`INSERT INTO style_card_versions (card_id, version, dimensions_json, prohibitions_json, facts_json, lineage_json, created_at)
		 VALUES (?, ?, ?, ?, ?, NULL, ?)`,
		c.ID, c.Version, string(dims), string(prohibitions), string(facts), now,
	)
	return err
}
